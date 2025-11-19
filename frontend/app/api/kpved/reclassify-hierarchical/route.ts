import { NextRequest, NextResponse } from 'next/server'
import { kpvedReclassifySchema, validateRequest, formatValidationError } from '@/lib/validation'

// Используем 127.0.0.1 вместо localhost для более надежного подключения
const BACKEND_URL = process.env.BACKEND_URL || 'http://127.0.0.1:9999'

// Увеличиваем максимальное время выполнения для длительных операций классификации
// Это важно для Vercel и других платформ с ограничениями времени выполнения
export const maxDuration = 3600 // 1 час (максимум для Vercel Pro, для Hobby - 10 секунд)

export async function POST(request: NextRequest) {
  try {
    const body = await request.json()

    // Validate request body
    const validation = validateRequest(kpvedReclassifySchema, body)
    if (!validation.success) {
      return NextResponse.json(
        {
          error: 'Validation failed',
          details: formatValidationError(validation.details)
        },
        { status: 400 }
      )
    }

    // Создаем AbortController для таймаута
    // Классификация 12 981 группы может занять много времени (несколько часов)
    // Увеличиваем таймаут до 2 часов для длительных операций
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), 2 * 60 * 60 * 1000) // 2 часа таймаут

    try {
      // Проверяем доступность бэкенда перед отправкой запроса
      try {
        const healthController = new AbortController()
        const healthTimeout = setTimeout(() => healthController.abort(), 5000)
        const healthCheck = await fetch(`${BACKEND_URL}/health`, {
          method: 'GET',
          signal: healthController.signal,
        })
        clearTimeout(healthTimeout)
        if (!healthCheck.ok) {
          throw new Error('Backend health check failed')
        }
      } catch (healthError: any) {
        console.error('Backend health check failed:', healthError)
        const isTimeout = healthError.name === 'AbortError' || healthError.code === 'ECONNREFUSED'
        return NextResponse.json(
          { 
            error: isTimeout 
              ? 'Бэкенд недоступен. Убедитесь, что сервер запущен на порту 9999.'
              : 'Ошибка подключения к бэкенду',
            details: healthError instanceof Error ? healthError.message : 'Unknown error'
          },
          { status: 503 } // Service Unavailable
        )
      }

      const response = await fetch(`${BACKEND_URL}/api/kpved/reclassify-hierarchical`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(validation.data),
        signal: controller.signal,
      })

      clearTimeout(timeoutId)

      if (!response.ok) {
        let errorMessage = 'Ошибка при запуске классификации КПВЭД'
        try {
          const errorText = await response.text()
          console.error(`Backend responded with status ${response.status}:`, errorText)
          
          // Пробуем распарсить как JSON
          try {
            const errorData = JSON.parse(errorText)
            errorMessage = errorData.error || errorData.message || errorData.detail || errorMessage
          } catch {
            // Если не JSON, используем текст ответа
            errorMessage = errorText.trim() || response.statusText || `HTTP ${response.status}`
          }
        } catch (parseError) {
          errorMessage = response.statusText || `HTTP ${response.status}`
          console.error('Error parsing error response:', parseError)
        }
        
        return NextResponse.json(
          { error: errorMessage },
          { status: response.status || 500 }
        )
      }

      const data = await response.json()
      return NextResponse.json(data)
    } catch (fetchError: any) {
      clearTimeout(timeoutId)
      
      // Проверяем, был ли это таймаут
      if (fetchError.name === 'AbortError' || controller.signal.aborted) {
        console.error('Timeout during KPVED reclassification')
        return NextResponse.json(
          { error: 'Таймаут при выполнении классификации. Операция может занять много времени для большого количества групп.' },
          { status: 504 } // Gateway Timeout
        )
      }
      
      // Проверяем тип ошибки подключения
      if (fetchError.code === 'ECONNREFUSED' || fetchError.message?.includes('fetch failed') || fetchError.message?.includes('ECONNREFUSED')) {
        console.error('Connection refused to backend:', fetchError)
        return NextResponse.json(
          { 
            error: 'Не удалось подключиться к бэкенду. Убедитесь, что сервер запущен на порту 9999.',
            details: fetchError.message || 'Connection refused'
          },
          { status: 503 } // Service Unavailable
        )
      }
      
      throw fetchError
    }
  } catch (error) {
    console.error('Error in KPVED reclassify-hierarchical API route:', error)
    const errorMessage = error instanceof Error ? error.message : 'Unknown error'
    
    // Проверяем, является ли это ошибкой подключения
    if (errorMessage.includes('fetch failed') || errorMessage.includes('ECONNREFUSED') || errorMessage.includes('ENOTFOUND')) {
      return NextResponse.json(
        { 
          error: 'Не удалось подключиться к бэкенду. Убедитесь, что сервер запущен на порту 9999.',
          details: errorMessage
        },
        { status: 503 } // Service Unavailable
      )
    }
    
    return NextResponse.json(
      { 
        error: 'Ошибка при обработке запроса',
        details: errorMessage
      },
      { status: 500 }
    )
  }
}

