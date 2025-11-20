import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url)
    const activeOnly = searchParams.get('active_only')
    const clientId = searchParams.get('client_id')
    const projectId = searchParams.get('project_id')

    let url = `${BACKEND_URL}/api/classification/classifiers`
    const params = new URLSearchParams()

    if (activeOnly) params.append('active_only', activeOnly)
    if (clientId) params.append('client_id', clientId)
    if (projectId) params.append('project_id', projectId)

    if (params.toString()) {
      url += `?${params.toString()}`
    }

    const response = await fetch(url, {
      cache: 'no-store',
    })

    if (!response.ok) {
      return NextResponse.json(
        { error: 'Failed to fetch classifiers' },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching classifiers:', error)
    return NextResponse.json(
      { error: 'Internal server error' },
      { status: 500 }
    )
  }
}

