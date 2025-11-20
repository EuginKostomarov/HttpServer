import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(request: NextRequest) {
  try {
    // Передаем query параметры в backend
    const searchParams = request.nextUrl.searchParams
    const queryString = searchParams.toString()
    const url = `${BACKEND_URL}/api/workers/models${queryString ? `?${queryString}` : ''}`

    const response = await fetch(url, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        'X-Request-ID': `frontend-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
      },
      cache: 'no-store',
    })

    if (!response.ok) {
      let errorMessage = 'Failed to fetch models'
      try {
        const errorText = await response.text()
        if (errorText) {
          try {
            const errorJson = JSON.parse(errorText)
            errorMessage = errorJson.error?.message || errorJson.error || errorText
          } catch {
            errorMessage = errorText
          }
        }
      } catch {
        // Используем дефолтное сообщение
      }
      
      return NextResponse.json(
        { error: errorMessage, models: [] },
        { status: response.status || 500 }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching models:', error)
    const errorMessage = error instanceof Error ? error.message : 'Failed to connect to backend'
    return NextResponse.json(
      { error: errorMessage, models: [] },
      { status: 500 }
    )
  }
}

