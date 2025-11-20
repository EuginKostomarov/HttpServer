# 🔧 Диагностика проблемы "Failed to run benchmark"

## Возможные причины ошибки

### 1. API ключ не настроен
**Ошибка:** `ARLIAI_API_KEY not configured`

**Решение:**
```bash
# Windows PowerShell
$env:ARLIAI_API_KEY="your-api-key-here"

# Linux/Mac
export ARLIAI_API_KEY="your-api-key-here"
```

### 2. Бэкенд недоступен
**Ошибка:** Сеть недоступна или таймаут

**Решение:**
1. Проверьте, что бэкенд запущен на порту 9999
2. Проверьте URL в переменной окружения `NEXT_PUBLIC_BACKEND_URL`
3. Проверьте файрвол и сетевые настройки

### 3. Нет доступных моделей
**Ошибка:** `No models available`

**Решение:**
1. Проверьте конфигурацию моделей в бэкенде
2. Убедитесь, что модели правильно настроены в конфигурации воркеров

### 4. Внутренняя ошибка сервера
**Ошибка:** HTTP 500

**Решение:**
1. Проверьте логи сервера
2. Убедитесь, что все зависимости установлены
3. Проверьте подключение к базе данных

## Как проверить проблему

### В консоли браузера (F12)
```javascript
// Проверка доступности API
fetch('http://localhost:9999/api/models/benchmark', {
  method: 'GET'
})
  .then(r => r.json())
  .then(console.log)
  .catch(console.error)
```

### Проверка переменных окружения
```bash
# Проверка API ключа
echo $ARLIAI_API_KEY  # Linux/Mac
echo %ARLIAI_API_KEY%  # Windows CMD
$env:ARLIAI_API_KEY    # Windows PowerShell
```

## Улучшенная обработка ошибок

Добавьте в `frontend/app/models/benchmark/page.tsx`:

```typescript
const runBenchmark = async () => {
  try {
    setRunning(true)
    const response = await fetch(`${API_BASE}/api/models/benchmark`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        auto_update_priorities: autoUpdatePriorities,
      }),
    })

    if (!response.ok) {
      let errorMessage = "Не удалось запустить бенчмарк"
      
      try {
        const errorData = await response.json()
        errorMessage = errorData.error || errorData.message || errorMessage
        
        // Специальные сообщения для известных ошибок
        if (errorMessage.includes("ARLIAI_API_KEY")) {
          errorMessage = "API ключ Arliai не настроен. Установите переменную окружения ARLIAI_API_KEY"
        } else if (errorMessage.includes("No models available")) {
          errorMessage = "Нет доступных моделей для тестирования"
        } else if (errorMessage.includes("Failed to get models")) {
          errorMessage = "Не удалось получить список моделей. Проверьте конфигурацию"
        }
      } catch (e) {
        // Если не удалось распарсить JSON, используем статус код
        if (response.status === 503) {
          errorMessage = "Сервис временно недоступен. Проверьте настройки API ключа"
        } else if (response.status === 404) {
          errorMessage = "Эндпоинт не найден. Проверьте версию API"
        } else if (response.status === 500) {
          errorMessage = "Внутренняя ошибка сервера. Проверьте логи сервера"
        } else {
          errorMessage = `Ошибка сервера: ${response.status} ${response.statusText}`
        }
      }
      
      throw new Error(errorMessage)
    }

    const data: BenchmarkResponse = await response.json()
    setBenchmarks(data.models || [])
    setTimestamp(data.timestamp || "")

    let message = `Бенчмарк завершен. Протестировано ${data.total} моделей`
    if ((data as any).priorities_updated) {
      message += ". Приоритеты моделей обновлены автоматически."
    }
    toast.success(message)
  } catch (error: any) {
    console.error("Error running benchmark:", error)
    const errorMessage = error.message || "Не удалось запустить бенчмарк"
    toast.error(errorMessage, {
      duration: 5000,
      description: "Проверьте консоль браузера для подробностей"
    })
  } finally {
    setRunning(false)
  }
}
```

## Быстрая проверка

1. **Проверьте бэкенд:**
   ```bash
   curl http://localhost:9999/api/models/benchmark
   ```

2. **Проверьте API ключ:**
   ```bash
   # В терминале, где запущен бэкенд
   echo $ARLIAI_API_KEY
   ```

3. **Проверьте логи бэкенда:**
   - Ищите сообщения об ошибках при запуске бенчмарка
   - Проверьте, что модели доступны

## Частые проблемы и решения

| Проблема | Решение |
|----------|---------|
| `ARLIAI_API_KEY not configured` | Установите переменную окружения |
| `No models available` | Проверьте конфигурацию моделей |
| `Failed to get models` | Проверьте подключение к API Arliai |
| Таймаут | Увеличьте таймаут запроса или проверьте сеть |
| CORS ошибка | Проверьте настройки CORS на бэкенде |

