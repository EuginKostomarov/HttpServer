/**
 * Утилиты для обработки ошибок API
 */

export interface ApiError {
  error: string
  details?: string
  status?: number
}

export class ApiErrorHandler {
  /**
   * Обрабатывает ошибку API запроса
   */
  static async handleError(response: Response): Promise<ApiError> {
    let errorText = 'Unknown error'
    let errorData: any = null

    try {
      const contentType = response.headers.get('content-type')
      if (contentType?.includes('application/json')) {
        errorData = await response.json()
        errorText = errorData.error || errorData.message || errorText
      } else {
        errorText = await response.text()
      }
    } catch {
      // Если не удалось прочитать ответ, используем статус
      errorText = `HTTP ${response.status}: ${response.statusText}`
    }

    return {
      error: errorText,
      details: errorData?.details || errorData?.message || undefined,
      status: response.status,
    }
  }

  /**
   * Логирует ошибку API
   */
  static logError(endpoint: string, error: ApiError | Error, context?: any) {
    const timestamp = new Date().toISOString()
    const errorInfo = error instanceof Error 
      ? { message: error.message, stack: error.stack }
      : error

    console.error(`[API Error ${timestamp}]`, {
      endpoint,
      error: errorInfo,
      context,
    })
  }

  /**
   * Создает стандартизированный ответ об ошибке
   */
  static createErrorResponse(
    error: ApiError | Error,
    defaultMessage: string = 'An error occurred'
  ): { error: string; details?: string } {
    if (error instanceof Error) {
      return {
        error: error.message || defaultMessage,
        details: error.stack,
      }
    }

    return {
      error: error.error || defaultMessage,
      details: error.details,
    }
  }
}

