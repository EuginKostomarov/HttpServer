import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE_URL = getBackendUrl()

export async function GET(
  request: Request,
  { params }: { params: Promise<{ clientId: string; projectId: string; dbId: string }> }
) {
  try {
    const { clientId, projectId, dbId } = await params

    const response = await fetch(`${API_BASE_URL}/api/clients/${clientId}/projects/${projectId}/databases/${dbId}`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      cache: 'no-store',
    })

    if (!response.ok) {
      if (response.status === 404) {
        return NextResponse.json(
          { error: 'Database not found' },
          { status: 404 }
        )
      }
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching database:', error)
    return NextResponse.json(
      { error: 'Failed to fetch database' },
      { status: 500 }
    )
  }
}

export async function PUT(
  request: Request,
  { params }: { params: Promise<{ clientId: string; projectId: string; dbId: string }> }
) {
  try {
    const { clientId, projectId, dbId } = await params
    const body = await request.json()

    const response = await fetch(`${API_BASE_URL}/api/clients/${clientId}/projects/${projectId}/databases/${dbId}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    })

    if (!response.ok) {
      if (response.status === 404) {
        const errorMsg = 'Backend endpoint not found. Please restart the backend server.'
        return NextResponse.json(
          { error: errorMsg },
          { status: 503 }
        )
      }
      const errorData = await response.json().catch(() => ({}))
      const errorText = await response.text().catch(() => '')
      console.error(`Backend error (${response.status}):`, errorData || errorText)
      return NextResponse.json(
        { error: errorData.error || errorText || `HTTP error! status: ${response.status}` },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error updating database:', error)
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to update database' },
      { status: 500 }
    )
  }
}

export async function DELETE(
  request: Request,
  { params }: { params: Promise<{ clientId: string; projectId: string; dbId: string }> }
) {
  try {
    const { clientId, projectId, dbId } = await params

    const response = await fetch(`${API_BASE_URL}/api/clients/${clientId}/projects/${projectId}/databases/${dbId}`, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
      },
    })

    if (!response.ok) {
      if (response.status === 404) {
        return NextResponse.json(
          { error: 'Database not found' },
          { status: 404 }
        )
      }
      const errorData = await response.json().catch(() => ({}))
      const errorText = await response.text().catch(() => '')
      console.error(`Backend error (${response.status}):`, errorData || errorText)
      return NextResponse.json(
        { error: errorData.error || errorText || `HTTP error! status: ${response.status}` },
        { status: response.status }
      )
    }

    return NextResponse.json({ success: true })
  } catch (error) {
    console.error('Error deleting database:', error)
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to delete database' },
      { status: 500 }
    )
  }
}

