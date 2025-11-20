import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE_URL = getBackendUrl()

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url)
    const history = searchParams.get('history')
    const limit = searchParams.get('limit')
    const model = searchParams.get('model')

    const queryParams = new URLSearchParams()
    if (history === 'true') queryParams.append('history', 'true')
    if (limit) queryParams.append('limit', limit)
    if (model) queryParams.append('model', model)

    const url = `${API_BASE_URL}/api/models/benchmark${queryParams.toString() ? `?${queryParams.toString()}` : ''}`
    
    const response = await fetch(url, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      cache: 'no-store',
    })

      if (!response.ok) {
        const errorText = await response.text().catch(() => '')
        console.error(`[Benchmark API] GET error (${response.status}):`, errorText)
        
        // Если это 404, возвращаем пустой ответ с сообщением
        if (response.status === 404) {
          return NextResponse.json(
            { 
              models: [], 
              total: 0, 
              test_count: 0, 
              timestamp: new Date().toISOString(),
              message: "Use POST to run benchmark or ?history=true to get history"
            },
            { status: 200 } // Возвращаем 200, чтобы фронтенд не показывал ошибку
          )
        }
        
        return NextResponse.json(
          { error: errorText || `HTTP error! status: ${response.status}`, models: [], total: 0, test_count: 0, timestamp: new Date().toISOString() },
          { status: response.status }
        )
      }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('[Benchmark API] GET error:', error)
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to fetch benchmarks', models: [], total: 0, test_count: 0, timestamp: new Date().toISOString() },
      { status: 500 }
    )
  }
}

export async function POST(request: Request) {
  try {
    const body = await request.json().catch(() => ({}))
    
    const response = await fetch(`${API_BASE_URL}/api/models/benchmark`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
      cache: 'no-store',
    })

    if (!response.ok) {
      let errorMessage = `HTTP error! status: ${response.status}`
      
      try {
        const errorData = await response.json()
        errorMessage = errorData.error || errorData.message || errorMessage
        
        // Улучшаем сообщения об ошибках для пользователя
        if (errorMessage.includes("ARLIAI_API_KEY") || errorMessage.includes("API key")) {
          errorMessage = "API ключ Arliai не настроен. Настройте его в разделе 'Воркеры' или установите переменную окружения ARLIAI_API_KEY"
        } else if (errorMessage.includes("No models available")) {
          errorMessage = "Нет доступных моделей для тестирования. Проверьте конфигурацию воркеров"
        } else if (errorMessage.includes("Failed to get models")) {
          errorMessage = "Не удалось получить список моделей. Проверьте конфигурацию"
        }
      } catch {
        // Если не удалось распарсить JSON, используем статус код
        const errorText = await response.text().catch(() => '')
        if (errorText) {
          errorMessage = errorText
        } else if (response.status === 503) {
          errorMessage = "Сервис временно недоступен. Проверьте настройки API ключа"
        } else if (response.status === 404) {
          errorMessage = "Эндпоинт не найден. Проверьте версию API"
        } else if (response.status === 500) {
          errorMessage = "Внутренняя ошибка сервера. Проверьте логи сервера"
        }
      }
      
      console.error(`[Benchmark API] POST error (${response.status}):`, errorMessage)
      
      return NextResponse.json(
        { error: errorMessage },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('[Benchmark API] POST error:', error)
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to run benchmark' },
      { status: 500 }
    )
  }
}

