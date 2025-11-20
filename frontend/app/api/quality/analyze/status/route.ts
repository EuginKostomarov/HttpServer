import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET() {
  try {
    console.log('Proxying GET /api/quality/analyze/status to backend')
    
    const response = await fetch(`${BACKEND_URL}/api/quality/analyze/status`, {
      cache: 'no-store',
      headers: {
        'Content-Type': 'application/json',
      },
    })

    if (!response.ok) {
      // Если бэкенд недоступен или вернул ошибку, возвращаем дефолтный статус
      const errorText = await response.text().catch(() => '')
      console.error(`Backend responded with status ${response.status}:`, errorText)
      
      if (response.status === 0 || response.status >= 500) {
        return NextResponse.json({
          is_running: false,
          progress: 0,
          processed: 0,
          total: 0,
          current_step: 'idle',
          duplicates_found: 0,
          violations_found: 0,
          suggestions_found: 0,
          error: 'Backend unavailable'
        })
      }
      
      let errorMessage = 'Failed to fetch analysis status'
      try {
        const errorData = await response.json()
        errorMessage = errorData.error || errorMessage
      } catch {
        errorMessage = `Backend responded with status ${response.status}`
      }
      
      return NextResponse.json(
        { error: errorMessage },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching analysis status:', error)
    const errorMessage = error instanceof Error ? error.message : 'Unknown error'
    
    // Возвращаем дефолтный статус вместо ошибки, чтобы фронтенд мог работать
    return NextResponse.json({
      is_running: false,
      progress: 0,
      processed: 0,
      total: 0,
      current_step: 'idle',
      duplicates_found: 0,
      violations_found: 0,
      suggestions_found: 0,
      error: 'Backend unavailable',
      details: errorMessage
    })
  }
}

