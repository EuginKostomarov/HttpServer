import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url)
    const q = searchParams.get('q')
    const limit = searchParams.get('limit')
    const database = searchParams.get('database')

    if (!q) {
      return NextResponse.json(
        { error: 'Search query (q) is required' },
        { status: 400 }
      )
    }

    const params = new URLSearchParams()
    params.append('q', q)
    if (limit) params.append('limit', limit)
    if (database) params.append('database', database)

    const url = `${BACKEND_URL}/api/kpved/search?${params.toString()}`

    const response = await fetch(url, {
      cache: 'no-store',
    })

    if (!response.ok) {
      const errorText = await response.text()
      return NextResponse.json(
        { error: errorText || 'Failed to search KPVED' },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error searching KPVED:', error)
    return NextResponse.json(
      { error: 'Internal server error' },
      { status: 500 }
    )
  }
}
