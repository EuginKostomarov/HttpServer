import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE_URL = getBackendUrl()

export async function GET() {
  try {
    const response = await fetch(`${API_BASE_URL}/api/clients`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      cache: 'no-store',
    })

    if (!response.ok) {
      // Если backend недоступен или возвращает 404, возвращаем пустой список
      if (response.status === 404) {
        console.warn('Backend endpoint /api/clients not found. Backend may need to be restarted.')
        return NextResponse.json([])
      }
      const errorText = await response.text().catch(() => 'Unknown error')
      console.error(`Backend error (${response.status}):`, errorText)
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching clients:', error)
    // Возвращаем пустой список вместо ошибки, чтобы UI не ломался
    return NextResponse.json([])
  }
}

export async function POST(request: Request) {
  try {
    const body = await request.json()
    
    const response = await fetch(`${API_BASE_URL}/api/clients`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    })

    if (!response.ok) {
      if (response.status === 404) {
        const errorMsg = 'Backend endpoint not found. Please restart the backend server to apply changes.'
        console.error(errorMsg)
        return NextResponse.json(
          { error: errorMsg },
          { status: 503 }
        )
      }
      const errorData = await response.json().catch(() => ({}))
      const errorText = await response.text().catch(() => '')
      console.error(`Backend error (${response.status}):`, errorData || errorText)
      throw new Error(errorData.error || errorText || `HTTP error! status: ${response.status}`)
    }

    const data = await response.json()
    return NextResponse.json(data, { status: 201 })
  } catch (error) {
    console.error('Error creating client:', error)
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to create client' },
      { status: 500 }
    )
  }
}

