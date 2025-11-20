import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const API_BASE_URL = getBackendUrl()

export async function GET(
  request: Request,
  { params }: { params: Promise<{ clientId: string; projectId: string }> }
) {
  try {
    const { clientId, projectId } = await params
    const response = await fetch(`${API_BASE_URL}/api/clients/${clientId}/projects/${projectId}/normalization/status`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
      cache: 'no-store',
    })

    if (!response.ok) {
      // Возвращаем дефолтные значения если endpoint не найден
      return NextResponse.json({
        total_processed: 0,
        total_groups: 0,
        benchmark_matches: 0,
        ai_enhanced: 0,
        basic_normalized: 0,
        is_running: false
      })
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching normalization status:', error)
    return NextResponse.json({
      total_processed: 0,
      total_groups: 0,
      benchmark_matches: 0,
      ai_enhanced: 0,
      basic_normalized: 0,
      is_running: false
    })
  }
}

