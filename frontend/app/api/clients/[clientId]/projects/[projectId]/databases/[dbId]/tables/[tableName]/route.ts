import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE_URL = getBackendUrl()

export async function GET(
  request: Request,
  { params }: { params: Promise<{ clientId: string; projectId: string; dbId: string; tableName: string }> }
) {
  try {
    const { clientId, projectId, dbId, tableName } = await params
    const { searchParams } = new URL(request.url)
    const page = searchParams.get('page') || '1'
    const pageSize = searchParams.get('pageSize') || '50'

    const url = new URL(`${API_BASE_URL}/api/clients/${clientId}/projects/${projectId}/databases/${dbId}/tables/${tableName}`)
    url.searchParams.set('page', page)
    url.searchParams.set('pageSize', pageSize)

    const response = await fetch(url.toString(), {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      cache: 'no-store',
    })

    if (!response.ok) {
      if (response.status === 404) {
        return NextResponse.json(
          { error: 'Table not found' },
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

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching table data:', error)
    return NextResponse.json(
      { error: 'Failed to fetch table data' },
      { status: 500 }
    )
  }
}

