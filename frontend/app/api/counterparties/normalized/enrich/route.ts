import { NextRequest, NextResponse } from 'next/server'
import { ApiErrorHandler } from '@/lib/api-error-handler'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function POST(request: NextRequest) {
  try {
    const body = await request.json()

    if (!body.counterparty_id && !body.inn && !body.bin) {
      return NextResponse.json(
        { error: 'counterparty_id or (inn/bin) is required' },
        { status: 400 }
      )
    }

    const response = await fetch(`${BACKEND_URL}/api/counterparties/normalized/enrich`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    })

    if (!response.ok) {
      const apiError = await ApiErrorHandler.handleError(response)
      ApiErrorHandler.logError('/api/counterparties/normalized/enrich', apiError, { body })
      return NextResponse.json(
        ApiErrorHandler.createErrorResponse(apiError, 'Failed to enrich counterparty'),
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    ApiErrorHandler.logError('/api/counterparties/normalized/enrich', error as Error)
    return NextResponse.json(
      ApiErrorHandler.createErrorResponse(
        error as Error,
        'Failed to connect to backend'
      ),
      { status: 500 }
    )
  }
}

