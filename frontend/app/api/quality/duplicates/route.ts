import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url)
    
    // Получаем параметры из query string
    const database = searchParams.get('database')
    const limit = searchParams.get('limit')
    const offset = searchParams.get('offset')
    const unmerged = searchParams.get('unmerged')

    // Формируем URL для бэкенда
    const params = new URLSearchParams()
    if (database) params.append('database', database)
    if (limit) params.append('limit', limit)
    if (offset) params.append('offset', offset)
    if (unmerged) params.append('unmerged', unmerged)

    const backendUrl = `${BACKEND_URL}/api/quality/duplicates?${params.toString()}`
    
    console.log(`Proxying GET /api/quality/duplicates to ${backendUrl}`)

    const response = await fetch(backendUrl, {
      method: 'GET',
      cache: 'no-store',
      headers: {
        'Content-Type': 'application/json',
      },
    })

    if (!response.ok) {
      let errorMessage = 'Failed to fetch duplicates'
      try {
        const errorText = await response.text()
        console.error(`Backend responded with status ${response.status}:`, errorText)
        try {
          const errorData = JSON.parse(errorText)
          errorMessage = errorData.error || errorMessage
        } catch {
          errorMessage = `Backend error: ${response.status} - ${errorText}`
        }
      } catch {
        errorMessage = `Backend responded with status ${response.status}`
      }
      
      console.error('Error fetching duplicates:', errorMessage)
      return NextResponse.json(
        { 
          error: errorMessage,
          details: errorMessage
        },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error in quality duplicates API route:', error)
    const errorMessage = error instanceof Error ? error.message : 'Unknown error'
    return NextResponse.json(
      { 
        error: 'Failed to connect to backend',
        details: errorMessage
      },
      { status: 500 }
    )
  }
}

