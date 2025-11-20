import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function POST() {
  try {
    const response = await fetch(`${BACKEND_URL}/api/reclassification/stop`, {
      method: 'POST',
    })

    if (!response.ok) {
      const error = await response.text()
      return NextResponse.json(
        { error: error || 'Failed to stop reclassification' },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error stopping reclassification:', error)
    return NextResponse.json(
      { error: 'Internal server error' },
      { status: 500 }
    )
  }
}

