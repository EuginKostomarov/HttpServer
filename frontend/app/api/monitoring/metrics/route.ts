import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET() {
  try {
    const response = await fetch(`${BACKEND_URL}/api/monitoring/metrics`, {
      cache: 'no-store',
    })

    if (!response.ok) {
      return NextResponse.json(
        { error: 'Failed to fetch monitoring metrics' },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching monitoring metrics:', error)
    return NextResponse.json(
      { error: 'Failed to connect to backend' },
      { status: 500 }
    )
  }
}
