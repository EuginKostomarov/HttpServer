import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE = getBackendUrl()

export async function GET(request: NextRequest) {
  try {
    // Получаем параметры из URL
    const { searchParams } = new URL(request.url)
    const normalizedName = searchParams.get('normalized_name')
    const category = searchParams.get('category')

    // Проверяем обязательные параметры
    if (!normalizedName || !category) {
      return NextResponse.json(
        { error: 'normalized_name and category are required' },
        { status: 400 }
      )
    }

    const queryString = searchParams.toString()

    const response = await fetch(
      `${API_BASE}/api/normalization/group-items?${queryString}`,
      {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      }
    )

    if (!response.ok) {
      // Если бэкенд недоступен, возвращаем пустые данные
      if (response.status === 0 || response.status >= 500) {
        return NextResponse.json({
          normalized_name: normalizedName,
          normalized_reference: '',
          category: category,
          merged_count: 0,
          items: [],
        })
      }
      return NextResponse.json(
        { error: `Backend responded with ${response.status}` },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    // Бэкенд недоступен - возвращаем пустые данные
    console.error('Error fetching group items:', error)
    return NextResponse.json({
      normalized_name: '',
      normalized_reference: '',
      category: '',
      merged_count: 0,
      items: [],
    })
  }
}
