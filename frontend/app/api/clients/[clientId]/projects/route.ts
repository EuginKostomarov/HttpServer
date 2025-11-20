import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE_URL = getBackendUrl()

export async function GET(
  request: Request,
  { params }: { params: Promise<{ clientId: string }> }
) {
  try {
    const { clientId } = await params
    const response = await fetch(`${API_BASE_URL}/api/clients/${clientId}/projects`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      cache: 'no-store',
    })

    if (!response.ok) {
      if (response.status === 404) {
        return NextResponse.json([])
      }
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching projects:', error)
    return NextResponse.json([])
  }
}

export async function POST(
  request: Request,
  { params }: { params: Promise<{ clientId: string }> }
) {
  try {
    const { clientId } = await params
    const body = await request.json()

    const response = await fetch(`${API_BASE_URL}/api/clients/${clientId}/projects`, {
      method: 'POST',
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
      throw new Error(errorData.error || errorText || `HTTP error! status: ${response.status}`)
    }

    const data = await response.json()
    return NextResponse.json(data, { status: 201 })
  } catch (error) {
    console.error('Error creating project:', error)
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to create project' },
      { status: 500 }
    )
  }
}

