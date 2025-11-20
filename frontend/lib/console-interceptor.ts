/**
 * Утилита для перехвата и логирования console методов
 * Позволяет отслеживать все логи в приложении и отправлять их на сервер
 */

import { getConsoleInterceptorConfig } from './console-interceptor-config'

type ConsoleMethod = 'log' | 'error' | 'warn' | 'info' | 'debug'

interface LogEntry {
  method: ConsoleMethod
  args: any[]
  timestamp: number
  stack?: string
}

type LogHandler = (entry: LogEntry) => void

class ConsoleInterceptor {
  private originalMethods: Partial<Record<ConsoleMethod, Function>> = {}
  private handlers: LogHandler[] = []
  private isIntercepted = false
  public maxLogs = 1000
  private logs: LogEntry[] = []

  /**
   * Добавляет обработчик для перехваченных логов
   */
  addHandler(handler: LogHandler) {
    this.handlers.push(handler)
  }

  /**
   * Удаляет обработчик
   */
  removeHandler(handler: LogHandler) {
    const index = this.handlers.indexOf(handler)
    if (index > -1) {
      this.handlers.splice(index, 1)
    }
  }

  /**
   * Включает перехват console методов
   */
  intercept() {
    if (this.isIntercepted) {
      return
    }

    const methods: ConsoleMethod[] = ['log', 'error', 'warn', 'info', 'debug']

    methods.forEach((methodName) => {
      const originalMethod = console[methodName] as Function
      this.originalMethods[methodName] = originalMethod

      const interceptor = this
      const wrapperMethod = function (
        this: typeof console,
        ...args: any[]
      ) {
        // Вызываем оригинальный метод
        originalMethod.apply(this, args)

        // Создаем запись лога
        const seen = new WeakSet<object>()
        const entry: LogEntry = {
          method: methodName,
          args: args.map((arg) => {
            // Сериализуем объекты для безопасного логирования
            try {
              if (typeof arg === 'object' && arg !== null) {
                return JSON.parse(JSON.stringify(arg, (key, value) => {
                  // Исключаем циклические ссылки
                  if (typeof value === 'object' && value !== null) {
                    if (seen.has(value)) {
                      return '[Circular]'
                    }
                    seen.add(value)
                  }
                  return value
                }))
              }
              return arg
            } catch {
              return String(arg)
            }
          }),
          timestamp: Date.now(),
        }

        // Получаем stack trace для ошибок
        if (methodName === 'error') {
          try {
            const stack = new Error().stack
            if (stack) {
              entry.stack = stack
            }
          } catch {
            // Игнорируем ошибки получения stack
          }
        }

        // Вызываем все обработчики
        interceptor.handlers.forEach((handler) => {
          try {
            handler(entry)
          } catch (err) {
            // Игнорируем ошибки в обработчиках, чтобы не сломать приложение
            originalMethod.call(console, 'Console interceptor handler error:', err)
          }
        })

        // Сохраняем лог в буфер
        interceptor.logs.push(entry)
        if (interceptor.logs.length > interceptor.maxLogs) {
          interceptor.logs.shift()
        }
      }

      // Сохраняем имя метода для отладки
      Object.defineProperty(wrapperMethod, 'name', {
        value: methodName,
        writable: false,
        configurable: true,
      })

      // Заменяем метод в console
      Object.defineProperty(console, methodName, {
        value: wrapperMethod,
        writable: true,
        configurable: true,
      })
    })

    this.isIntercepted = true
  }

  /**
   * Отключает перехват console методов
   */
  restore() {
    if (!this.isIntercepted) {
      return
    }

    Object.keys(this.originalMethods).forEach((methodName) => {
      const originalMethod = this.originalMethods[methodName as ConsoleMethod]
      if (originalMethod) {
        Object.defineProperty(console, methodName, {
          value: originalMethod,
          writable: true,
          configurable: true,
        })
      }
    })

    this.originalMethods = {}
    this.isIntercepted = false
  }

  /**
   * Получает все сохраненные логи
   */
  getLogs(): LogEntry[] {
    return [...this.logs]
  }

  /**
   * Очищает буфер логов
   */
  clearLogs() {
    this.logs = []
  }

  /**
   * Отправляет логи на сервер
   */
  async sendLogsToServer(endpoint: string = '/api/logs') {
    const logs = this.getLogs()
    if (logs.length === 0) {
      return
    }

    try {
      await fetch(endpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          logs,
          timestamp: Date.now(),
          userAgent: typeof navigator !== 'undefined' ? navigator.userAgent : 'unknown',
          url: typeof window !== 'undefined' ? window.location.href : 'unknown',
        }),
      })
    } catch (error) {
      // Игнорируем ошибки отправки, чтобы не сломать приложение
      console.error('Failed to send logs to server:', error)
    }
  }
}

// Создаем глобальный экземпляр
const consoleInterceptor = new ConsoleInterceptor()

// Rate limiting для отправки ошибок
let lastErrorSendTime = 0
const config = getConsoleInterceptorConfig()
const ERROR_SEND_INTERVAL = config.errorSendInterval || 5000 // Минимум 5 секунд между отправками (настраивается)
const MAX_ERROR_QUEUE_SIZE = config.maxErrorQueueSize || 10 // Максимум ошибок в очереди
const errorQueue: LogEntry[] = []
let errorSendTimeout: NodeJS.Timeout | null = null

// Обработчик для отправки критических ошибок на сервер с rate limiting
const errorHandler: LogHandler = (entry) => {
  if (entry.method === 'error') {
    // Ограничиваем размер очереди
    if (errorQueue.length >= MAX_ERROR_QUEUE_SIZE) {
      // Удаляем старые ошибки, оставляем место для новых
      errorQueue.shift()
    }
    
    // Добавляем ошибку в очередь
    errorQueue.push(entry)

    // Проверяем, можно ли отправить сейчас
    const now = Date.now()
    const timeSinceLastSend = now - lastErrorSendTime

    if (timeSinceLastSend >= ERROR_SEND_INTERVAL) {
      // Отправляем немедленно
      sendErrorFromQueue()
    } else {
      // Планируем отправку через некоторое время
      if (!errorSendTimeout) {
        const delay = ERROR_SEND_INTERVAL - timeSinceLastSend
        errorSendTimeout = setTimeout(() => {
          sendErrorFromQueue()
          errorSendTimeout = null
        }, delay)
      }
    }
  }
}

// Функция для отправки ошибок из очереди
function sendErrorFromQueue() {
  if (errorQueue.length === 0) {
    return
  }

  // Берем последнюю ошибку (самую свежую) и несколько предыдущих для контекста
  const entriesToSend = errorQueue.slice(-3) // Последние 3 ошибки для контекста
  errorQueue.length = 0 // Очищаем очередь

  lastErrorSendTime = Date.now()

  // Отправляем ошибки (последняя + контекст)
  const primaryError = entriesToSend[entriesToSend.length - 1]
  
  fetch('/api/logs/error', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      error: primaryError.args,
      stack: primaryError.stack,
      timestamp: primaryError.timestamp,
      url: typeof window !== 'undefined' ? window.location.href : 'unknown',
      context: entriesToSend.length > 1 
        ? entriesToSend.slice(0, -1).map(e => ({
            method: e.method,
            args: e.args,
            timestamp: e.timestamp,
          }))
        : undefined,
    }),
  }).catch((err) => {
    // Игнорируем ошибки отправки, но не логируем их в консоль, чтобы избежать рекурсии
    if (config.debug) {
      // Используем оригинальный console.warn напрямую, чтобы избежать рекурсии
      const originalWarn = console.warn
      originalWarn('[Console Interceptor] Failed to send error:', err)
    }
  })
}

// Добавляем обработчик ошибок
consoleInterceptor.addHandler(errorHandler)

export { consoleInterceptor, ConsoleInterceptor }
export type { LogEntry, LogHandler, ConsoleMethod }

