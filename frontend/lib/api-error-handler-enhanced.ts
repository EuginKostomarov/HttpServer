/**
 * Расширенный обработчик ошибок для API routes
 * Предоставляет единообразную обработку ошибок и логирование
 */

import { NextResponse } from 'next/server'

export interface ApiError {
  message: string
  status: number
  code?: string
  details?: any
}

export class ApiErrorHandler {
  /**
   * Обрабатывает ошибку ответа от бэкенда
   */
  static async handleError(response: Response): Promise<ApiError> {
    let errorMessage = `HTTP error! status: ${response.status}`
    let errorDetails: any = null

    try {
      const contentType = response.headers.get('content-type')
      if (contentType && contentType.includes('application/json')) {
        const errorData = await response.json()
        errorMessage = errorData.error || errorData.message || errorMessage
        errorDetails = errorData
      } else {
        const errorText = await response.text()
        if (errorText) {
          errorMessage = errorText
        }
      }
    } catch {
      // Если не удалось распарсить ответ, используем статус код
      errorMessage = this.getDefaultErrorMessage(response.status)
    }

    return {
      message: errorMessage,
      status: response.status,
      details: errorDetails,
    }
  }

  /**
   * Обрабатывает исключение
   */
  static handleException(error: unknown): ApiError {
    if (error instanceof Error) {
      return {
        message: error.message,
        status: 500,
        code: error.name,
      }
    }

    return {
      message: 'Unknown error occurred',
      status: 500,
    }
  }

  /**
   * Создает JSON ответ с ошибкой
   */
  static createErrorResponse(
    error: ApiError | Error | unknown,
    defaultMessage?: string
  ): NextResponse {
    const apiError = error instanceof Error 
      ? this.handleException(error)
      : (error as ApiError)

    const message = apiError.message || defaultMessage || 'An error occurred'
    const status = apiError.status || 500

    return NextResponse.json(
      {
        error: message,
        status,
        code: apiError.code,
        ...(apiError.details && { details: apiError.details }),
      },
      { status }
    )
  }

  /**
   * Логирует ошибку
   */
  static logError(
    endpoint: string,
    error: ApiError | Error | unknown,
    context?: Record<string, any>
  ): void {
    const apiError = error instanceof Error
      ? this.handleException(error)
      : (error as ApiError)

    console.error(`[API Error] ${endpoint}:`, {
      message: apiError.message,
      status: apiError.status,
      code: apiError.code,
      context,
      timestamp: new Date().toISOString(),
    })
  }

  /**
   * Получает понятное сообщение об ошибке по статус коду
   */
  static getDefaultErrorMessage(status: number): string {
    const messages: Record<number, string> = {
      400: 'Некорректный запрос. Проверьте параметры запроса',
      401: 'Требуется аутентификация',
      403: 'Доступ запрещен',
      404: 'Ресурс не найден',
      408: 'Время ожидания истекло',
      409: 'Конфликт данных',
      413: 'Файл слишком большой',
      422: 'Ошибка валидации данных',
      429: 'Слишком много запросов. Попробуйте позже',
      500: 'Внутренняя ошибка сервера. Проверьте логи сервера',
      502: 'Ошибка шлюза. Бэкенд недоступен',
      503: 'Сервис временно недоступен',
      504: 'Таймаут шлюза',
    }

    return messages[status] || `Ошибка сервера: ${status}`
  }

  /**
   * Улучшает сообщение об ошибке для пользователя
   */
  static enhanceErrorMessage(message: string): string {
    // Специальные сообщения для известных ошибок
    if (message.includes('ARLIAI_API_KEY') || message.includes('API key')) {
      return 'API ключ Arliai не настроен. Настройте его в разделе "Воркеры" или установите переменную окружения ARLIAI_API_KEY'
    }

    if (message.includes('No models available')) {
      return 'Нет доступных моделей для тестирования. Проверьте конфигурацию воркеров'
    }

    if (message.includes('Failed to get models')) {
      return 'Не удалось получить список моделей. Проверьте конфигурацию'
    }

    if (message.includes('Failed to fetch') || message.includes('NetworkError')) {
      return 'Ошибка подключения к серверу. Проверьте, что бэкенд запущен'
    }

    if (message.includes('timeout') || message.includes('aborted')) {
      return 'Время ожидания истекло. Файл может быть слишком большим. Попробуйте еще раз'
    }

    return message
  }

  /**
   * Обрабатывает ошибку с улучшенным сообщением
   */
  static async handleErrorWithEnhancement(
    response: Response,
    endpoint: string,
    context?: Record<string, any>
  ): Promise<NextResponse> {
    const error = await this.handleError(response)
    this.logError(endpoint, error, context)

    const enhancedMessage = this.enhanceErrorMessage(error.message)

    return NextResponse.json(
      {
        error: enhancedMessage,
        status: error.status,
        ...(error.details && { details: error.details }),
      },
      { status: error.status }
    )
  }
}

