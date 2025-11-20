# Улучшения AI классификатора - Фаза 2

## Обзор

Добавлены дополнительные улучшения для AI классификатора: конфигурируемые параметры, метрики производительности и расширенное управление логированием.

## Новые возможности

### 1. Конфигурируемые параметры

Классификатор теперь поддерживает настройку через структуру `AIClassifierConfig`:

```go
type AIClassifierConfig struct {
    MaxCategories      int  // Максимальное количество категорий (по умолчанию 15)
    MaxCategoryNameLen int  // Максимальная длина названия (по умолчанию 50)
    EnableLogging      bool // Включить логирование (по умолчанию true)
}
```

**Методы управления конфигурацией:**
- `GetConfig()` - получить текущую конфигурацию
- `SetConfig(config)` - установить новую конфигурацию (автоматически сбрасывает кэш)

### 2. Переменные окружения

Параметры можно настраивать через переменные окружения:

```bash
# Максимальное количество категорий в списке
export AI_CLASSIFIER_MAX_CATEGORIES=10

# Максимальная длина названия категории
export AI_CLASSIFIER_MAX_NAME_LEN=30

# Включить/выключить логирование
export AI_CLASSIFIER_ENABLE_LOGGING=false
```

### 3. Метрики производительности

Добавлен сбор метрик производительности:

```go
// Получить статистику производительности
totalRequests, avgLatency := aiClassifier.GetPerformanceStats()

fmt.Printf("Total requests: %d\n", totalRequests)
fmt.Printf("Average latency: %v\n", avgLatency)
```

**Метрики:**
- `totalRequests` - общее количество запросов
- `avgLatency` - средняя задержка выполнения

### 4. Улучшенное логирование

Логирование теперь управляется через конфигурацию:

- При `EnableLogging = true`: детальное логирование всех операций
- При `EnableLogging = false`: только критические ошибки

**Примеры логов:**
```
[AIClassifier] Prompt size: 1234 bytes, estimated tokens: ~411
[AIClassifier] System prompt size: 78 bytes, estimated tokens: ~26
[AIClassifier] Cache HIT: category list reused (hits: 5, misses: 1)
[AIClassifier] Cache MISS: category list generated (hits: 5, misses: 2)
[AIClassifier] Classification completed in 234ms
```

### 5. Обновленный API эндпоинт

Эндпоинт `/api/classification/optimization-stats` теперь включает:
- Информацию о конфигурации
- Описание переменных окружения
- Статус метрик производительности

## Использование

### Базовое использование

```go
// Создание классификатора (конфигурация загружается из env)
aiClassifier := classification.NewAIClassifier(apiKey, model)

// Установка дерева классификатора
aiClassifier.SetClassifierTree(tree)

// Классификация
response, err := aiClassifier.ClassifyWithAI(request)
```

### Настройка конфигурации

```go
// Получить текущую конфигурацию
config := aiClassifier.GetConfig()
fmt.Printf("Max categories: %d\n", config.MaxCategories)

// Изменить конфигурацию
newConfig := classification.AIClassifierConfig{
    MaxCategories:      10,
    MaxCategoryNameLen: 30,
    EnableLogging:      false,
}
aiClassifier.SetConfig(newConfig)
```

### Мониторинг производительности

```go
// Получить статистику кэша
hits, misses := aiClassifier.GetCacheStats()
hitRate := float64(hits) / float64(hits+misses) * 100.0
fmt.Printf("Cache hit rate: %.2f%%\n", hitRate)

// Получить статистику производительности
requests, avgLatency := aiClassifier.GetPerformanceStats()
fmt.Printf("Total requests: %d, Avg latency: %v\n", requests, avgLatency)
```

## Тестирование

Добавлены новые тесты:
- `TestAIClassifierConfig` - тест конфигурации
- `TestAIClassifierPerformanceStats` - тест метрик производительности

Все тесты проходят успешно.

## Примеры конфигурации

### Минимальный размер промпта
```bash
export AI_CLASSIFIER_MAX_CATEGORIES=5
export AI_CLASSIFIER_MAX_NAME_LEN=30
export AI_CLASSIFIER_ENABLE_LOGGING=false
```

### Максимальная детализация
```bash
export AI_CLASSIFIER_MAX_CATEGORIES=20
export AI_CLASSIFIER_MAX_NAME_LEN=100
export AI_CLASSIFIER_ENABLE_LOGGING=true
```

### Сбалансированная конфигурация (по умолчанию)
```bash
export AI_CLASSIFIER_MAX_CATEGORIES=15
export AI_CLASSIFIER_MAX_NAME_LEN=50
export AI_CLASSIFIER_ENABLE_LOGGING=true
```

## Совместимость

Все изменения обратно совместимы:
- Если переменные окружения не установлены, используются значения по умолчанию
- Существующий код продолжает работать без изменений
- Новые функции опциональны

## Файлы изменений

- `classification/ai_classifier.go` - основные улучшения
- `classification/classifier_test.go` - новые тесты
- `server/server_classification.go` - обновленный API эндпоинт
- `OPTIMIZATION_IMPLEMENTATION.md` - обновленная документация

