import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE_URL = getBackendUrl()

export async function GET(
  request: Request,
  { params }: { params: Promise<{ clientId: string; projectId: string }> }
) {
  try {
    const { clientId, projectId } = await params
    const { searchParams } = new URL(request.url)
    const activeOnly = searchParams.get('active_only') === 'true'

    const queryParams = new URLSearchParams()
    if (activeOnly) queryParams.append('active_only', 'true')

    const url = `${API_BASE_URL}/api/clients/${clientId}/projects/${projectId}/databases${queryParams.toString() ? `?${queryParams.toString()}` : ''}`
    
    const response = await fetch(url, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      cache: 'no-store',
    })

    if (!response.ok) {
      if (response.status === 404) {
        return NextResponse.json({ databases: [], total: 0 })
      }
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching databases:', error)
    return NextResponse.json({ databases: [], total: 0 })
  }
}

export async function POST(
  request: Request,
  { params }: { params: Promise<{ clientId: string; projectId: string }> }
) {
  try {
    const { clientId, projectId } = await params
    const contentType = request.headers.get('content-type') || ''
    const requestID = request.headers.get('x-request-id') || `req_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`
    const clientIP = request.headers.get('x-forwarded-for')?.split(',')[0] || request.headers.get('x-real-ip') || 'unknown'

    // Проверяем, является ли запрос multipart/form-data (загрузка файла)
    if (contentType.includes('multipart/form-data')) {
      const uploadStartTime = Date.now()
      console.log(`[API Route] [${requestID}] Proxying multipart/form-data request to backend for client ${clientId}, project ${projectId} (IP: ${clientIP})`)
      console.log(`[API Route] Content-Type: ${contentType}`)
      const contentLength = request.headers.get('content-length')
      const fileSizeMB = contentLength ? (parseInt(contentLength) / 1024 / 1024).toFixed(2) : 'unknown'
      console.log(`[API Route] Content-Length: ${contentLength || 'not set'} (~${fileSizeMB} MB)`)
      
      // В Next.js для multipart/form-data используем request.formData()
      // и затем пересоздаем FormData для передачи к бэкенду
      const formData = await request.formData()
      
      if (!formData) {
        console.error('[API Route] FormData is null or undefined')
        return NextResponse.json(
          { error: 'No form data received' },
          { status: 400 }
        )
      }
      
      // Создаем новый FormData для передачи к бэкенду
      // В Node.js 18+ FormData поддерживается нативно
      const backendFormData = new FormData()
      
      // Копируем все поля из оригинального FormData
      let fileCount = 0
      let fieldCount = 0
      let hasFileField = false
      
      // Сначала обрабатываем файлы (они требуют await)
      const fileEntries: Array<[string, File]> = []
      const textEntries: Array<[string, string]> = []
      
      for (const [key, value] of Array.from(formData.entries())) {
        if (value instanceof File) {
          fileEntries.push([key, value])
        } else {
          textEntries.push([key, String(value)])
        }
      }
      
      // Обрабатываем файлы с await
      for (const [key, file] of fileEntries) {
        try {
          // Логируем информацию о файле для диагностики кодировки
          const fileNameInfo = {
            name: file.name,
            nameLength: file.name.length,
            nameBytes: Buffer.from(file.name, 'utf8').length,
            nameEncoded: encodeURIComponent(file.name),
            firstChars: file.name.substring(0, Math.min(50, file.name.length))
          }
          console.log(`[API Route] Processing file field: ${key}`, fileNameInfo)
          
          // В Node.js 18+ FormData поддерживает File напрямую
          // Передаём File напрямую, так как Node.js FormData поддерживает его
          backendFormData.append(key, file, file.name)
          fileCount++
          if (key === 'file') hasFileField = true
          console.log(`[API Route] ✅ Added file field: ${key}, filename: ${file.name}, size: ${file.size} bytes, type: ${file.type}`)
        } catch (fileError) {
          console.error(`[API Route] ❌ Error processing file field ${key}:`, fileError)
          throw fileError
        }
      }
      
      // Обрабатываем текстовые поля
      for (const [key, value] of textEntries) {
        backendFormData.append(key, value)
        fieldCount++
        console.log(`[API Route] Added form field: ${key} = ${value}`)
      }
      
      // Проверяем, что есть поле 'file'
      if (!hasFileField) {
        console.error('[API Route] No file field found in FormData')
        return NextResponse.json(
          { error: 'No file field found in form data. Please ensure the file is sent with the field name "file".' },
          { status: 400 }
        )
      }
      
      console.log(`[API Route] FormData prepared: ${fileCount} file(s), ${fieldCount} field(s), sending to backend`)
      
      // Устанавливаем таймаут для больших файлов (10 минут)
      const controller = new AbortController()
      const timeoutId = setTimeout(() => controller.abort(), 10 * 60 * 1000)
      
      try {
        const response = await fetch(`${API_BASE_URL}/api/clients/${clientId}/projects/${projectId}/databases`, {
          method: 'POST',
          body: backendFormData,
          signal: controller.signal,
          headers: {
            'X-Request-ID': requestID,
          },
          // Не устанавливаем Content-Type - fetch автоматически установит правильный Content-Type с boundary
          // В Node.js 18+ fetch поддерживает FormData напрямую
        } as RequestInit)
        
        clearTimeout(timeoutId)
        const backendResponseTime = ((Date.now() - uploadStartTime) / 1000).toFixed(2)
        console.log(`[API Route] 📡 Ответ от бэкенда получен: статус ${response.status} (время: ${backendResponseTime}s)`)

        if (!response.ok) {
          let errorData: any = {}
          let errorText = ''
          try {
            errorData = await response.json()
          } catch {
            try {
              errorText = await response.text()
            } catch {
              errorText = `HTTP error! status: ${response.status}`
            }
          }
          console.error(`[API Route] ❌ Ошибка бэкенда (${response.status}, время: ${backendResponseTime}s):`, errorData || errorText)
          console.error(`[API Route] Детали запроса:`, {
            url: `${API_BASE_URL}/api/clients/${clientId}/projects/${projectId}/databases`,
            method: 'POST',
            hasFormData: !!backendFormData,
            fileCount,
            fieldCount,
            hasFileField
          })
          return NextResponse.json(
            { error: errorData.error || errorText || `HTTP error! status: ${response.status}` },
            { status: response.status }
          )
        }

        const data = await response.json()
        const uploadDuration = ((Date.now() - uploadStartTime) / 1000).toFixed(2)
        console.log(`[API Route] Successfully uploaded file in ${uploadDuration}s, response:`, { 
          suggested_name: data.suggested_name, 
          file_path: data.file_path,
          file_size_mb: fileSizeMB
        })
        return NextResponse.json(data, { status: response.status })
      } catch (fetchError: any) {
        clearTimeout(timeoutId)
        if (fetchError.name === 'AbortError') {
          console.error('[API Route] Request timeout after 10 minutes')
          return NextResponse.json(
            { error: 'Request timeout. The file may be too large or the server is not responding.' },
            { status: 408 }
          )
        }
        console.error('[API Route] Fetch error:', fetchError)
        throw fetchError
      }
    }

    // Обычный JSON запрос
    const body = await request.json()

    const response = await fetch(`${API_BASE_URL}/api/clients/${clientId}/projects/${projectId}/databases`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    })

    if (!response.ok) {
      if (response.status === 404) {
        const errorMsg = 'Backend endpoint not found. Please restart the backend server.'
        return NextResponse.json(
          { error: errorMsg },
          { status: 503 }
        )
      }
      const errorData = await response.json().catch(() => ({}))
      const errorText = await response.text().catch(() => '')
      console.error(`Backend error (${response.status}):`, errorData || errorText)
      return NextResponse.json(
        { error: errorData.error || errorText || `HTTP error! status: ${response.status}` },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data, { status: 201 })
  } catch (error) {
    console.error('Error creating database:', error)
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to create database' },
      { status: 500 }
    )
  }
}


