#!/bin/bash
# Тест управления сессиями нормализации

echo "=== Тест управления сессиями нормализации ==="
echo ""

CLIENT_ID=1
PROJECT_ID=1

# Тест 1: Получение списка сессий
echo "Тест 1: Получение списка сессий"
curl -X GET "http://localhost:8080/api/clients/${CLIENT_ID}/projects/${PROJECT_ID}/normalization/sessions" \
  --max-time 7 \
  -w "\nHTTP Status: %{http_code}\n" \
  -s | jq '.' 2>/dev/null || echo "Response received"

echo ""
echo "---"

# Тест 2: Запуск нормализации (создание сессии)
echo ""
echo "Тест 2: Запуск нормализации для создания сессии"
curl -X POST "http://localhost:8080/api/clients/${CLIENT_ID}/projects/${PROJECT_ID}/normalization/start" \
  -H "Content-Type: application/json" \
  -d '{"database_path": "test.db", "all_active": false}' \
  --max-time 7 \
  -w "\nHTTP Status: %{http_code}\n" \
  -s | jq '.' 2>/dev/null || echo "Response received"

echo ""
echo "Ожидание 3 секунды для создания сессии..."
sleep 3

# Тест 3: Получение списка сессий после запуска
echo ""
echo "Тест 3: Получение списка сессий после запуска"
SESSIONS_RESPONSE=$(curl -X GET "http://localhost:8080/api/clients/${CLIENT_ID}/projects/${PROJECT_ID}/normalization/sessions" \
  --max-time 7 \
  -s)

echo "$SESSIONS_RESPONSE" | jq '.' 2>/dev/null || echo "$SESSIONS_RESPONSE"

# Извлекаем session_id из ответа (если есть)
SESSION_ID=$(echo "$SESSIONS_RESPONSE" | jq -r '.sessions[0].id' 2>/dev/null)

if [ "$SESSION_ID" != "null" ] && [ -n "$SESSION_ID" ]; then
  echo ""
  echo "Найдена сессия ID: $SESSION_ID"
  
  # Тест 4: Обновление приоритета сессии
  echo ""
  echo "Тест 4: Обновление приоритета сессии $SESSION_ID на 10"
  curl -X POST "http://localhost:8080/api/clients/${CLIENT_ID}/projects/${PROJECT_ID}/normalization/sessions" \
    -H "Content-Type: application/json" \
    -d "{\"session_id\": $SESSION_ID, \"priority\": 10}" \
    --max-time 7 \
    -w "\nHTTP Status: %{http_code}\n" \
    -s | jq '.' 2>/dev/null || echo "Response received"
  
  # Тест 5: Остановка сессии
  echo ""
  echo "Тест 5: Остановка сессии $SESSION_ID"
  curl -X POST "http://localhost:8080/api/clients/${CLIENT_ID}/projects/${PROJECT_ID}/normalization/sessions/${SESSION_ID}" \
    --max-time 7 \
    -w "\nHTTP Status: %{http_code}\n" \
    -s | jq '.' 2>/dev/null || echo "Response received"
  
  # Тест 6: Проверка статуса после остановки
  echo ""
  echo "Тест 6: Проверка статуса сессии после остановки"
  sleep 1
  curl -X GET "http://localhost:8080/api/clients/${CLIENT_ID}/projects/${PROJECT_ID}/normalization/sessions" \
    --max-time 7 \
    -s | jq '.sessions[] | select(.id == '$SESSION_ID')' 2>/dev/null || echo "Response received"
else
  echo ""
  echo "Сессия не найдена, пропускаем тесты 4-6"
fi

echo ""
echo "=== Тесты завершены ==="

