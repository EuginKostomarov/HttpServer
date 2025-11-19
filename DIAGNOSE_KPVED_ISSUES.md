# Диагностика проблем с классификацией КПВЭД

## Проблема: Все группы завершились с ошибкой

### Шаги диагностики

1. **Проверка API ключа**
   - Убедитесь, что API ключ Arliai настроен в конфигурации воркеров
   - Проверьте переменную окружения `ARLIAI_API_KEY`
   - Endpoint для проверки: `GET /api/workers/config`

2. **Проверка данных КПВЭД классификатора**
   - Убедитесь, что таблица `kpved_classifier` существует в service DB
   - Проверьте, что таблица не пуста
   - Endpoint для проверки: `GET /api/kpved/stats`
   - Endpoint для загрузки: `POST /api/kpved/load-from-file`

3. **Проверка логов сервера**
   - Ищите сообщения с префиксом `[KPVED]`
   - Проверьте первые ошибки в логах
   - Ищите сообщения типа:
     - `[KPVED] ERROR creating hierarchical classifier`
     - `[KPVED Worker N] ERROR classifying`
     - `[KPVED] ERROR: All N groups failed classification!`

4. **Проверка сети и AI API**
   - Убедитесь, что сервер может подключиться к Arliai API
   - Проверьте rate limits
   - Endpoint для проверки: `GET /api/workers/arliai/status`

### Типичные ошибки и решения

#### Ошибка: "AI API key not configured"
**Решение:** Настройте API ключ через `/api/workers/config/update` или переменную окружения

#### Ошибка: "kpved_classifier table is empty"
**Решение:** Загрузите классификатор через `/api/kpved/load-from-file` с файлом КПВЭД.txt

#### Ошибка: "no candidates found"
**Решение:** Проверьте структуру данных в таблице kpved_classifier. Должны быть секции (A-U), классы, подклассы, группы.

#### Ошибка: "ai call failed"
**Решение:** 
- Проверьте API ключ
- Проверьте доступность Arliai API
- Проверьте rate limits (максимум 2 параллельных запроса)

#### Ошибка: "rate limit" или "429"
**Решение:** Система автоматически делает паузы, но убедитесь, что используется не более 2 воркеров

### Команды для диагностики

```bash
# Проверка конфигурации воркеров
curl http://localhost:9999/api/workers/config

# Проверка статистики КПВЭД
curl http://localhost:9999/api/kpved/stats

# Проверка статуса Arliai
curl http://localhost:9999/api/workers/arliai/status

# Тестовая классификация одного товара
curl -X POST http://localhost:9999/api/kpved/classify-hierarchical \
  -H "Content-Type: application/json" \
  -d '{"normalized_name": "болт м10", "category": "инструмент"}'
```

### Проверка логов

В логах сервера ищите:
- `[KPVED]` - общие сообщения
- `[KPVED Worker N]` - сообщения от воркеров
- `[HierarchicalClassifier]` - сообщения от классификатора
- `[Keyword]` - классификация по ключевым словам
- `[BaseWordCache]` - попадания в кэш корневых слов
- `[Cache]` - попадания в полный кэш

### Улучшения в коде

1. ✅ Добавлено детальное логирование ошибок
2. ✅ Добавлены примеры ошибок в ответ API
3. ✅ Улучшено отображение ошибок на фронтенде
4. ✅ Добавлена защита от nil для keywordClassifier

### Следующие шаги

1. Проверьте логи сервера при запуске классификации
2. Проверьте, что API ключ настроен
3. Проверьте, что данные КПВЭД загружены
4. Проверьте примеры ошибок в ответе API

