/**
 * Конфигурация для Console Interceptor
 * Можно настроить через переменные окружения или глобальные переменные
 */

export interface ConsoleInterceptorConfig {
  /**
   * Интервал между отправками ошибок на сервер (в миллисекундах)
   * @default 5000
   */
  errorSendInterval?: number

  /**
   * Максимальный размер очереди ошибок
   * @default 10
   */
  maxErrorQueueSize?: number

  /**
   * Включить отладочный режим
   * @default false
   */
  debug?: boolean

  /**
   * Отправлять ли логи в development режиме
   * @default false
   */
  sendInDevelopment?: boolean
}

/**
 * Получает конфигурацию из переменных окружения или глобальных переменных
 */
export function getConsoleInterceptorConfig(): ConsoleInterceptorConfig {
  const config: ConsoleInterceptorConfig = {}

  // Проверяем глобальные переменные (устанавливаются через window)
  if (typeof window !== 'undefined') {
    const globalConfig = (window as any).__CONSOLE_INTERCEPTOR_CONFIG__
    if (globalConfig) {
      Object.assign(config, globalConfig)
    }
  }

  // Проверяем переменные окружения
  if (typeof process !== 'undefined' && process.env) {
    if (process.env.NEXT_PUBLIC_CONSOLE_INTERCEPTOR_ERROR_INTERVAL) {
      config.errorSendInterval = parseInt(
        process.env.NEXT_PUBLIC_CONSOLE_INTERCEPTOR_ERROR_INTERVAL,
        10
      )
    }

    if (process.env.NEXT_PUBLIC_CONSOLE_INTERCEPTOR_DEBUG === 'true') {
      config.debug = true
    }

    if (process.env.NEXT_PUBLIC_CONSOLE_INTERCEPTOR_SEND_IN_DEV === 'true') {
      config.sendInDevelopment = true
    }
  }

  return config
}

/**
 * Устанавливает конфигурацию глобально (для использования в браузере)
 */
export function setConsoleInterceptorConfig(config: ConsoleInterceptorConfig) {
  if (typeof window !== 'undefined') {
    (window as any).__CONSOLE_INTERCEPTOR_CONFIG__ = config
    if (config.debug) {
      (window as any).__CONSOLE_INTERCEPTOR_DEBUG__ = true
    }
  }
}

