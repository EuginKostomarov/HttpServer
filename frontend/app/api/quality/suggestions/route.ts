import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:9999'

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url)
    
    // Получаем параметры из query string
    const database = searchParams.get('database')
    const limit = searchParams.get('limit')
    const offset = searchParams.get('offset')
    const priority = searchParams.get('priority')
    const autoApplyable = searchParams.get('auto_applyable')
    const applied = searchParams.get('applied')

    // Формируем URL для бэкенда
    const params = new URLSearchParams()
    if (database) params.append('database', database)
    if (limit) params.append('limit', limit)
    if (offset) params.append('offset', offset)
    if (priority) params.append('priority', priority)
    if (autoApplyable) params.append('auto_applyable', autoApplyable)
    if (applied) params.append('applied', applied)

    const backendUrl = `${BACKEND_URL}/api/quality/suggestions?${params.toString()}`
    
    console.log('Fetching suggestions from backend:', backendUrl)

    const response = await fetch(backendUrl, {
      method: 'GET',
      cache: 'no-store',
      headers: {
        'Content-Type': 'application/json',
      },
    })

    if (!response.ok) {
      let errorMessage = 'Failed to fetch suggestions'
      try {
        const errorData = await response.json()
        errorMessage = errorData.error || errorMessage
      } catch {
        errorMessage = `Backend responded with status ${response.status}`
      }
      
      console.error('Error fetching suggestions:', errorMessage)
      return NextResponse.json(
        { error: errorMessage },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error in quality suggestions API route:', error)
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

