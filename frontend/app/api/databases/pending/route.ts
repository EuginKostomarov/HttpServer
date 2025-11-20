import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE = getBackendUrl()

export async function GET(request: NextRequest) {
  try {
    const searchParams = request.nextUrl.searchParams
    const status = searchParams.get('status')
    
    const url = status
      ? `${API_BASE}/api/databases/pending?status=${status}`
      : `${API_BASE}/api/databases/pending`

    const response = await fetch(url, {
      cache: 'no-store',
      headers: {
        'Accept': 'application/json',
      },
    })

    if (!response.ok) {
      const errorText = await response.text().catch(() => 'Unknown error')
      return NextResponse.json(
        { error: errorText || `Backend responded with ${response.status}` },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    return NextResponse.json(
      { error: `Failed to fetch pending databases: ${error instanceof Error ? error.message : 'Unknown error'}` },
      { status: 500 }
    )
  }
}

