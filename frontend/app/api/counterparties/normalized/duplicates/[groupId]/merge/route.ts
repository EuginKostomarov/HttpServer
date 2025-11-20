import { NextRequest, NextResponse } from 'next/server'
import { ApiErrorHandler } from '@/lib/api-error-handler'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ groupId: string }> }
) {
  const resolvedParams = await params
  const groupId: string = resolvedParams.groupId
  
  try {
    const body = await request.json()

    if (!body.master_id) {
      return NextResponse.json(
        { error: 'master_id is required' },
        { status: 400 }
      )
    }

    if (!body.merge_ids || !Array.isArray(body.merge_ids)) {
      return NextResponse.json(
        { error: 'merge_ids array is required' },
        { status: 400 }
      )
    }

    const response = await fetch(
      `${BACKEND_URL}/api/counterparties/normalized/duplicates/${groupId}/merge`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      }
    )

    if (!response.ok) {
      const apiError = await ApiErrorHandler.handleError(response)
      ApiErrorHandler.logError(`/api/counterparties/normalized/duplicates/${groupId}/merge`, apiError, { groupId, body })
      return NextResponse.json(
        ApiErrorHandler.createErrorResponse(apiError, 'Failed to merge counterparty duplicates'),
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    ApiErrorHandler.logError(`/api/counterparties/normalized/duplicates/${groupId}/merge`, error as Error)
    return NextResponse.json(
      ApiErrorHandler.createErrorResponse(
        error as Error,
        'Failed to connect to backend'
      ),
      { status: 500 }
    )
  }
}
