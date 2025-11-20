import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url)
    const parent = searchParams.get('parent')
    const level = searchParams.get('level')

    let url = `${BACKEND_URL}/api/okpd2/hierarchy`
    const params = new URLSearchParams()

    if (parent) params.append('parent', parent)
    if (level) params.append('level', level)

    if (params.toString()) {
      url += `?${params.toString()}`
    }

    const response = await fetch(url, {
      cache: 'no-store',
    })

    if (!response.ok) {
      return NextResponse.json(
        { error: 'Failed to fetch OKPD2 hierarchy' },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching OKPD2 hierarchy:', error)
    return NextResponse.json(
      { error: 'Internal server error' },
      { status: 500 }
    )
  }
}

