# Console Interceptor

Утилита для перехвата и логирования всех вызовов console методов в приложении.

## Возможности

- ✅ Перехват всех console методов (log, error, warn, info, debug)
- ✅ Автоматическая отправка ошибок на сервер с rate limiting
- ✅ Безопасная сериализация объектов (обработка циклических ссылок)
- ✅ Сохранение логов в буфер для последующего анализа
- ✅ Возможность добавления кастомных обработчиков
- ✅ Автоматическая активация при загрузке приложения
- ✅ Настраиваемая конфигурация через переменные окружения
- ✅ Очередь ошибок с контекстом для лучшей отладки
- ✅ Исключение из rate limiting в middleware

## Использование

### Автоматическая активация

Перехватчик автоматически активируется при загрузке приложения через `ConsoleInterceptorProvider` в `layout.tsx`.

### Ручное использование

```typescript
import { consoleInterceptor } from '@/lib/console-interceptor'

// Включить перехват
consoleInterceptor.intercept()

// Добавить кастомный обработчик
consoleInterceptor.addHandler((entry) => {
  console.log('Custom handler:', entry)
})

// Получить все логи
const logs = consoleInterceptor.getLogs()

// Отправить логи на сервер
await consoleInterceptor.sendLogsToServer('/api/logs')

// Очистить буфер
consoleInterceptor.clearLogs()

// Отключить перехват
consoleInterceptor.restore()
```

## API Endpoints

### POST /api/logs/error

Принимает ошибки из консоли браузера и отправляет их на бэкенд.

**Request:**
```json
{
  "error": ["Error message"],
  "stack": "Error stack trace",
  "timestamp": 1234567890,
  "url": "https://example.com/page"
}
```

### POST /api/logs

Принимает массив логов из консоли браузера.

**Request:**
```json
{
  "logs": [
    {
      "method": "error",
      "args": ["Error message"],
      "timestamp": 1234567890,
      "stack": "Error stack trace"
    }
  ],
  "timestamp": 1234567890,
  "url": "https://example.com/page",
  "userAgent": "Mozilla/5.0..."
}
```

## Безопасность

- Перехватчик не блокирует выполнение приложения при ошибках
- Ошибки в обработчиках логируются, но не прерывают работу
- Объекты сериализуются безопасно (обработка циклических ссылок)
- В production отправляются только важные логи (error, warn)

## Настройка

### Через переменные окружения

```env
# Интервал между отправками ошибок (в миллисекундах)
NEXT_PUBLIC_CONSOLE_INTERCEPTOR_ERROR_INTERVAL=5000

# Включить отладочный режим
NEXT_PUBLIC_CONSOLE_INTERCEPTOR_DEBUG=true

# Отправлять логи в development режиме
NEXT_PUBLIC_CONSOLE_INTERCEPTOR_SEND_IN_DEV=false
```

### Программная настройка

```typescript
import { setConsoleInterceptorConfig } from '@/lib/console-interceptor-config'

// Настройка через глобальные переменные
setConsoleInterceptorConfig({
  errorSendInterval: 10000, // 10 секунд
  maxErrorQueueSize: 20,
  debug: true,
  sendInDevelopment: false,
})

// Настройка максимального количества сохраняемых логов
consoleInterceptor.maxLogs = 5000
```

## Rate Limiting

- Endpoint `/api/logs` исключен из rate limiting в middleware
- Ошибки отправляются с интервалом минимум 5 секунд (настраивается)
- Максимальный размер очереди ошибок: 10 (настраивается)
- При отправке ошибки включается контекст из предыдущих ошибок

