import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET() {
  try {
    const response = await fetch(`${BACKEND_URL}/api/1c/processing/xml`, {
      cache: 'no-store',
    })

    if (!response.ok) {
      const errorText = await response.text().catch(() => 'Unknown error')
      return NextResponse.json(
        { error: `Failed to fetch XML: ${errorText}` },
        { status: response.status }
      )
    }

    // Получаем XML как blob
    const blob = await response.blob()
    
    // Получаем заголовки из ответа бэкенда
    const contentType = response.headers.get('Content-Type') || 'application/xml; charset=utf-8'
    const contentDisposition = response.headers.get('Content-Disposition') || 
      `attachment; filename="1c_processing_export_${new Date().toISOString().split('T')[0].replace(/-/g, '')}.xml"`

    // Возвращаем XML файл с правильными заголовками
    return new NextResponse(blob, {
      status: 200,
      headers: {
        'Content-Type': contentType,
        'Content-Disposition': contentDisposition,
      },
    })
  } catch (error) {
    console.error('Error fetching 1C processing XML:', error)
    return NextResponse.json(
      { error: `Failed to connect to backend: ${error instanceof Error ? error.message : 'Unknown error'}` },
      { status: 500 }
    )
  }
}

