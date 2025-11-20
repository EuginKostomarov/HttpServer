'use client'

import { useEffect } from 'react'
import { consoleInterceptor } from '@/lib/console-interceptor'
import { getConsoleInterceptorConfig } from '@/lib/console-interceptor-config'

/**
 * Компонент для инициализации перехватчика console методов
 * Автоматически активируется при монтировании компонента
 */
export function ConsoleInterceptorProvider() {
  useEffect(() => {
    // Включаем перехват console методов только в браузере
    if (typeof window !== 'undefined') {
      const config = getConsoleInterceptorConfig()
      
      // Логируем инициализацию только в debug режиме
      if (config.debug) {
        console.log('[Console Interceptor] Initializing with config:', config)
      }

      consoleInterceptor.intercept()

      // Очистка при размонтировании
      return () => {
        consoleInterceptor.restore()
        if (config.debug) {
          console.log('[Console Interceptor] Restored original console methods')
        }
      }
    }
  }, [])

  return null
}

