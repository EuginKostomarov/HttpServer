import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(request: NextRequest) {
  try {
    const searchParams = request.nextUrl.searchParams
    const filePath = searchParams.get('file_path')

    if (!filePath) {
      return NextResponse.json(
        { error: 'file_path parameter is required' },
        { status: 400 }
      )
    }

    const backendUrl = new URL(`${BACKEND_URL}/api/databases/find-project`)
    backendUrl.searchParams.append('file_path', filePath)

    const response = await fetch(backendUrl.toString(), {
      cache: 'no-store',
    })

    if (!response.ok) {
      return NextResponse.json(
        { error: 'Failed to find project' },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error finding project:', error)
    return NextResponse.json(
      { error: 'Internal server error' },
      { status: 500 }
    )
  }
}

