import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ groupId: string }> }
) {
  try {
    const { groupId } = await params

    if (!groupId || isNaN(Number(groupId))) {
      return NextResponse.json(
        { error: 'Invalid group ID' },
        { status: 400 }
      )
    }

    const backendUrl = `${BACKEND_URL}/api/quality/duplicates/${groupId}/merge`
    
    console.log(`Proxying POST /api/quality/duplicates/${groupId}/merge to ${backendUrl}`)

    const response = await fetch(backendUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
    })

    if (!response.ok) {
      let errorMessage = 'Failed to merge duplicate group'
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
      
      console.error('Error merging duplicate group:', errorMessage)
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
    console.error('Error in quality duplicates merge API route:', error)
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

