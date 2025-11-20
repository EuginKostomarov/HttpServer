import { NextRequest, NextResponse } from 'next/server'
import { violationResolveSchema, validateRequest, formatValidationError } from '@/lib/validation'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ violationId: string }> }
) {
  try {
    const { violationId } = await params

    if (!violationId || isNaN(Number(violationId))) {
      return NextResponse.json(
        { error: 'Invalid violation ID' },
        { status: 400 }
      )
    }

    const body = await request.json().catch(() => ({}))

    // Validate request body
    const validation = validateRequest(violationResolveSchema, body)
    if (!validation.success) {
      return NextResponse.json(
        {
          error: 'Validation failed',
          details: formatValidationError(validation.details)
        },
        { status: 400 }
      )
    }
    
    const backendUrl = `${BACKEND_URL}/api/quality/violations/${violationId}`
    
    console.log('Resolving violation:', backendUrl)

    const response = await fetch(backendUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(validation.data),
    })

    if (!response.ok) {
      let errorMessage = 'Failed to resolve violation'
      try {
        const errorData = await response.json()
        errorMessage = errorData.error || errorMessage
      } catch {
        errorMessage = `Backend responded with status ${response.status}`
      }
      
      console.error('Error resolving violation:', errorMessage)
      return NextResponse.json(
        { error: errorMessage },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error in quality violations resolve API route:', error)
    const errorMessage = error instanceof Error ? error.message : 'Unknown error'
    return NextResponse.json(
      { 
        error: 'Failed to connect to backend',
        details: errorMessage
      },
      { status: 500 }
    )
  }
}

