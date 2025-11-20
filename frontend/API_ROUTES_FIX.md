# 🔧 Исправление API Routes для /api/databases/pending и /api/databases/scan

## ✅ Проблема

Получались ошибки 404 для:
- `GET /api/databases/pending`
- `POST /api/databases/scan`

## ✅ Решение

### 1. Исправлен порт в `/api/databases/pending/route.ts`

**Было:**
```typescript
const API_BASE = process.env.BACKEND_URL || process.env.NEXT_PUBLIC_BACKEND_URL || 'http://localhost:8080'
```

**Стало:**
```typescript
const API_BASE = process.env.BACKEND_URL || process.env.NEXT_PUBLIC_BACKEND_URL || 'http://localhost:9999'
```

### 2. Улучшена обработка переменных окружения в `/api/databases/scan/route.ts`

**Было:**
```typescript
const API_BASE = process.env.BACKEND_URL || 'http://localhost:9999'
```

**Стало:**
```typescript
const API_BASE = process.env.BACKEND_URL || process.env.NEXT_PUBLIC_BACKEND_URL || 'http://localhost:9999'
```

## 🔍 Проверка файлов

Оба файла существуют и правильно экспортируют функции:

- ✅ `frontend/app/api/databases/pending/route.ts` - экспортирует `GET`
- ✅ `frontend/app/api/databases/scan/route.ts` - экспортирует `POST`

## 🚀 Если проблема сохраняется

### 1. Очистить кэш Next.js:

```bash
cd frontend
rm -rf .next
npm run dev
```

### 2. Проверить, что dev сервер перезапущен:

- Остановите текущий dev сервер (Ctrl+C)
- Запустите заново: `npm run dev`

### 3. Проверить переменные окружения:

Убедитесь, что в `.env.local` или `.env` правильно указан `BACKEND_URL`:

```env
BACKEND_URL=http://localhost:9999
```

### 4. Проверить структуру файлов:

```bash
# Должны существовать:
frontend/app/api/databases/pending/route.ts
frontend/app/api/databases/scan/route.ts
```

## 📝 Структура API Routes

```
frontend/app/api/databases/
├── pending/
│   ├── route.ts          # GET /api/databases/pending
│   ├── [id]/
│   │   ├── route.ts      # GET/POST /api/databases/pending/[id]
│   │   └── [action]/
│   │       └── route.ts  # POST /api/databases/pending/[id]/[action]
└── scan/
    └── route.ts          # POST /api/databases/scan
```

## ✅ Итог

- ✅ Исправлены порты в обоих файлах
- ✅ Улучшена обработка переменных окружения
- ✅ Файлы правильно структурированы
- ✅ Экспорты функций корректны

Если проблема сохраняется после перезапуска dev сервера, проверьте логи Next.js на наличие ошибок компиляции.

