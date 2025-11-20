import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE = getBackendUrl()

export async function GET(request: NextRequest) {
  try {
    // Получаем параметры из URL
    const { searchParams } = new URL(request.url)
    const normalizedName = searchParams.get('normalized_name')
    const category = searchParams.get('category')
    // format передается в queryString через searchParams и используется бэкендом

    // Проверяем обязательные параметры
    if (!normalizedName || !category) {
      return NextResponse.json(
        { error: 'normalized_name and category are required' },
        { status: 400 }
      )
    }

    const queryString = searchParams.toString()

    const response = await fetch(
      `${API_BASE}/api/normalization/export-group?${queryString}`,
      {
        method: 'GET',
      }
    )

    if (!response.ok) {
      return NextResponse.json(
        { error: `Backend responded with ${response.status}` },
        { status: response.status }
      )
    }

    // Получаем данные из бэкенда
    const data = await response.blob()

    // Получаем заголовки из ответа бэкенда
    const contentType = response.headers.get('Content-Type')
    const contentDisposition = response.headers.get('Content-Disposition')

    // Создаем новый ответ с правильными заголовками
    const headers = new Headers()
    if (contentType) headers.set('Content-Type', contentType)
    if (contentDisposition) headers.set('Content-Disposition', contentDisposition)

    return new NextResponse(data, { headers })
  } catch (error) {
    console.error('Error exporting group:', error)
    return NextResponse.json(
      { error: 'Failed to export group' },
      { status: 500 }
    )
  }
}
