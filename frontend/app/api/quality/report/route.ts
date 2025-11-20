import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE_URL = getBackendUrl()

export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url)
  const database = searchParams.get('database')

  if (!database) {
    return NextResponse.json(
      { error: 'database parameter is required' },
      { status: 400 }
    )
  }

  try {
    const backendResponse = await fetch(
      `${API_BASE_URL}/api/quality/report?database=${encodeURIComponent(database)}`
    )

    if (!backendResponse.ok) {
      const errorData = await backendResponse.json().catch(() => ({ error: 'Failed to fetch quality report' }))
      return NextResponse.json(
        { error: errorData.error || 'Failed to fetch quality report' },
        { status: backendResponse.status }
      )
    }

    const data = await backendResponse.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching quality report:', error)
    return NextResponse.json(
      { error: 'Internal server error' },
      { status: 500 }
    )
  }
}

