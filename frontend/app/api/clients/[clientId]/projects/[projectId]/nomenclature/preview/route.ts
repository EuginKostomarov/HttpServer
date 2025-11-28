import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'
import { logger } from '@/lib/logger'
import { handleErrorWithDetails as handleError } from '@/lib/error-handler'

const API_BASE_URL = getBackendUrl()

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ clientId: string; projectId: string }> }
) {
  const startTime = performance.now()
  
  try {
    const resolvedParams = await params
    const { clientId, projectId } = resolvedParams

    // Валидация параметров
    if (!clientId || !projectId) {
      logger.error('Missing clientId or projectId in nomenclature preview route', {
        component: 'NomenclaturePreviewAPI',
        clientId,
        projectId
      })
      
      return NextResponse.json(
        { error: 'Missing clientId or projectId' },
        { status: 400 }
      )
    }

    // Проверяем, что это числа
    const clientIdNum = parseInt(clientId, 10)
    const projectIdNum = parseInt(projectId, 10)
    
    if (isNaN(clientIdNum) || isNaN(projectIdNum)) {
      logger.error('Invalid clientId or projectId format', {
        component: 'NomenclaturePreviewAPI',
        clientId,
        projectId,
        clientIdNum,
        projectIdNum
      })
      
      return NextResponse.json(
        { error: 'Invalid clientId or projectId. Expected numeric values.' },
        { status: 400 }
      )
    }

    // Получаем query параметры
    const { searchParams } = new URL(request.url)
    const page = searchParams.get('page') || '1'
    const limit = searchParams.get('limit') || '100'
    const search = searchParams.get('search') || ''
    const databaseId = searchParams.get('database_id') || ''

    logger.debug('Fetching nomenclature preview from backend', {
      component: 'NomenclaturePreviewAPI',
      clientId: clientIdNum,
      projectId: projectIdNum,
      page,
      limit,
      search,
      databaseId,
      backendUrl: API_BASE_URL
    })

    // Формируем URL с query параметрами
    const url = new URL(`${API_BASE_URL}/api/clients/${clientIdNum}/projects/${projectIdNum}/nomenclature/preview`)
    url.searchParams.set('page', page)
    url.searchParams.set('limit', limit)
    if (search) {
      url.searchParams.set('search', search)
    }
    if (databaseId) {
      url.searchParams.set('database_id', databaseId)
    }

    const backendStartTime = performance.now()
    const response = await fetch(url.toString(), {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      signal: AbortSignal.timeout(30000), // 30 секунд таймаут
    })
    const backendDuration = performance.now() - backendStartTime

    if (!response.ok) {
      let errorText = 'Unknown error'
      let errorData: { error?: string; message?: string } = {}
      
      try {
        errorText = await response.text()
        try {
          errorData = JSON.parse(errorText)
        } catch {
          // Если не JSON, используем текст
        }
      } catch (err) {
        logger.warn('Failed to read error response from backend', {
          component: 'NomenclaturePreviewAPI',
          status: response.status,
          statusText: response.statusText
        })
      }

      const errorMessage = errorData.error || errorData.message || errorText || `HTTP ${response.status}`
      
      logger.error('Backend returned error in nomenclature preview', {
        component: 'NomenclaturePreviewAPI',
        clientId: clientIdNum,
        projectId: projectIdNum,
        status: response.status,
        statusText: response.statusText,
        error: errorMessage,
        backendDuration: `${backendDuration.toFixed(2)}ms`
      })

      return NextResponse.json(
        { error: errorMessage },
        { status: response.status }
      )
    }

    const data = await response.json()
    const totalDuration = performance.now() - startTime

    logger.info('Nomenclature preview fetched successfully', {
      component: 'NomenclaturePreviewAPI',
      clientId: clientIdNum,
      projectId: projectIdNum,
      totalRecords: data.total,
      recordsReturned: data.records?.length || 0,
      backendDuration: `${backendDuration.toFixed(2)}ms`,
      totalDuration: `${totalDuration.toFixed(2)}ms`
    })

    return NextResponse.json(data)
  } catch (error) {
    const duration = performance.now() - startTime
    const errorDetails = handleError(
      error,
      'NomenclaturePreviewAPI',
      'GET',
      { duration: `${duration.toFixed(2)}ms` }
    )

    return NextResponse.json(
      { 
        error: errorDetails.message,
        code: errorDetails.code,
        ...(process.env.NODE_ENV === 'development' && errorDetails.context)
      },
      { status: errorDetails.statusCode || 500 }
    )
  }
}

