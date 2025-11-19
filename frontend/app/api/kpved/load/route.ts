import { NextRequest, NextResponse } from 'next/server'
import { kpvedLoadSchema, validateRequest, formatValidationError } from '@/lib/validation'

const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:9999'

export async function POST(request: NextRequest) {
  try {
    const body = await request.json()

    // Validate request body
    const validation = validateRequest(kpvedLoadSchema, body)
    if (!validation.success) {
      return NextResponse.json(
        {
          error: 'Validation failed',
          details: formatValidationError(validation.details)
        },
        { status: 400 }
      )
    }

    const response = await fetch(`${BACKEND_URL}/api/kpved/load`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(validation.data),
    })

    if (!response.ok) {
      const errorText = await response.text()
      console.error('Error loading KPVED:', errorText)
      return new NextResponse(errorText, { status: response.status })
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error loading KPVED:', error)
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to load KPVED' },
      { status: 500 }
    )
  }
}

