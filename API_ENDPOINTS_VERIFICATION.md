# Проверка цепочки вызовов API - Результаты

## ✅ Проверенные цепочки

### 1. Контрагенты

#### GET /api/counterparties/normalized
**Цепочка:**
1. Фронтенд: `fetch('/api/counterparties/normalized?client_id=${clientId}&project_id=${projectId}')`
2. Next.js API: `frontend/app/api/counterparties/normalized/route.ts`
3. Бэкенд: `server/server.go::handleNormalizedCounterparties`
4. БД: `database/service_db.go::GetNormalizedCounterpartiesByClient`
**Статус:** ✅ Работает корректно

#### GET /api/counterparties/normalized/stats
**Цепочка:**
1. Фронтенд: `fetch('/api/counterparties/normalized/stats?project_id=${projectId}')`
2. Next.js API: `frontend/app/api/counterparties/normalized/stats/route.ts`
3. Бэкенд: `server/server.go::handleNormalizedCounterpartyStats`
4. БД: `database/service_db.go::GetNormalizedCounterpartyStats`
**Статус:** ✅ Работает корректно

#### GET /api/counterparties/normalized/{id}
**Цепочка:**
1. Фронтенд: `fetch('/api/counterparties/normalized/${id}')`
2. Next.js API: `frontend/app/api/counterparties/normalized/[id]/route.ts`
3. Бэкенд: `server/server.go::handleGetNormalizedCounterparty`
4. БД: `database/service_db.go::GetNormalizedCounterparty`
**Статус:** ✅ Работает корректно

#### PUT /api/counterparties/normalized/{id}
**Цепочка:**
1. Фронтенд: `fetch('/api/counterparties/normalized/${id}', { method: 'PUT', body: ... })`
2. Next.js API: `frontend/app/api/counterparties/normalized/[id]/route.ts`
3. Бэкенд: `server/server.go::handleUpdateNormalizedCounterparty`
4. БД: `database/service_db.go::UpdateNormalizedCounterparty`
**Статус:** ✅ Работает корректно

### 2. Нормализация

#### POST /api/clients/{id}/projects/{projectId}/normalization/start
**Цепочка:**
1. Фронтенд: `fetch('/api/clients/${clientId}/projects/${projectId}/normalization/start', { method: 'POST', body: ... })`
2. Next.js API: `frontend/app/api/clients/[clientId]/projects/[projectId]/normalization/start/route.ts`
3. Бэкенд: `server/server.go::handleStartClientNormalization`
4. Нормализатор: `normalization.NewClientNormalizerWithConfig`
**Статус:** ✅ Исправлено - добавлена передача body

#### GET /api/normalization/stats
**Цепочка:**
1. Фронтенд: `fetch('/api/normalization/stats')`
2. Next.js API: `frontend/app/api/normalization/stats/route.ts`
3. Бэкенд: `server/server.go::handleNormalizationStats`
4. БД: `database/service_db.go::GetNormalizationStats`
**Статус:** ✅ Работает корректно

#### POST /api/normalization/start (старый endpoint)
**Цепочка:**
1. Фронтенд: `fetch('/api/normalization/start', { method: 'POST', body: ... })`
2. Next.js API: `frontend/app/api/normalization/start/route.ts`
3. Бэкенд: `server/server.go::handleNormalizeStart`
**Статус:** ✅ Работает (для обратной совместимости)

### 3. Клиенты и проекты

#### GET /api/clients
**Цепочка:**
1. Фронтенд: `fetch('/api/clients')`
2. Next.js API: `frontend/app/api/clients/route.ts`
3. Бэкенд: `server/server.go::handleClients`
4. БД: `database/service_db.go::GetClients`
**Статус:** ✅ Работает корректно

#### GET /api/clients/{id}/projects
**Цепочка:**
1. Фронтенд: `fetch('/api/clients/${clientId}/projects')`
2. Next.js API: `frontend/app/api/clients/[clientId]/projects/route.ts`
3. Бэкенд: `server/server.go::handleClientRoutes`
4. БД: `database/service_db.go::GetClientProjects`
**Статус:** ✅ Работает корректно

## ⚠️ Обнаруженные проблемы

### 1. Несоответствие endpoints нормализации
- **Проблема:** Есть два endpoint для запуска нормализации:
  - `/api/normalize/start` (старый)
  - `/api/normalization/start` (новый)
  - `/api/clients/{id}/projects/{projectId}/normalization/start` (для проектов)
- **Решение:** Все три endpoint работают, но рекомендуется использовать последний для проектов

### 2. Отсутствие передачи body в normalization/start
- **Проблема:** В `frontend/app/api/clients/[clientId]/projects/[projectId]/normalization/start/route.ts` не передавался body запроса
- **Решение:** ✅ Исправлено - добавлена передача body

## 📋 Endpoints требующие проверки

### Контрагенты (дополнительные)
- `POST /api/counterparties/normalized/enrich` - ручное обогащение
- `GET /api/counterparties/normalized/duplicates` - получение дубликатов
- `POST /api/counterparties/normalized/duplicates/{groupId}/merge` - объединение дубликатов
- `POST /api/counterparties/normalized/export` - экспорт контрагентов

**Статус:** Endpoints существуют на бэкенде, но нет фронтенд API routes

## 🔧 Рекомендации

1. **Создать фронтенд API routes для дополнительных endpoints контрагентов:**
   - `/api/counterparties/normalized/enrich`
   - `/api/counterparties/normalized/duplicates`
   - `/api/counterparties/normalized/duplicates/[groupId]/merge`
   - `/api/counterparties/normalized/export`

2. **Унифицировать endpoints нормализации:**
   - Использовать `/api/clients/{id}/projects/{projectId}/normalization/start` для проектов
   - Оставить `/api/normalize/start` для обратной совместимости

3. **Добавить обработку ошибок:**
   - Все фронтенд API routes должны корректно обрабатывать ошибки бэкенда
   - Добавить логирование для отладки

4. **Проверить переменные окружения:**
   - Убедиться, что `BACKEND_URL` правильно настроен
   - Проверить доступность бэкенда на указанном URL

## 🧪 Тестирование

Для тестирования цепочки вызовов используйте:
```powershell
.\test_api_chain.ps1
```

Скрипт проверяет:
- Доступность бэкенда и фронтенда
- Получение клиентов
- Получение проектов
- Получение контрагентов
- Получение статистики
- Работу фронтенд API (прокси)

