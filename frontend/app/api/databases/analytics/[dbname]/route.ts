import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ dbname: string }> }
) {
  try {
    const { dbname } = await params
    if (!dbname) {
      return NextResponse.json(
        { error: 'Database name is required' },
        { status: 400 }
      )
    }

    // Получаем путь из query параметра
    const url = new URL(request.url)
    const dbPath = url.searchParams.get('path')
    
    // Если путь не указан в query, используем имя из параметра
    const finalPath = dbPath || dbname
    
    if (!finalPath) {
      return NextResponse.json(
        { error: 'Database path is required' },
        { status: 400 }
      )
    }
    
    // Используем путь из query параметра или имя файла
    const response = await fetch(`${BACKEND_URL}/api/databases/analytics?path=${encodeURIComponent(finalPath)}`, {
      cache: 'no-store',
      headers: {
        'Accept': 'application/json',
      },
    })

    if (!response.ok) {
      let errorMessage = 'Failed to fetch database analytics'
      const contentType = response.headers.get('content-type')
      
      try {
        // Пытаемся получить текст ответа (можно прочитать только один раз)
        const responseText = await response.text()
        
        // Пытаемся распарсить как JSON
        if (contentType?.includes('application/json')) {
          try {
            const errorData = JSON.parse(responseText)
            errorMessage = errorData.error || errorMessage
          } catch {
            // Если не JSON, используем текст как есть
            if (responseText) {
              errorMessage = responseText.length > 200 
                ? responseText.substring(0, 200) + '...' 
                : responseText
            }
          }
        } else if (responseText) {
          errorMessage = responseText.length > 200 
            ? responseText.substring(0, 200) + '...' 
            : responseText
        }
      } catch (parseError) {
        console.error('Error parsing error response:', parseError)
      }
      
      console.error('Backend error:', {
        status: response.status,
        statusText: response.statusText,
        message: errorMessage,
      })
      
      return NextResponse.json(
        { error: errorMessage },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching database analytics:', error)
    const errorMessage = error instanceof Error ? error.message : 'Unknown error'
    return NextResponse.json(
      { error: `Internal server error: ${errorMessage}` },
      { status: 500 }
    )
  }
}

