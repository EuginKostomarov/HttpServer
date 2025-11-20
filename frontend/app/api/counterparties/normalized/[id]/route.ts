import { NextRequest, NextResponse } from 'next/server'
import { ApiErrorHandler } from '@/lib/api-error-handler'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  try {

    const response = await fetch(
      `${BACKEND_URL}/api/counterparties/normalized/${id}`,
      {
        cache: 'no-store',
        headers: {
          'Content-Type': 'application/json',
        },
      }
    )

    if (!response.ok) {
      const apiError = await ApiErrorHandler.handleError(response)
      ApiErrorHandler.logError(`/api/counterparties/normalized/${id}`, apiError, { id })
      return NextResponse.json(
        ApiErrorHandler.createErrorResponse(apiError, `Failed to fetch counterparty ${id}`),
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    ApiErrorHandler.logError(`/api/counterparties/normalized/${id}`, error as Error, { id })
    return NextResponse.json(
      ApiErrorHandler.createErrorResponse(
        error as Error,
        'Failed to connect to backend'
      ),
      { status: 500 }
    )
  }
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params
  try {
    const body = await request.json()

    const response = await fetch(
      `${BACKEND_URL}/api/counterparties/normalized/${id}`,
      {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      }
    )

    if (!response.ok) {
      const apiError = await ApiErrorHandler.handleError(response)
      ApiErrorHandler.logError(`/api/counterparties/normalized/${id}`, apiError, { id, body })
      return NextResponse.json(
        ApiErrorHandler.createErrorResponse(apiError, `Failed to update counterparty ${id}`),
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    ApiErrorHandler.logError(`/api/counterparties/normalized/${id}`, error as Error, { id })
    return NextResponse.json(
      ApiErrorHandler.createErrorResponse(
        error as Error,
        'Failed to connect to backend'
      ),
      { status: 500 }
    )
  }
}

