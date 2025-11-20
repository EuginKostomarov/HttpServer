import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url)
    const projectType = searchParams.get('project_type')

    if (!projectType) {
      return NextResponse.json(
        { error: 'project_type parameter is required' },
        { status: 400 }
      )
    }

    const url = `${BACKEND_URL}/api/classification/classifiers/by-project-type?project_type=${encodeURIComponent(projectType)}`

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
    console.error('Error fetching classifiers by project type:', error)
    return NextResponse.json(
      { error: 'Internal server error' },
      { status: 500 }
    )
  }
}

