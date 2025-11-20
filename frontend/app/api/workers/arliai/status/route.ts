import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET() {
  try {
    const response = await fetch(`${BACKEND_URL}/api/workers/arliai/status`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      cache: 'no-store',
    })

    if (!response.ok) {
      let errorMessage = 'Failed to check Arliai connection'
      try {
        const errorText = await response.text()
        if (errorText) {
          try {
            const errorJson = JSON.parse(errorText)
            errorMessage = errorJson.error || errorText
          } catch {
            errorMessage = errorText
          }
        }
      } catch {
        // Используем дефолтное сообщение
      }
      
      return NextResponse.json(
        { error: errorMessage, connected: false },
        { status: response.status || 500 }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error checking Arliai connection:', error)
    const errorMessage = error instanceof Error ? error.message : 'Failed to connect to backend'
    return NextResponse.json(
      { error: errorMessage, connected: false },
      { status: 500 }
    )
  }
}

