# Скрипт для проверки цепочки вызовов API
$ErrorActionPreference = "Stop"

$BACKEND_URL = "http://localhost:9999"
$FRONTEND_URL = "http://localhost:3000"

Write-Host "=== Тестирование цепочки вызовов API ===" -ForegroundColor Cyan

# Проверка доступности бэкенда
Write-Host "`n1. Проверка доступности бэкенда..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$BACKEND_URL/api/v1/health" -Method Get -TimeoutSec 5
    Write-Host "✓ Бэкенд доступен" -ForegroundColor Green
} catch {
    Write-Host "✗ Бэкенд недоступен: $_" -ForegroundColor Red
    exit 1
}

# Проверка доступности фронтенда
Write-Host "`n2. Проверка доступности фронтенда..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "$FRONTEND_URL" -Method Get -TimeoutSec 5 -UseBasicParsing
    Write-Host "✓ Фронтенд доступен" -ForegroundColor Green
} catch {
    Write-Host "✗ Фронтенд недоступен: $_" -ForegroundColor Red
    exit 1
}

# Тест получения клиентов
Write-Host "`n3. Тест получения клиентов..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$BACKEND_URL/api/clients" -Method Get -TimeoutSec 5
    Write-Host "✓ Получено клиентов: $($response.clients.Count)" -ForegroundColor Green
    if ($response.clients.Count -gt 0) {
        $clientId = $response.clients[0].id
        Write-Host "  Используем клиента ID: $clientId" -ForegroundColor Gray
        
        # Тест получения проектов клиента
        Write-Host "`n4. Тест получения проектов клиента..." -ForegroundColor Yellow
        try {
            $projectsResponse = Invoke-RestMethod -Uri "$BACKEND_URL/api/clients/$clientId/projects" -Method Get -TimeoutSec 5
            Write-Host "✓ Получено проектов: $($projectsResponse.projects.Count)" -ForegroundColor Green
            if ($projectsResponse.projects.Count -gt 0) {
                $projectId = $projectsResponse.projects[0].id
                Write-Host "  Используем проект ID: $projectId" -ForegroundColor Gray
                
                # Тест получения контрагентов
                Write-Host "`n5. Тест получения контрагентов..." -ForegroundColor Yellow
                try {
                    $counterpartiesResponse = Invoke-RestMethod -Uri "$BACKEND_URL/api/counterparties/normalized?client_id=$clientId&project_id=$projectId&limit=5" -Method Get -TimeoutSec 5
                    Write-Host "✓ Получено контрагентов: $($counterpartiesResponse.counterparties.Count)" -ForegroundColor Green
                    Write-Host "  Всего: $($counterpartiesResponse.total)" -ForegroundColor Gray
                    
                    # Тест получения статистики контрагентов
                    Write-Host "`n6. Тест получения статистики контрагентов..." -ForegroundColor Yellow
                    try {
                        $statsResponse = Invoke-RestMethod -Uri "$BACKEND_URL/api/counterparties/normalized/stats?project_id=$projectId" -Method Get -TimeoutSec 5
                        Write-Host "✓ Статистика получена" -ForegroundColor Green
                        Write-Host "  Всего: $($statsResponse.total_count)" -ForegroundColor Gray
                        Write-Host "  Производителей: $($statsResponse.manufacturers_count)" -ForegroundColor Gray
                    } catch {
                        Write-Host "✗ Ошибка получения статистики: $_" -ForegroundColor Red
                    }
                } catch {
                    Write-Host "✗ Ошибка получения контрагентов: $_" -ForegroundColor Red
                }
            }
        } catch {
            Write-Host "✗ Ошибка получения проектов: $_" -ForegroundColor Red
        }
    }
} catch {
    Write-Host "✗ Ошибка получения клиентов: $_" -ForegroundColor Red
}

# Тест через фронтенд API
Write-Host "`n7. Тест через фронтенд API (прокси)..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$FRONTEND_URL/api/clients" -Method Get -TimeoutSec 5
    Write-Host "✓ Фронтенд API работает" -ForegroundColor Green
    Write-Host "  Получено клиентов: $($response.clients.Count)" -ForegroundColor Gray
} catch {
    Write-Host "✗ Ошибка фронтенд API: $_" -ForegroundColor Red
}

Write-Host "`n=== Тестирование завершено ===" -ForegroundColor Cyan

