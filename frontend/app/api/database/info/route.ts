import { NextResponse } from 'next/server'
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()
const SERVICE_DB_NAME = process.env.SERVICE_DB_NAME || 'Сервисная БД'

export async function GET() {
  try {
    // Проверяем доступность backend
    const healthResponse = await fetch(`${BACKEND_URL}/health`, {
      cache: 'no-store',
    })
    
    const isBackendAvailable = healthResponse.ok
    
    // Если backend доступен, пытаемся получить информацию о БД
    if (isBackendAvailable) {
      const response = await fetch(`${BACKEND_URL}/api/database/info`, {
        cache: 'no-store',
      })

      if (response.ok) {
        const data = await response.json()
        // Преобразуем структуру ответа для совместимости с фронтендом
        return NextResponse.json({
          ...data,
          stats: {
            uploads_count: data.stats?.total_uploads || data.stats?.uploads_count || 0,
            catalogs_count: data.stats?.total_catalogs || data.stats?.catalogs_count || 0,
            items_count: data.stats?.total_items || data.stats?.items_count || 0
          }
        })
      }
    }
    
    // Если backend недоступен или endpoint не найден, возвращаем информацию о сервисной БД
    return NextResponse.json({
      name: SERVICE_DB_NAME,
      status: isBackendAvailable ? 'connected' : 'disconnected',
      path: 'service.db',
      size: 0,
      modified_at: '',
      stats: {
        uploads_count: 0,
        catalogs_count: 0,
        items_count: 0
      }
    })
  } catch (error) {
    console.error('Error fetching database info:', error)
    // Возвращаем информацию о сервисной БД при ошибке
    return NextResponse.json({
      name: SERVICE_DB_NAME,
      status: 'disconnected',
      path: 'service.db',
      size: 0,
      modified_at: '',
      stats: {
        uploads_count: 0,
        catalogs_count: 0,
        items_count: 0
      }
    })
  }
}
