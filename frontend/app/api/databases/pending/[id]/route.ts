import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE = getBackendUrl()

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params
    const response = await fetch(`${API_BASE}/api/databases/pending/${id}`, {
      cache: 'no-store',
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
      { error: `Failed to fetch pending database: ${error instanceof Error ? error.message : 'Unknown error'}` },
      { status: 500 }
    )
  }
}

export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params
    const response = await fetch(`${API_BASE}/api/databases/pending/${id}`, {
      method: 'DELETE',
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
      { error: `Failed to delete pending database: ${error instanceof Error ? error.message : 'Unknown error'}` },
      { status: 500 }
    )
  }
}

