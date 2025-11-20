import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE = getBackendUrl()

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params

    if (!id || isNaN(Number(id))) {
      return NextResponse.json(
        { error: 'Invalid item ID' },
        { status: 400 }
      )
    }

    const response = await fetch(
      `${API_BASE}/api/normalization/item-attributes/${id}`,
      {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      }
    )

    if (!response.ok) {
      // Если бэкенд недоступен, возвращаем пустой список атрибутов
      if (response.status === 0 || response.status >= 500) {
        return NextResponse.json({
          item_id: Number(id),
          attributes: [],
          count: 0,
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
    // Бэкенд недоступен - возвращаем пустой список атрибутов
    console.error('Error fetching item attributes:', error)
    const { id } = await params
    return NextResponse.json({
      item_id: Number(id),
      attributes: [],
      count: 0,
    })
  }
}

