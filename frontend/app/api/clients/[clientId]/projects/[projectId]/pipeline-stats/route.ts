import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ clientId: string; projectId: string }> }
) {
  try {
    const { clientId, projectId } = await params

    const response = await fetch(
      `${BACKEND_URL}/api/clients/${clientId}/projects/${projectId}/pipeline-stats`,
      {
        cache: 'no-store',
        headers: {
          'Content-Type': 'application/json',
        },
      }
    )

    if (!response.ok) {
      const errorText = await response.text().catch(() => 'Unknown error')
      console.error('Backend pipeline stats error:', errorText)
      return NextResponse.json(
        { error: `Backend error: ${response.status}`, details: errorText },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data, {
      headers: {
        'Cache-Control': 'no-store, no-cache, must-revalidate',
      },
    })
  } catch (error) {
    console.error('Error fetching pipeline stats:', error)
    return NextResponse.json(
      {
        error: 'Failed to connect to backend',
        details: error instanceof Error ? error.message : 'Unknown error',
      },
      { status: 500 }
    )
  }
}

