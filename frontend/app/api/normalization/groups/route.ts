import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE = getBackendUrl()

export async function GET(request: NextRequest) {
  try {
    // Получаем параметры из URL
    const { searchParams } = new URL(request.url)
    const queryString = searchParams.toString()

    const response = await fetch(
      `${API_BASE}/api/normalization/groups${queryString ? `?${queryString}` : ''}`,
      {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      }
    )

    if (!response.ok) {
      // Если бэкенд недоступен, возвращаем пустой список
      if (response.status === 0 || response.status >= 500) {
        return NextResponse.json({
          groups: [],
          total: 0,
          page: 1,
          limit: 20,
          totalPages: 0,
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
    // Бэкенд недоступен - возвращаем пустой список
    console.error('Error fetching groups:', error)
    return NextResponse.json({
      groups: [],
      total: 0,
      page: 1,
      limit: 20,
      totalPages: 0,
    })
  }
}
