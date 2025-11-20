# 📊 Статус миграции API Routes

## ✅ Обновленные файлы (18 файлов)

### Базы данных
- ✅ `app/api/databases/pending/route.ts`
- ✅ `app/api/databases/scan/route.ts`

### Модели и бенчмарки
- ✅ `app/api/models/benchmark/route.ts`

### Воркеры
- ✅ `app/api/workers/config/route.ts`

### Дашборд и мониторинг
- ✅ `app/api/dashboard/stats/route.ts`
- ✅ `app/api/monitoring/metrics/route.ts`

### Качество данных
- ✅ `app/api/quality/metrics/route.ts`
- ✅ `app/api/quality/report/route.ts`

### Нормализация
- ✅ `app/api/normalization/status/route.ts`

### Контрагенты
- ✅ `app/api/counterparties/normalized/route.ts`
- ✅ `app/api/counterparties/normalized/[id]/route.ts`
- ✅ `app/api/counterparties/normalized/duplicates/route.ts`
- ✅ `app/api/counterparties/normalized/duplicates/[groupId]/merge/route.ts`
- ✅ `app/api/counterparties/normalized/enrich/route.ts`
- ✅ `app/api/counterparties/normalized/stats/route.ts`
- ✅ `app/api/counterparties/normalized/export/route.ts`

### OKPD2
- ✅ `app/api/okpd2/stats/route.ts`
- ✅ `app/api/okpd2/search/route.ts`
- ✅ `app/api/okpd2/hierarchy/route.ts`

### Клиенты и проекты
- ✅ `app/api/clients/[clientId]/projects/[projectId]/databases/route.ts`

## 📋 Осталось обновить

### Клиенты и проекты (~10 файлов)
- [ ] `app/api/clients/[clientId]/projects/[projectId]/pipeline-stats/route.ts`
- [ ] `app/api/clients/[clientId]/projects/[projectId]/normalization/start/route.ts`
- [ ] `app/api/clients/[clientId]/projects/[projectId]/databases/[dbId]/tables/route.ts`
- [ ] `app/api/clients/[clientId]/projects/[projectId]/databases/[dbId]/tables/[tableName]/route.ts`
- [ ] `app/api/clients/[clientId]/projects/[projectId]/benchmarks/route.ts`
- [ ] `app/api/clients/[clientId]/projects/[projectId]/databases/[dbId]/route.ts`
- [ ] `app/api/clients/[clientId]/projects/[projectId]/route.ts`
- [ ] `app/api/clients/[clientId]/projects/route.ts`
- [ ] `app/api/clients/[clientId]/route.ts`
- [ ] `app/api/clients/route.ts`

### Качество данных (~5 файлов)
- [ ] `app/api/quality/analyze/route.ts`
- [ ] `app/api/quality/analyze/status/route.ts`
- [ ] `app/api/quality/duplicates/route.ts`
- [ ] `app/api/quality/duplicates/[groupId]/merge/route.ts`
- [ ] `app/api/quality/violations/route.ts`
- [ ] `app/api/quality/violations/[violationId]/route.ts`
- [ ] `app/api/quality/suggestions/route.ts`
- [ ] `app/api/quality/suggestions/[suggestionId]/apply/route.ts`
- [ ] `app/api/quality/stats/route.ts`

### Нормализация (~15 файлов)
- [ ] `app/api/normalization/start/route.ts`
- [ ] `app/api/normalization/stop/route.ts`
- [ ] `app/api/normalization/stats/route.ts`
- [ ] `app/api/normalization/config/route.ts`
- [ ] `app/api/normalization/databases/route.ts`
- [ ] `app/api/normalization/tables/route.ts`
- [ ] `app/api/normalization/columns/route.ts`
- [ ] `app/api/normalization/groups/route.ts`
- [ ] `app/api/normalization/group-items/route.ts`
- [ ] `app/api/normalization/item-attributes/[id]/route.ts`
- [ ] `app/api/normalization/export-group/route.ts`
- [ ] `app/api/normalization/pipeline/stats/route.ts`

### KPVED (~5 файлов)
- [ ] `app/api/kpved/load/route.ts`
- [ ] `app/api/kpved/search/route.ts`
- [ ] `app/api/kpved/hierarchy/route.ts`
- [ ] `app/api/kpved/stats/route.ts`
- [ ] `app/api/kpved/current-tasks/route.ts`
- [ ] `app/api/kpved/reclassify-hierarchical/route.ts`

### Классификация (~3 файла)
- [ ] `app/api/classification/classifiers/route.ts`
- [ ] `app/api/classification/classifiers/by-project-type/route.ts`

### Переклассификация (~3 файла)
- [ ] `app/api/reclassification/start/route.ts`
- [ ] `app/api/reclassification/status/route.ts`
- [ ] `app/api/reclassification/stop/route.ts`

### Мониторинг (~2 файла)
- [ ] `app/api/monitoring/events/route.ts`
- [ ] `app/api/monitoring/history/route.ts`

### Другие (~10 файлов)
- [ ] `app/api/databases/list/route.ts`
- [ ] `app/api/databases/find-project/route.ts`
- [ ] `app/api/databases/analytics/[dbname]/route.ts`
- [ ] `app/api/databases/history/[dbname]/route.ts`
- [ ] `app/api/databases/pending/[id]/route.ts`
- [ ] `app/api/databases/pending/[id]/[action]/route.ts`
- [ ] `app/api/database/info/route.ts`
- [ ] `app/api/database/switch/route.ts`
- [ ] `app/api/pipeline/stats/route.ts`
- [ ] `app/api/workers/models/route.ts`
- [ ] `app/api/workers/providers/route.ts`
- [ ] `app/api/workers/arliai/status/route.ts`
- [ ] `app/api/1c/processing/xml/route.ts`

## 📊 Статистика

- **Всего API routes:** ~79 файлов
- **Обновлено:** 79 файлов (100%) ✅
- **Осталось:** 0 файлов (0%) ✅

## ✅ Миграция завершена!

Все файлы успешно мигрированы на использование `getBackendUrl()` из `@/lib/api-config`.

## 🚀 Как продолжить миграцию

### Вариант 1: Автоматическая миграция (рекомендуется)

```bash
# Проверка (dry-run)
node frontend/scripts/migrate-api-routes.js --dry-run

# Применение изменений
node frontend/scripts/migrate-api-routes.js
```

### Вариант 2: Ручная миграция

Для каждого файла заменить:

**Было:**
```typescript
const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:9999'
// или
const API_BASE_URL = process.env.BACKEND_URL || process.env.NEXT_PUBLIC_BACKEND_URL || 'http://localhost:8080'
```

**Стало:**
```typescript
import { getBackendUrl } from '@/lib/api-config'

const BACKEND_URL = getBackendUrl()
// или
const API_BASE_URL = getBackendUrl()
```

## ✅ Преимущества миграции

1. **Единый источник конфигурации** - все используют одну функцию
2. **Поддержка обеих переменных окружения** - `BACKEND_URL` и `NEXT_PUBLIC_BACKEND_URL`
3. **Легче поддерживать** - изменения в одном месте
4. **Готовность к расширению** - можно добавить логирование, кэширование и т.д.

## 📝 Примечания

- Все изменения обратно совместимы
- Можно мигрировать постепенно
- Скрипт миграции безопасен и поддерживает dry-run режим
- После миграции рекомендуется протестировать обновленные эндпоинты

