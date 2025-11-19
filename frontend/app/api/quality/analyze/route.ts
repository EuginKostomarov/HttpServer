import { NextRequest, NextResponse } from 'next/server'
import { qualityAnalyzeSchema, validateRequest, formatValidationError } from '@/lib/validation'

const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:9999'

export async function POST(request: NextRequest) {
  try {
    const body = await request.json()

    console.log('Proxying POST /api/quality/analyze to backend')

    // Validate request body
    const validation = validateRequest(qualityAnalyzeSchema, body)
    if (!validation.success) {
      return NextResponse.json(
        {
          error: 'Validation failed',
          details: formatValidationError(validation.details)
        },
        { status: 400 }
      )
    }

    const response = await fetch(`${BACKEND_URL}/api/quality/analyze`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(validation.data),
    })

    if (!response.ok) {
      let errorMessage = 'Failed to start analysis'
      try {
        const errorText = await response.text()
        console.error(`Backend responded with status ${response.status}:`, errorText)
        try {
          const errorData = JSON.parse(errorText)
          errorMessage = errorData.error || errorMessage
        } catch {
          errorMessage = `Backend error: ${response.status} - ${errorText}`
        }
      } catch {
        errorMessage = `Backend responded with status ${response.status}`
      }
      
      return NextResponse.json(
        { 
          error: errorMessage,
          details: errorMessage
        },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error starting quality analysis:', error)
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

