import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url)
    const database = searchParams.get('database')

    let url = `${BACKEND_URL}/api/normalization/tables`
    if (database) {
      url += `?database=${encodeURIComponent(database)}`
    }

    const response = await fetch(url, {
      cache: 'no-store',
    })

    if (!response.ok) {
      return NextResponse.json(
        { error: 'Failed to fetch tables' },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching tables:', error)
    return NextResponse.json(
      { error: 'Failed to connect to backend' },
      { status: 500 }
    )
  }
}
