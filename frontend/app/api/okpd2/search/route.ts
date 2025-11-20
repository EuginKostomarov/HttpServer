import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url)
    const query = searchParams.get('q')
    const limit = searchParams.get('limit') || '50'

    if (!query) {
      return NextResponse.json(
        { error: 'Query parameter q is required' },
        { status: 400 }
      )
    }

    const url = `${BACKEND_URL}/api/okpd2/search?q=${encodeURIComponent(query)}&limit=${limit}`

    const response = await fetch(url, {
      cache: 'no-store',
    })

    if (!response.ok) {
      return NextResponse.json(
        { error: 'Failed to search OKPD2' },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data.results || data)
  } catch (error) {
    console.error('Error searching OKPD2:', error)
    return NextResponse.json(
      { error: 'Internal server error' },
      { status: 500 }
    )
  }
}

