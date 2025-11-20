import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE_URL = getBackendUrl()

export async function POST(
  request: Request,
  { params }: { params: Promise<{ clientId: string; projectId: string }> }
) {
  try {
    const { clientId, projectId } = await params
    const response = await fetch(`${API_BASE_URL}/api/clients/${clientId}/projects/${projectId}/normalization/stop`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
    })

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}))
      return NextResponse.json(
        { error: errorData.error || 'Failed to stop normalization' },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error stopping normalization:', error)
    return NextResponse.json(
      { error: 'Failed to stop normalization' },
      { status: 500 }
    )
  }
}

