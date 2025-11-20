import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()

export async function GET(request: Request) {
  try {
    const { searchParams } = new URL(request.url)
    const parent = searchParams.get('parent')
    const level = searchParams.get('level')
    const database = searchParams.get('database')

    let url = `${BACKEND_URL}/api/kpved/hierarchy`
    const params = new URLSearchParams()

    if (parent) params.append('parent', parent)
    if (level) params.append('level', level)
    if (database) params.append('database', database)

    if (params.toString()) {
      url += `?${params.toString()}`
    }

    const response = await fetch(url, {
      cache: 'no-store',
    })

    if (!response.ok) {
      let errorMessage = 'Failed to fetch KPVED hierarchy'
      try {
        const errorData = await response.json()
        errorMessage = errorData.error || errorMessage
      } catch {
        const errorText = await response.text()
        if (errorText) {
          errorMessage = errorText
        }
      }
      
      // Если таблица не существует, возвращаем пустой массив
      if (response.status === 500 && (errorMessage.includes('no such table') || errorMessage.includes('kpved_classifier'))) {
        return NextResponse.json({
          nodes: [],
          total: 0,
        })
      }
      
      return NextResponse.json(
        { error: errorMessage },
        { status: response.status }
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Error fetching KPVED hierarchy:', error)
    // При ошибке подключения возвращаем пустой массив вместо ошибки
    return NextResponse.json({
      nodes: [],
      total: 0,
    })
  }
}
