import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE_URL = getBackendUrl()

export async function GET(
  request: Request,
  { params }: { params: Promise<{ clientId: string; projectId: string }> }
) {
  try {
    const { clientId, projectId } = await params
    const { searchParams } = new URL(request.url)
    const category = searchParams.get('category') || ''
    const approvedOnly = searchParams.get('approved_only') === 'true'

    const queryParams = new URLSearchParams()
    if (category) queryParams.append('category', category)
    if (approvedOnly) queryParams.append('approved_only', 'true')

    const url = `${API_BASE_URL}/api/clients/${clientId}/projects/${projectId}/benchmarks${queryParams.toString() ? `?${queryParams.toString()}` : ''}`
    
    const response = await fetch(url, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      cache: 'no-store',
    })

    if (!response.ok) {
      if (response.status === 404) {
        return NextResponse.json({ benchmarks: [], total: 0 })
      }
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    const data = await response.json()
    console.log('Benchmarks API response:', { 
      status: response.status, 
      dataKeys: Object.keys(data),
      benchmarksCount: Array.isArray(data) ? data.length : (data.benchmarks?.length || 0)
    })
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching benchmarks:', error)
    return NextResponse.json({ benchmarks: [], total: 0 })
  }
}

export async function POST(
  request: Request,
  { params }: { params: Promise<{ clientId: string; projectId: string }> }
) {
  try {
    const { clientId, projectId } = await params
    const body = await request.json()

    const response = await fetch(`${API_BASE_URL}/api/clients/${clientId}/projects/${projectId}/benchmarks`, {
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
    console.error('Error creating benchmark:', error)
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to create benchmark' },
      { status: 500 }
    )
  }
}

