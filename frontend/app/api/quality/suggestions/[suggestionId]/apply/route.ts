import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ suggestionId: string }> }
) {
  try {
    const { suggestionId } = await params

    if (!suggestionId || isNaN(Number(suggestionId))) {
      return NextResponse.json(
        { error: 'Invalid suggestion ID' },
        { status: 400 }
      )
    }

    const backendUrl = `${BACKEND_URL}/api/quality/suggestions/${suggestionId}/apply`
    
    console.log('Applying suggestion:', backendUrl)

    const response = await fetch(backendUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
    })

    if (!response.ok) {
      let errorMessage = 'Failed to apply suggestion'
      try {
        const errorData = await response.json()
        errorMessage = errorData.error || errorMessage
      } catch {
        errorMessage = `Backend responded with status ${response.status}`
      }
      
      console.error('Error applying suggestion:', errorMessage)
      return NextResponse.json(
        { error: errorMessage },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error in quality suggestions apply API route:', error)
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

