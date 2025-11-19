# Анализ цепочки вызовов классификации по КПВЭД

## Обзор

Данный документ описывает полную цепочку вызовов при проведении классификации товаров по КПВЭД (Классификатор продукции по видам экономической деятельности) в системе.

## Точки входа

### 1. API Endpoint: `/api/kpved/reclassify-hierarchical`

**Файл:** `server/server.go` (строка 7383)

**Метод:** `handleKpvedReclassifyHierarchical`

**Описание:** Основной endpoint для запуска переклассификации существующих групп нормализованных данных по КПВЭД с использованием иерархического подхода.

**Входные параметры:**
```json
{
  "limit": 0  // Количество групп для переклассификации (0 = все)
}
```

### 2. API Endpoint: `/api/normalize/start` (с флагом `use_kpved`)

**Файл:** `server/server.go` (строка 3077)

**Описание:** Запускает процесс нормализации с последующей КПВЭД классификацией, если установлен флаг `use_kpved: true`.

### 3. API Endpoint: `/api/kpved/current-tasks`

**Файл:** `server/server.go` (строка 7938)

**Метод:** `handleKpvedCurrentTasks`

**Описание:** Возвращает список текущих обрабатываемых задач классификации (для мониторинга прогресса).

**Response:**
```json
{
  "current_tasks": [
    {
      "worker_id": 0,
      "normalized_name": "...",
      "category": "...",
      "merged_count": 10,
      "index": 5
    }
  ],
  "count": 1
}
```

## Полная цепочка вызовов

### Этап 1: Инициализация и валидация

#### 1.1. Обработка HTTP запроса
```
POST /api/kpved/reclassify-hierarchical
  ↓
server.handleKpvedReclassifyHierarchical()
```

**Код:** `server/server.go:7383-7395`

**Действия:**
- Проверка метода запроса (должен быть POST)
- Парсинг тела запроса (извлечение параметра `limit`)
- Установка значения по умолчанию: `limit = 10` (если не указан)

#### 1.2. Получение конфигурации AI
```
workerConfigManager.GetModelAndAPIKey()
  ↓
Возвращает: apiKey, model, error
```

**Код:** `server/server.go:7398-7404`

**Действия:**
- Получение API ключа и модели из `WorkerConfigManager`
- Валидация наличия API ключа
- Логирование используемой модели

#### 1.3. Валидация базы данных

**Код:** `server/server.go:7406-7445`

**Проверки:**
1. **Проверка наличия данных в `normalized_data`:**
   ```sql
   SELECT COUNT(DISTINCT normalized_name || '|' || category) 
   FROM normalized_data
   ```

2. **Проверка существования таблицы `kpved_classifier`:**
   ```sql
   SELECT EXISTS (
     SELECT 1 FROM sqlite_master 
     WHERE type='table' AND name='kpved_classifier'
   )
   ```

3. **Проверка количества записей в `kpved_classifier`:**
   ```sql
   SELECT COUNT(*) FROM kpved_classifier
   ```

**Важно:** Таблица `kpved_classifier` находится в сервисной БД (`serviceDB`), а `normalized_data` - в основной БД (`db`).

#### 1.4. Получение групп для классификации

**Код:** `server/server.go:7447-7470`

**SQL запрос:**
```sql
SELECT normalized_name, category, MAX(merged_count) as merged_count
FROM normalized_data
WHERE (kpved_code IS NULL OR kpved_code = '' OR TRIM(kpved_code) = '')
GROUP BY normalized_name, category
ORDER BY merged_count DESC
LIMIT ?
```

**Особенности:**
- Группировка по `normalized_name` и `category`
- Сортировка по `merged_count DESC` (сначала группы с наибольшим количеством дублей)
- Фильтрация только групп без КПВЭД классификации

### Этап 2: Создание иерархического классификатора

#### 2.1. Создание AI клиента

**Код:** `nomenclature/ai_client.go:86-112`

```go
aiClient := nomenclature.NewAIClient(apiKey, model)
```

**Компоненты:**
- **Rate Limiter:** 60 запросов в минуту (1 запрос/сек) с burst=5
- **Circuit Breaker:** 
  - 5 ошибок подряд → открытие breaker (блокировка запросов)
  - 30 секунд ожидания перед попыткой восстановления
  - 2 успешных запроса → закрытие breaker
- **HTTP Client:** таймаут 60 секунд
- **Base URL:** `https://api.arliai.com/v1/chat/completions`

#### 2.2. Создание иерархического классификатора

**Код:** `normalization/hierarchical_classifier.go:54-91`

```go
hierarchicalClassifier, err := normalization.NewHierarchicalClassifier(s.serviceDB, aiClient)
```

**Процесс инициализации:**

1. **Построение дерева КПВЭД из базы данных:**
   ```go
   tree := NewKpvedTree()
   tree.BuildFromDatabase(db)
   ```

   **Код:** `normalization/kpved_tree.go:56-133`

   **SQL запрос:**
   ```sql
   SELECT code, name, parent_code, level 
   FROM kpved_classifier 
   ORDER BY code
   ```

   **Структура дерева:**
   - **Root** - корневой узел
   - **Секции (LevelSection):** A, B, C... (21 раздел)
   - **Классы (LevelClass):** 01, 02, 03... (88 классов)
   - **Подклассы (LevelSubclass):** 01.1, 01.2...
   - **Группы (LevelGroup):** 01.11, 01.12...
   - **Подгруппы (LevelSubgroup):** 01.11.1, 01.11.2...

2. **Создание PromptBuilder:**
   ```go
   promptBuilder := NewPromptBuilder(tree)
   ```

   **Код:** `normalization/kpved_prompts.go:19-22`

3. **Инициализация кэша:**
   ```go
   cache := &sync.Map{}
   ```

4. **Установка минимального порога уверенности:**
   ```go
   minConfidence: 0.7  // 70%
   ```

### Этап 3: Параллельная обработка задач

#### 3.1. Подготовка задач

**Код:** `server/server.go:7597-7614`

**Действия:**
- Сканирование результатов SQL запроса
- Создание массива задач типа `classificationTask`:
  ```go
  type classificationTask struct {
      normalizedName string
      category       string
      mergedCount    int
      index          int
  }
  ```

#### 3.2. Настройка воркеров

**Код:** `server/server.go:7638-7669`

**Определение количества воркеров:**
- **ВАЖНО:** Arliai API поддерживает только 2 параллельных вызова
- Жесткое ограничение: максимум 2 воркера
- Минимум: 1 воркер
- Если в конфигурации указано меньше 2, используется значение из конфигурации
- Если в конфигурации указано больше 2, выводится предупреждение и ограничивается до 2

**Создание каналов:**
```go
taskChan := make(chan classificationTask, maxWorkers*2)
resultChan := make(chan classificationResult, maxWorkers*2)
```

#### 3.3. Запуск воркеров

**Код:** `server/server.go:7679-7768`

**Каждый воркер выполняет:**

1. **Получение задачи из канала:**
   ```go
   for task := range taskChan {
   ```

2. **Обновление статуса задачи:**
   ```go
   s.kpvedCurrentTasks[workerID] = &task
   ```

3. **Вызов иерархической классификации:**
   ```go
   result, err := hierarchicalClassifier.Classify(task.normalizedName, task.category)
   ```

### Этап 4: Иерархическая классификация

#### 4.1. Метод Classify

**Код:** `normalization/hierarchical_classifier.go:97-230`

**Процесс:**

1. **Проверка кэша полной классификации:**
   ```go
   cacheKey := h.getCacheKey(normalizedName, category, "")
   if cached, ok := h.cache.Load(cacheKey); ok {
       return cachedResult, nil
   }
   ```

2. **Проверка кэша корневых слов:**
   ```go
   rootWord := h.keywordClassifier.extractRootWord(normalizedName)
   if rootWord != "" {
       baseCacheKey := rootWord + "|" + category
       if cached, ok := h.baseWordCache.Load(baseCacheKey); ok {
           if cachedResult.FinalConfidence > 0.9 {
               // Сохраняем в полный кэш и возвращаем
               h.cache.Store(cacheKey, cachedResult)
               return cachedResult, nil
           }
       }
   }
   ```
   
   **Особенности:**
   - Извлечение корневого слова из нормализованного названия
   - Проверка кэша по корневому слову + категория
   - Использование только при высокой уверенности (> 0.9)

3. **Быстрая классификация по ключевым словам:**
   ```go
   if keywordResult, found := h.keywordClassifier.ClassifyByKeyword(normalizedName, category); found {
       // Сохраняем в оба кэша
       h.cache.Store(cacheKey, keywordResult)
       if rootWord != "" {
           baseCacheKey := rootWord + "|" + category
           h.baseWordCache.Store(baseCacheKey, keywordResult)
       }
       return keywordResult, nil
   }
   ```
   
   **Код:** `normalization/keyword_classifier.go:291-333`
   
   **Процесс:**
   - Извлечение корневого слова
   - Поиск паттерна в словаре ключевых слов
   - Если найден → возврат результата без вызова AI
   - Предопределенные паттерны: болт, винт, гайка, ключ, сверло, подшипник и др.
   - Обучение на успешных классификациях (автоматическое добавление новых паттернов)

4. **Шаг 1: Классификация по секциям (A-U)**
   ```go
   sectionStep, err := h.classifyLevel(normalizedName, category, LevelSection, "")
   ```
   
   - Получение кандидатов: все секции (A, B, C...)
   - Построение промпта для секций
   - Вызов AI API
   - Парсинг ответа
   - Проверка уверенности (minConfidence = 0.7)
   - Если уверенность < 0.7 → остановка, возврат результата

5. **Шаг 2: Классификация по классам (01, 02...)**
   ```go
   classStep, err := h.classifyLevel(normalizedName, category, LevelClass, sectionStep.Code)
   ```
   
   - Получение кандидатов: классы в выбранной секции
   - Построение промпта для классов
   - Вызов AI API
   - Проверка уверенности

6. **Шаг 3: Классификация по подклассам (XX.Y)**
   ```go
   subclassStep, err := h.classifyLevel(normalizedName, category, LevelSubclass, classStep.Code)
   ```

7. **Шаг 4: Классификация по группам (XX.YY)**
   ```go
   groupStep, err := h.classifyLevel(normalizedName, category, LevelGroup, subclassStep.Code)
   ```

8. **Сохранение в кэш:**
   ```go
   h.cache.Store(cacheKey, result)
   
   // Если уверенность высокая, сохраняем также в кэш корневых слов
   if result.FinalConfidence > 0.9 && rootWord != "" {
       baseCacheKey := rootWord + "|" + category
       h.baseWordCache.Store(baseCacheKey, result)
       // Обучаем классификатор ключевых слов
       h.keywordClassifier.learnFromSuccessfulClassification(
           normalizedName, category, result.FinalCode, result.FinalName, result.FinalConfidence)
   }
   ```
   
   **Особенности:**
   - Сохранение в полный кэш (по normalizedName + category)
   - Сохранение в кэш корневых слов (если уверенность > 0.9)
   - Автоматическое обучение KeywordClassifier на успешных классификациях

#### 4.2. Метод classifyLevel

**Код:** `normalization/hierarchical_classifier.go:189-257`

**Процесс для каждого уровня:**

1. **Проверка кэша уровня:**
   ```go
   levelCacheKey := h.getCacheKey(normalizedName, category, string(level)+":"+parentCode)
   ```

2. **Получение кандидатов:**
   ```go
   candidates := h.tree.GetNodesAtLevel(level, parentCode)
   ```
   
   **Код:** `normalization/kpved_tree.go:209-247`
   
   - Для секций: возвращает `Root.Children`
   - Для других уровней: возвращает детей родительского узла с указанным уровнем

3. **Построение промпта:**
   ```go
   prompt := h.promptBuilder.BuildLevelPrompt(normalizedName, category, level, candidates)
   ```
   
   **Код:** `normalization/kpved_prompts.go:24-43`
   
   **Типы промптов:**
   - `buildSectionPrompt`: для секций (A-U)
   - `buildClassPrompt`: для классов (01, 02...)
   - `buildSubclassPrompt`: для подклассов (XX.Y)
   - `buildGroupPrompt`: для групп (XX.YY)

4. **Вызов AI API:**
   ```go
   response, err := h.aiClient.GetCompletion(systemPrompt, userPrompt)
   ```
   
   **Код:** `nomenclature/ai_client.go:250-335`
   
   **Процесс:**
   - Проверка Circuit Breaker
   - Применение Rate Limiter
   - Формирование HTTP запроса:
     ```json
     {
       "model": "GLM-4.5-Air",
       "messages": [
         {"role": "system", "content": systemPrompt},
         {"role": "user", "content": userPrompt}
       ],
       "temperature": 0.3,
       "max_tokens": 1024,
       "stream": false
     }
     ```
   - Отправка POST запроса на `https://api.arliai.com/v1/chat/completions`
   - Обработка ответа
   - Обновление Circuit Breaker (успех/ошибка)

5. **Парсинг ответа AI:**
   ```go
   aiResponse, err := h.parseAIResponse(response)
   ```
   
   **Код:** `normalization/hierarchical_classifier.go:259-331`
   
   **Обработка:**
   - Удаление markdown оберток (```json, ```)
   - Исправление некорректных JSON значений:
     - Пустой `confidence` → 0.5
     - `null` → 0.5
     - Числа, начинающиеся с точки (.95 → 0.95)
     - Отсутствующие запятые между полями
   - Парсинг JSON в структуру `AIResponse`:
     ```go
     type AIResponse struct {
         SelectedCode string  `json:"selected_code"`
         Confidence   float64 `json:"confidence"`
         Reasoning    string  `json:"reasoning"`
     }
     ```
   - Валидация:
     - `selected_code` не должен быть пустым
     - `confidence` должен быть от 0 до 1 (если в процентах 0-100, конвертируется)

6. **Поиск выбранного узла в дереве:**
   ```go
   selectedNode, exists := h.tree.GetNode(aiResponse.SelectedCode)
   ```
   
   **Код:** `normalization/kpved_tree.go:249-253`

7. **Создание шага классификации:**
   ```go
   step := &ClassificationStep{
       Level:      level,
       LevelName:  GetLevelName(level),
       Code:       selectedNode.Code,
       Name:       selectedNode.Name,
       Confidence: aiResponse.Confidence,
       Reasoning:  aiResponse.Reasoning,
       Duration:   time.Since(stepStart).Milliseconds(),
   }
   ```

8. **Сохранение в кэш уровня:**
   ```go
   h.cache.Store(levelCacheKey, step)
   ```

### Этап 5: Обновление базы данных

#### 5.1. Retry логика для UPDATE

**Код:** `server/server.go:7551-7595`

**Функция `retryUpdate`:**
- Максимум попыток: 5
- Базовая задержка: 50ms
- Экспоненциальная задержка: 50ms, 100ms, 200ms, 400ms, 800ms
- Таймаут для каждого запроса: 10 секунд
- Обработка ошибок:
  - `database is locked` → retry
  - `timeout` → retry
  - Другие ошибки → немедленный возврат

#### 5.2. SQL UPDATE запрос

**Код:** `server/server.go:7728-7746`

```sql
UPDATE normalized_data
SET kpved_code = ?, 
    kpved_name = ?, 
    kpved_confidence = ?
WHERE normalized_name = ? AND category = ?
```

**Параметры:**
- `kpved_code`: `result.FinalCode`
- `kpved_name`: `result.FinalName`
- `kpved_confidence`: `result.FinalConfidence`
- `normalized_name`: `task.normalizedName`
- `category`: `task.category`

**Особенности:**
- Обновление всех записей в группе (по `normalized_name` и `category`)
- Подсчет затронутых строк (`rowsAffected`)
- Логирование результата обновления

### Этап 6: Сбор результатов

#### 6.1. Обработка результатов воркеров

**Код:** `server/server.go:7778-7850`

**Процесс:**
1. Закрытие канала результатов после завершения всех воркеров
2. Сбор результатов из канала:
   ```go
   for result := range resultChan {
   ```
3. Подсчет статистики:
   - `classified`: успешно классифицировано
   - `failed`: ошибки классификации
   - `totalDuration`: общее время выполнения
   - `avgDuration`: среднее время на группу
   - `avgSteps`: среднее количество шагов
   - `avgAICalls`: среднее количество вызовов AI
   - `totalAICalls`: общее количество вызовов AI

#### 6.2. Формирование ответа

**Код:** `server/server.go:7851-7900`

**Структура ответа:**
```json
{
  "classified": 150,
  "failed": 2,
  "total_duration": 45000,
  "avg_duration": 300,
  "avg_steps": 3.5,
  "avg_ai_calls": 3.5,
  "total_ai_calls": 525,
  "results": [
    {
      "normalized_name": "...",
      "category": "...",
      "kpved_code": "01.11",
      "kpved_name": "...",
      "confidence": 0.95,
      "steps": [...],
      "duration_ms": 300
    }
  ],
  "message": "Классификация завершена"
}
```

## Альтернативная цепочка: Нормализация с КПВЭД

### Точка входа: `/api/normalize/start`

**Код:** `server/server.go:3077-3177`

**Процесс:**

1. **Запуск нормализации:**
   - Обработка всех записей из `catalog_items`
   - Нормализация названий товаров
   - Сохранение в `normalized_data`

2. **КПВЭД классификация (если `use_kpved: true`):**
   ```go
   if req.UseKpved && s.hierarchicalClassifier != nil {
       // Получение записей без КПВЭД классификации
       rows, err := dbToUse.Query(`
           SELECT id, normalized_name, category
           FROM normalized_data
           WHERE (kpved_code IS NULL OR kpved_code = '' OR TRIM(kpved_code) = '')
       `)
       
       // Классификация каждой записи
       for _, record := range recordsToClassify {
           result, err := classifier.Classify(record.NormalizedName, record.Category)
           // Обновление записи
           dbToUse.Exec(`
               UPDATE normalized_data
               SET kpved_code = ?, kpved_name = ?, kpved_confidence = ?
               WHERE id = ?
           `, result.FinalCode, result.FinalName, result.FinalConfidence, record.ID)
       }
   }
   ```

**Отличия от переклассификации:**
- Последовательная обработка (не параллельная)
- Обработка всех записей, а не только групп
- Использование `id` для обновления, а не `normalized_name + category`

## Новые компоненты

### KeywordClassifier

**Код:** `normalization/keyword_classifier.go`

**Назначение:** Быстрая классификация по ключевым словам без вызова AI API.

**Основные методы:**

1. **`extractRootWord(normalizedName string) string`**
   - Извлекает корневое слово из нормализованного названия
   - Удаляет числа, размеры, артикулы
   - Пример: "болт м10" → "болт"

2. **`ClassifyByKeyword(normalizedName, category string) (*HierarchicalResult, bool)`**
   - Пытается классифицировать по ключевому слову
   - Проверяет паттерны в словаре
   - Возвращает результат с высокой уверенностью (> 0.9) или false

3. **`learnFromSuccessfulClassification(...)`**
   - Обучается на успешных классификациях
   - Автоматически добавляет новые паттерны
   - Обновляет статистику использования

**Предопределенные паттерны:**
- Метизы: болт, винт, гайка, шайба, саморез, заклепка
- Инструменты: ключ, сверло, фреза, коронка
- Подшипники: подшипник
- Строительные материалы: панель, уголок
- И другие...

## Структуры данных

### HierarchicalResult

**Код:** `normalization/hierarchical_classifier.go:26-35`

```go
type HierarchicalResult struct {
    FinalCode       string                `json:"final_code"`
    FinalName       string                `json:"final_name"`
    FinalConfidence float64               `json:"final_confidence"`
    Steps           []ClassificationStep  `json:"steps"`
    TotalDuration   int64                 `json:"total_duration_ms"`
    CacheHits       int                   `json:"cache_hits"`
    AICallsCount    int                   `json:"ai_calls_count"`
}
```

### ClassificationStep

**Код:** `normalization/hierarchical_classifier.go:15-24`

```go
type ClassificationStep struct {
    Level      KpvedLevel `json:"level"`
    LevelName  string     `json:"level_name"`
    Code       string     `json:"code"`
    Name       string     `json:"name"`
    Confidence float64    `json:"confidence"`
    Reasoning  string     `json:"reasoning"`
    Duration   int64      `json:"duration_ms"`
}
```

### KpvedNode

**Код:** `normalization/kpved_tree.go:28-35`

```go
type KpvedNode struct {
    Code       string       `json:"code"`
    Name       string       `json:"name"`
    Level      KpvedLevel   `json:"level"`
    ParentCode string       `json:"parent_code,omitempty"`
    Children   []*KpvedNode `json:"children,omitempty"`
}
```

## Кэширование

### Трехуровневое кэширование

1. **Кэш полной классификации:**
   - Ключ: `normalizedName:category`
   - Значение: `*HierarchicalResult`
   - Использование: для полных результатов классификации
   - Реализация: `sync.Map`

2. **Кэш корневых слов:**
   - Ключ: `rootWord|category` (например: `"болт|инструмент"`)
   - Значение: `*HierarchicalResult`
   - Использование: для быстрого поиска по корневому слову
   - Условие: используется только при уверенности > 0.9
   - Реализация: `sync.Map`

3. **Кэш уровня:**
   - Ключ: `normalizedName:category:level:parentCode`
   - Значение: `*ClassificationStep`
   - Использование: для отдельных шагов классификации
   - Реализация: `sync.Map`

**Преимущества:**
- Кэш корневых слов позволяет быстро классифицировать похожие товары без вызова AI
- Автоматическое обучение KeywordClassifier на успешных классификациях
- Снижение количества вызовов AI API

## Обработка ошибок

### Типы ошибок и их обработка

1. **Ошибки AI API:**
   - Circuit Breaker открыт → блокировка запросов
   - Rate Limiter → задержка запросов
   - Таймаут → retry с экспоненциальной задержкой
   - Неверный формат ответа → парсинг с исправлениями

2. **Ошибки базы данных:**
   - `database is locked` → retry с задержкой
   - Таймаут → retry с задержкой
   - Другие ошибки → логирование и пропуск записи

3. **Ошибки классификации:**
   - Низкая уверенность → остановка на текущем уровне
   - Отсутствие кандидатов → ошибка с детальным сообщением
   - Неверный код → валидация и исправление

## Производительность

### Оптимизации

1. **Параллельная обработка:**
   - **Ограничение:** максимум 2 воркера (из-за ограничений Arliai API)
   - Буферизованные каналы (размер = maxWorkers * 2)

2. **Трехуровневое кэширование:**
   - Кэш полных результатов
   - Кэш корневых слов (для похожих товаров)
   - Кэш отдельных шагов
   - Снижение количества вызовов AI API

3. **Классификация по ключевым словам:**
   - Быстрая классификация без вызова AI для известных паттернов
   - Предопределенные паттерны: болт, винт, гайка, ключ, сверло, подшипник и др.
   - Автоматическое обучение на успешных классификациях
   - Извлечение корневых слов для группировки похожих товаров

4. **Сортировка задач:**
   - По `merged_count DESC`
   - Сначала обрабатываются группы с наибольшим количеством дублей

5. **Retry логика:**
   - Экспоненциальная задержка (50ms, 100ms, 200ms, 400ms, 800ms)
   - Таймауты для запросов к БД (10 секунд)
   - Обработка блокировок БД
   - Максимум 5 попыток

### Метрики

- **Среднее время классификации:** ~300ms на группу (может быть меньше при использовании кэша или ключевых слов)
- **Количество шагов:** 1-4 (в зависимости от уверенности)
- **Количество вызовов AI:** 0-4 на группу (0 при использовании ключевых слов или кэша)
- **Скорость обработки:** зависит от количества воркеров (максимум 2) и скорости AI API
- **Эффективность кэша:** значительно снижает количество вызовов AI для похожих товаров

## Логирование

### Уровни логирования

1. **INFO:**
   - Запуск классификации
   - Прогресс обработки (каждые 10 записей)
   - Завершение классификации

2. **WARNING:**
   - Низкая уверенность на уровне
   - Пустой результат обновления
   - Проблемы с кэшем

3. **ERROR:**
   - Ошибки AI API
   - Ошибки базы данных
   - Ошибки парсинга ответов

### Формат логов

```
[KPVED] <сообщение>
[KPVED Worker N] <сообщение>
[HierarchicalClassifier] <сообщение>
[Level <level>] <сообщение>
[AI Call] <сообщение>
[Cache] <сообщение>
```

## Зависимости

### Внешние зависимости

1. **Arliai AI API:**
   - URL: `https://api.arliai.com/v1/chat/completions`
   - Модель: `GLM-4.5-Air` (по умолчанию)
   - Требуется API ключ

2. **База данных SQLite:**
   - Основная БД: `1c_data.db` (таблица `normalized_data`)
   - Сервисная БД: `service.db` (таблица `kpved_classifier`)

### Внутренние зависимости

1. **WorkerConfigManager:**
   - Получение API ключа и модели
   - Настройка количества воркеров

2. **KpvedTree:**
   - Построение дерева из БД
   - Поиск узлов по коду
   - Получение кандидатов для уровня

3. **PromptBuilder:**
   - Построение промптов для каждого уровня
   - Форматирование списка кандидатов

4. **AIClient:**
   - Управление HTTP запросами
   - Rate Limiting
   - Circuit Breaker
   - Парсинг ответов

## Заключение

Цепочка вызовов классификации по КПВЭД представляет собой сложную многоуровневую систему с:

- **Иерархической классификацией:** от секций до групп (4 уровня)
- **Параллельной обработкой:** максимум 2 воркера (ограничение Arliai API)
- **Трехуровневым кэшированием:** полные результаты, корневые слова, отдельные шаги
- **Классификацией по ключевым словам:** быстрая классификация без AI для известных паттернов
- **Автоматическим обучением:** KeywordClassifier обучается на успешных классификациях
- **Обработкой ошибок:** retry логика, Circuit Breaker, Rate Limiting
- **Оптимизацией производительности:** сортировка задач, буферизация каналов, кэширование

Система спроектирована для обработки больших объемов данных с минимальным количеством вызовов AI API благодаря:
- Кэшированию результатов (трехуровневое)
- Классификации по ключевым словам (избежание вызовов AI)
- Иерархическому подходу (постепенное уточнение классификации)
- Автоматическому обучению (улучшение точности со временем)

**Важные ограничения:**
- Arliai API поддерживает только 2 параллельных вызова → максимум 2 воркера
- Это ограничение жестко контролируется в коде для предотвращения ошибок rate limiting

