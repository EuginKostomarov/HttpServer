#!/bin/bash

# Тестовый скрипт для проверки API отчёта качества
# Использование: bash test_quality_report_api.sh [database_path]

API_URL="http://localhost:8080/api/quality/report"
DATABASE_PATH="${1:-E:/HttpServer/data/normalized.db}"

echo "========================================="
echo "Тестирование API отчёта качества"
echo "========================================="
echo ""
echo "URL: $API_URL"
echo "Database: $DATABASE_PATH"
echo ""

# Тест 1: Проверка без параметра database (должен использовать default)
echo "Тест 1: Запрос без параметра database"
response1=$(curl -s -w "\nHTTP_CODE:%{http_code}" "$API_URL" --max-time 7)
http_code1=$(echo "$response1" | grep "HTTP_CODE" | cut -d: -f2)
body1=$(echo "$response1" | sed '/HTTP_CODE/d')

echo "HTTP Code: $http_code1"
if [ "$http_code1" == "200" ]; then
    echo "✓ Успешно"
    echo "Response preview:"
    echo "$body1" | head -c 200
    echo "..."
else
    echo "✗ Ошибка: $body1"
fi
echo ""

# Тест 2: Проверка с параметром database
echo "Тест 2: Запрос с параметром database"
response2=$(curl -s -w "\nHTTP_CODE:%{http_code}" "$API_URL?database=$(echo "$DATABASE_PATH" | sed 's/#/%23/g')" --max-time 7)
http_code2=$(echo "$response2" | grep "HTTP_CODE" | cut -d: -f2)
body2=$(echo "$response2" | sed '/HTTP_CODE/d')

echo "HTTP Code: $http_code2"
if [ "$http_code2" == "200" ]; then
    echo "✓ Успешно"
    echo "Response preview:"
    echo "$body2" | head -c 200
    echo "..."
    
    # Проверяем структуру ответа
    if echo "$body2" | grep -q "quality_score"; then
        echo "✓ Поле quality_score найдено"
    fi
    if echo "$body2" | grep -q "summary"; then
        echo "✓ Поле summary найдено"
    fi
    if echo "$body2" | grep -q "distribution"; then
        echo "✓ Поле distribution найдено"
    fi
    if echo "$body2" | grep -q "detailed"; then
        echo "✓ Поле detailed найдено"
    fi
    if echo "$body2" | grep -q "recommendations"; then
        echo "✓ Поле recommendations найдено"
    fi
else
    echo "✗ Ошибка: $body2"
fi
echo ""

# Тест 3: Проверка с неверным методом (должен вернуть 405)
echo "Тест 3: POST запрос (должен вернуть 405)"
response3=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST "$API_URL" --max-time 7)
http_code3=$(echo "$response3" | grep "HTTP_CODE" | cut -d: -f2)

echo "HTTP Code: $http_code3"
if [ "$http_code3" == "405" ]; then
    echo "✓ Корректно отклонён POST запрос"
else
    echo "✗ Неожиданный код: $http_code3"
fi
echo ""

echo "========================================="
echo "Тестирование завершено"
echo "========================================="

