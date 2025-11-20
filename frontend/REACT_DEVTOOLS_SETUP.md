# Настройка React DevTools для Next.js

## ⚠️ ВАЖНО: Режим запуска приложения

**React DevTools работает лучше всего в development режиме!**

Если вы видите сообщение:
> "This page is using the production build of React"

Это означает, что приложение запущено в **production режиме** (`npm start`).

### ✅ Правильный запуск для разработки:

```bash
cd frontend
npm run dev
```

**НЕ используйте:**
- ❌ `npm start` (production режим)
- ❌ `npm run build && npm start` (production режим)

**Используйте:**
- ✅ `npm run dev` (development режим)
- ✅ `npm run dev:with-backend` (development режим + бэкенд)

### Разница между режимами:

| Режим | Команда | DevTools | Hot Reload | Предупреждения React |
|-------|---------|----------|-----------|---------------------|
| **Development** | `npm run dev` | ✅ Полная функциональность | ✅ Да | ✅ Детальные |
| **Production** | `npm start` | ⚠️ Ограниченная | ❌ Нет | ❌ Минимальные |

## Проблема: Вкладка React не отображается в DevTools

### Быстрые решения

#### 1. Проверка версии Chrome
- Убедитесь, что используете **Chrome v102 или новее**
- Проверьте версию: `chrome://version/`
- Если версия ниже 102, обновите Chrome

#### 2. Проверка Service Worker расширения
1. Откройте `chrome://extensions`
2. Найдите "React Developer Tools"
3. Если видите "service worker (inactive)":
   - Отключите расширение
   - Включите его снова
   - Перезагрузите страницу приложения
   - Закройте и снова откройте DevTools

#### 3. Проверка доступа к файлам (если используете file://)
- Откройте `chrome://extensions`
- Найдите React Developer Tools
- Включите "Allow access to file URLs"

#### 4. Очистка кэша и перезапуск
```bash
# Очистить кэш Next.js
rm -rf frontend/.next

# Перезапустить dev сервер
cd frontend
npm run dev
```

### Для Next.js 16 + React 19

#### Проверка конфигурации
Убедитесь, что в `next.config.ts` нет настроек, которые могут блокировать DevTools:

```typescript
const nextConfig: NextConfig = {
  // Убедитесь, что React DevTools может работать
  reactStrictMode: true, // Рекомендуется для разработки
};
```

#### Проверка в консоли браузера
Откройте консоль браузера (F12) и проверьте:
```javascript
// Проверка наличия React DevTools hook
console.log(window.__REACT_DEVTOOLS_GLOBAL_HOOK__);

// Должно вывести объект, а не undefined
```

### Отладка

#### 1. Проверка, что React загружен
В консоли браузера:
```javascript
// Проверка версии React
console.log(React.version);

// Проверка наличия React
console.log(typeof React !== 'undefined' ? 'React найден' : 'React не найден');
```

#### 2. Проверка режима разработки
Убедитесь, что приложение запущено в режиме разработки:
```bash
npm run dev  # Не npm run build && npm start
```

#### 3. Проверка расширения
- Убедитесь, что расширение React Developer Tools установлено и включено
- Попробуйте переустановить расширение
- Проверьте, что нет конфликтов с другими расширениями

### Альтернативное решение: Standalone DevTools

Если расширение Chrome не работает, используйте standalone версию:

```bash
# Установка React DevTools standalone
npm install -g react-devtools

# Запуск
react-devtools
```

Затем в вашем приложении добавьте в начало `app/layout.tsx`:
```typescript
if (typeof window !== 'undefined' && process.env.NODE_ENV === 'development') {
  require('react-devtools-core').connectToDevTools();
}
```

### Частые проблемы

#### Проблема: Компоненты не отображаются
- **Решение**: Обновите Chrome до v102+
- **Решение**: Перезапустите service worker расширения
- **Решение**: Очистите кэш браузера

#### Проблема: DevTools показывает "No components"
- **Решение**: Убедитесь, что страница использует React (не только SSR)
- **Решение**: Проверьте, что компоненты рендерятся на клиенте (используйте 'use client')

#### Проблема: DevTools не подключается
- **Решение**: Проверьте консоль браузера на наличие ошибок
- **Решение**: Убедитесь, что нет блокировщиков расширений
- **Решение**: Попробуйте режим инкогнито

### Проверка работоспособности

После применения решений:
1. Откройте приложение в Chrome
2. Откройте DevTools (F12)
3. Должна появиться вкладка "⚛️ Components"
4. Вкладка должна показывать дерево компонентов

### Дополнительная информация

- [Официальная документация React DevTools](https://react.dev/learn/react-developer-tools)
- [Troubleshooting React DevTools](https://github.com/facebook/react/tree/main/packages/react-devtools#troubleshooting)

