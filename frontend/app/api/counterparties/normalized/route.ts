import { NextRequest, NextResponse } from 'next/server'
import { ApiErrorHandler } from '@/lib/api-error-handler'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(request: NextRequest) {
  const endpoint = '/api/counterparties/normalized'
  
  try {
    const { searchParams } = new URL(request.url)
    const clientId = searchParams.get('client_id')
    const projectId = searchParams.get('project_id')
    const offset = searchParams.get('offset') || '0'
    const limit = searchParams.get('limit') || '20'
    const search = searchParams.get('search')
    const enrichment = searchParams.get('enrichment')
    const subcategory = searchParams.get('subcategory')

    if (!clientId && !projectId) {
      return NextResponse.json(
        { error: 'Either client_id or project_id is required' },
        { status: 400 }
      )
    }

    let url = `${BACKEND_URL}/api/counterparties/normalized?`
    if (clientId) {
      url += `client_id=${encodeURIComponent(clientId)}&`
    }
    if (projectId) {
      url += `project_id=${encodeURIComponent(projectId)}&`
    }
    url += `offset=${encodeURIComponent(offset)}&limit=${encodeURIComponent(limit)}`
    
    if (search) {
      url += `&search=${encodeURIComponent(search)}`
    }
    if (enrichment) {
      url += `&enrichment=${encodeURIComponent(enrichment)}`
    }
    if (subcategory) {
      url += `&subcategory=${encodeURIComponent(subcategory)}`
    }

    const response = await fetch(url, {
      cache: 'no-store',
      headers: {
        'Content-Type': 'application/json',
      },
    })

    if (!response.ok) {
      const apiError = await ApiErrorHandler.handleError(response)
      ApiErrorHandler.logError(endpoint, apiError, { clientId, projectId })
      return NextResponse.json(
        ApiErrorHandler.createErrorResponse(apiError, 'Failed to fetch counterparties'),
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    ApiErrorHandler.logError(endpoint, error as Error)
    return NextResponse.json(
      ApiErrorHandler.createErrorResponse(
        error as Error,
        'Failed to connect to backend'
      ),
      { status: 500 }
    )
  }
}

