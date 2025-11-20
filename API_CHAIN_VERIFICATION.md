# Проверка цепочки вызовов API

## Структура API

### Бэкенд (Go) - `server/server.go`

#### Контрагенты
- `GET /api/counterparties/normalized` - получение списка контрагентов
  - Параметры: `client_id`, `project_id`, `offset`, `limit`, `search`
  - Обработчик: `handleNormalizedCounterparties`
  
- `GET /api/counterparties/normalized/stats` - статистика контрагентов
  - Параметры: `project_id` (обязательный)
  - Обработчик: `handleNormalizedCounterpartyStats`
  
- `GET /api/counterparties/normalized/{id}` - получение контрагента по ID
  - Обработчик: `handleGetNormalizedCounterparty`
  
- `PUT /api/counterparties/normalized/{id}` - обновление контрагента
  - Обработчик: `handleUpdateNormalizedCounterparty`
  
- `POST /api/counterparties/normalized/enrich` - ручное обогащение
  - Обработчик: `handleEnrichCounterparty`
  
- `GET /api/counterparties/normalized/duplicates` - получение дубликатов
  - Обработчик: `handleGetCounterpartyDuplicates`
  
- `POST /api/counterparties/normalized/duplicates/{groupId}/merge` - объединение дубликатов
  - Обработчик: `handleMergeCounterpartyDuplicates`
  
- `POST /api/counterparties/normalized/export` - экспорт контрагентов
  - Обработчик: `handleExportCounterparties`

#### Нормализация
- `GET /api/normalization/stats` - статистика нормализации
- `GET /api/normalization/groups` - группы нормализованных записей
- `GET /api/normalization/groups/{name}/{category}` - детали группы
- `POST /api/normalization/start` - запуск нормализации
- `GET /api/normalization/status` - статус нормализации
- `POST /api/normalization/stop` - остановка нормализации

#### Клиенты и проекты
- `GET /api/clients` - список клиентов
- `GET /api/clients/{id}` - детали клиента
- `GET /api/clients/{id}/projects` - проекты клиента
- `GET /api/clients/{id}/projects/{projectId}` - детали проекта

### Фронтенд (Next.js) - API Routes

#### Контрагенты
- `GET /api/counterparties/normalized` - прокси к бэкенду
  - Файл: `frontend/app/api/counterparties/normalized/route.ts`
  - Проксирует: `GET /api/counterparties/normalized`
  
- `GET /api/counterparties/normalized/stats` - прокси статистики
  - Файл: `frontend/app/api/counterparties/normalized/stats/route.ts`
  - Проксирует: `GET /api/counterparties/normalized/stats`
  
- `GET /api/counterparties/normalized/[id]` - прокси получения контрагента
  - Файл: `frontend/app/api/counterparties/normalized/[id]/route.ts`
  - Проксирует: `GET /api/counterparties/normalized/{id}`
  
- `PUT /api/counterparties/normalized/[id]` - прокси обновления
  - Файл: `frontend/app/api/counterparties/normalized/[id]/route.ts`
  - Проксирует: `PUT /api/counterparties/normalized/{id}`

## Цепочка вызовов

### Получение списка контрагентов

1. **Фронтенд компонент** (`frontend/app/clients/[clientId]/projects/[projectId]/counterparties/page.tsx`)
   ```typescript
   fetch('/api/counterparties/normalized?client_id=${clientId}&project_id=${projectId}')
   ```

2. **Next.js API Route** (`frontend/app/api/counterparties/normalized/route.ts`)
   ```typescript
   fetch(`${BACKEND_URL}/api/counterparties/normalized?client_id=${clientId}&project_id=${projectId}`)
   ```

3. **Бэкенд обработчик** (`server/server.go`)
   ```go
   handleNormalizedCounterparties(w, r)
   ```

4. **База данных** (`database/service_db.go`)
   ```go
   GetNormalizedCounterpartiesByClient(clientID, projectID, offset, limit)
   ```

### Получение статистики контрагентов

1. **Фронтенд компонент**
   ```typescript
   fetch(`/api/counterparties/normalized/stats?project_id=${projectId}`)
   ```

2. **Next.js API Route** (`frontend/app/api/counterparties/normalized/stats/route.ts`)
   ```typescript
   fetch(`${BACKEND_URL}/api/counterparties/normalized/stats?project_id=${projectId}`)
   ```

3. **Бэкенд обработчик**
   ```go
   handleNormalizedCounterpartyStats(w, r)
   ```

4. **База данных**
   ```go
   GetNormalizedCounterpartyStats(projectID)
   ```

### Обновление контрагента

1. **Фронтенд компонент**
   ```typescript
   fetch(`/api/counterparties/normalized/${id}`, {
     method: 'PUT',
     body: JSON.stringify(editForm)
   })
   ```

2. **Next.js API Route** (`frontend/app/api/counterparties/normalized/[id]/route.ts`)
   ```typescript
   fetch(`${BACKEND_URL}/api/counterparties/normalized/${id}`, {
     method: 'PUT',
     body: JSON.stringify(body)
   })
   ```

3. **Бэкенд обработчик**
   ```go
   handleUpdateNormalizedCounterparty(w, r, id)
   ```

4. **База данных**
   ```go
   UpdateNormalizedCounterparty(id, ...)
   ```

## Проверка целостности

### ✅ Проверенные связи

1. **Контрагенты**
   - ✅ GET список - фронтенд → Next.js API → бэкенд → БД
   - ✅ GET статистика - фронтенд → Next.js API → бэкенд → БД
   - ✅ GET по ID - фронтенд → Next.js API → бэкенд → БД
   - ✅ PUT обновление - фронтенд → Next.js API → бэкенд → БД

2. **Нормализация**
   - ✅ GET статистика - фронтенд → Next.js API → бэкенд → БД
   - ✅ GET группы - фронтенд → Next.js API → бэкенд → БД

3. **Клиенты и проекты**
   - ✅ GET клиенты - фронтенд → Next.js API → бэкенд → БД
   - ✅ GET проекты - фронтенд → Next.js API → бэкенд → БД

### ⚠️ Требует проверки

1. **Обогащение контрагентов**
   - POST /api/counterparties/normalized/enrich
   - Нужно проверить наличие фронтенд API route

2. **Дубликаты контрагентов**
   - GET /api/counterparties/normalized/duplicates
   - POST /api/counterparties/normalized/duplicates/{groupId}/merge
   - Нужно проверить наличие фронтенд API routes

3. **Экспорт контрагентов**
   - POST /api/counterparties/normalized/export
   - Нужно проверить наличие фронтенд API route

## Переменные окружения

### Фронтенд
- `BACKEND_URL` - URL бэкенда (по умолчанию: `http://localhost:9999`)

### Бэкенд
- Порт: `9999` (по умолчанию)
- База данных: `service.db`

## Тестирование

Для тестирования цепочки вызовов используйте скрипт:
```powershell
.\test_api_chain.ps1
```

Скрипт проверяет:
1. Доступность бэкенда
2. Доступность фронтенда
3. Получение клиентов
4. Получение проектов
5. Получение контрагентов
6. Получение статистики
7. Работу фронтенд API (прокси)

