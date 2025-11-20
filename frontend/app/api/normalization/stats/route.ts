import { NextRequest, NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE = getBackendUrl()

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export async function GET(_request: NextRequest) {
  try {
    const response = await fetch(`${API_BASE}/api/normalization/stats`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      // Таймаут обрабатывается через catch блок
    })

    if (!response.ok) {
      // Если бэкенд недоступен, возвращаем дефолтную статистику
      if (response.status === 0 || response.status >= 500) {
        return NextResponse.json({
          total_processed: 0,
          total_groups: 0,
          total_merged: 0,
          categories: {},
        })
      }
      return NextResponse.json(
        { error: `Backend responded with ${response.status}` },
        { status: response.status }
      )
    }

    const data = await response.json()
    // Преобразуем структуру ответа для совместимости с фронтендом
    return NextResponse.json({
      total_processed: data.totalItems || data.total_processed || 0,
      total_groups: data.totalGroups || data.total_groups || 0,
      total_merged: data.mergedItems || data.total_merged || 0,
      ...data // Сохраняем остальные поля
    })
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
  } catch (_error) {
    // Бэкенд недоступен - возвращаем дефолтную статистику
    return NextResponse.json({
      total_processed: 0,
      total_groups: 0,
      total_merged: 0,
      categories: {},
    })
  }
}

