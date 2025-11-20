import { NextRequest, NextResponse } from 'next/server'
import { ApiErrorHandler } from '@/lib/api-error-handler'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function POST(request: NextRequest) {
  try {
    const body = await request.json()

    if (!body.project_id) {
      return NextResponse.json(
        { error: 'project_id is required' },
        { status: 400 }
      )
    }

    const format = body.format || 'json' // csv, json, xml

    const response = await fetch(`${BACKEND_URL}/api/counterparties/normalized/export`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        project_id: body.project_id,
        format: format,
      }),
    })

    if (!response.ok) {
      const apiError = await ApiErrorHandler.handleError(response)
      ApiErrorHandler.logError('/api/counterparties/normalized/export', apiError, { projectId: body.project_id, format })
      return NextResponse.json(
        ApiErrorHandler.createErrorResponse(apiError, 'Failed to export counterparties'),
        { status: response.status }
      )
    }

    // Для файловых форматов возвращаем blob
    if (format === 'csv' || format === 'xml') {
      const blob = await response.blob()
      const contentType = format === 'csv' ? 'text/csv' : 'application/xml'
      
      return new NextResponse(blob, {
        status: 200,
        headers: {
          'Content-Type': contentType,
          'Content-Disposition': response.headers.get('Content-Disposition') || 
            `attachment; filename=counterparties_${body.project_id}.${format}`,
        },
      })
    }

    // Для JSON возвращаем JSON
    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    ApiErrorHandler.logError('/api/counterparties/normalized/export', error as Error)
    return NextResponse.json(
      ApiErrorHandler.createErrorResponse(
        error as Error,
        'Failed to connect to backend'
      ),
      { status: 500 }
    )
  }
}

