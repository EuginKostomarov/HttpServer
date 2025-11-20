#!/bin/bash
# Тест функциональности фильтрации результатов нормализации по базе данных

echo "=== Тест фильтрации результатов нормализации ==="
echo ""

# Тест 1: Проверка создания сессии нормализации
echo "Тест 1: Создание сессии нормализации"
curl -X POST "http://localhost:8080/api/clients/1/projects/1/normalization/start" \
  -H "Content-Type: application/json" \
  -d '{"database_path": "test.db", "all_active": false}' \
  --max-time 7 \
  -w "\nHTTP Status: %{http_code}\n" \
  -s | head -20

echo ""
echo "Ожидание 2 секунды..."
sleep 2

# Тест 2: Проверка получения групп нормализации для базы данных
echo ""
echo "Тест 2: Получение групп нормализации для базы данных"
echo "Сначала нужно получить db_id для тестовой базы..."
curl -X GET "http://localhost:8080/api/clients/1/projects/1/databases?active_only=false" \
  --max-time 7 \
  -w "\nHTTP Status: %{http_code}\n" \
  -s | head -30

echo ""
echo "=== Тесты завершены ==="

