import { NextRequest, NextResponse } from 'next/server'
import { ApiErrorHandler } from '@/lib/api-error-handler'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url)
    const projectId = searchParams.get('project_id')

    if (!projectId) {
      return NextResponse.json(
        { error: 'project_id parameter is required' },
        { status: 400 }
      )
    }

    const url = `${BACKEND_URL}/api/counterparties/normalized/duplicates?project_id=${encodeURIComponent(projectId)}`

    const response = await fetch(url, {
      cache: 'no-store',
      headers: {
        'Content-Type': 'application/json',
      },
    })

    if (!response.ok) {
      const apiError = await ApiErrorHandler.handleError(response)
      ApiErrorHandler.logError('/api/counterparties/normalized/duplicates', apiError, { projectId })
      return NextResponse.json(
        ApiErrorHandler.createErrorResponse(apiError, 'Failed to fetch counterparty duplicates'),
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    ApiErrorHandler.logError('/api/counterparties/normalized/duplicates', error as Error)
    return NextResponse.json(
      ApiErrorHandler.createErrorResponse(
        error as Error,
        'Failed to connect to backend'
      ),
      { status: 500 }
    )
  }
}

