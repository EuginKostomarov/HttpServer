/**
 * Утилита для получения конфигурации API
 * Унифицирует получение BACKEND_URL из переменных окружения
 */

export function getBackendUrl(): string {
  return (
    process.env.BACKEND_URL ||
    process.env.NEXT_PUBLIC_BACKEND_URL ||
    'http://localhost:9999'
  )
}

/**
 * Создает полный URL для API эндпоинта
 */
export function getApiUrl(endpoint: string): string {
  const baseUrl = getBackendUrl()
  // Убираем ведущий слэш, если есть
  const cleanEndpoint = endpoint.startsWith('/') ? endpoint.slice(1) : endpoint
  return `${baseUrl}/${cleanEndpoint}`
}

/**
 * Проверяет, доступен ли бэкенд
 */
export async function checkBackendHealth(): Promise<boolean> {
  try {
    const response = await fetch(`${getBackendUrl()}/api/health`, {
      method: 'GET',
      cache: 'no-store',
      signal: AbortSignal.timeout(5000), // 5 секунд таймаут
    })
    return response.ok
  } catch {
    return false
  }
}

