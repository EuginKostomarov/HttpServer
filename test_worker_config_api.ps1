# Тестирование Worker Config API
Write-Host "=== Тестирование Worker Config API ===" -ForegroundColor Cyan

$baseUrl = "http://localhost:9999"

# Тест 1: GET /api/workers/config
Write-Host "`n1. Тест GET /api/workers/config" -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/api/workers/config" -Method GET -TimeoutSec 5
    Write-Host "✅ Успешно получена конфигурация" -ForegroundColor Green
    Write-Host "   Default provider: $($response.default_provider)"
    Write-Host "   Arliai has_api_key: $($response.providers.arliai.has_api_key)"
    if ($response.providers.arliai.api_key) {
        Write-Host "   ❌ ОШИБКА: API ключ присутствует в ответе!" -ForegroundColor Red
    } else {
        Write-Host "   ✅ API ключ скрыт" -ForegroundColor Green
    }
} catch {
    Write-Host "❌ Ошибка: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Тест 2: POST - изменение priority
Write-Host "`n2. Тест изменения priority (не должно стирать API ключ)" -ForegroundColor Yellow
$currentPriority = $response.providers.arliai.priority
$newPriority = if ($currentPriority -eq 1) { 2 } else { 1 }
$body = @{
    action = "update_provider"
    data = @{
        name = "arliai"
        priority = $newPriority
    }
} | ConvertTo-Json -Depth 10

try {
    Write-Host "   Отправка запроса: priority $currentPriority → $newPriority"
    $updateResponse = Invoke-RestMethod -Uri "$baseUrl/api/workers/config/update" -Method POST -Body $body -ContentType "application/json" -TimeoutSec 10
    Write-Host "   ✅ Ответ: $($updateResponse.message)" -ForegroundColor Green
    
    Start-Sleep -Seconds 1
    
    # Проверяем что priority изменился, а API ключ остался
    $checkResponse = Invoke-RestMethod -Uri "$baseUrl/api/workers/config" -Method GET -TimeoutSec 5
    if ($checkResponse.providers.arliai.priority -eq $newPriority) {
        Write-Host "   ✅ Priority изменен: $newPriority" -ForegroundColor Green
    } else {
        Write-Host "   ❌ Priority не изменился!" -ForegroundColor Red
    }
    if ($checkResponse.providers.arliai.has_api_key) {
        Write-Host "   ✅ API ключ сохранен (has_api_key=true)" -ForegroundColor Green
    } else {
        Write-Host "   ❌ API ключ потерян!" -ForegroundColor Red
    }
} catch {
    Write-Host "   ❌ Ошибка: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $responseBody = $reader.ReadToEnd()
        Write-Host "   Response: $responseBody" -ForegroundColor Red
    }
}

# Тест 3: POST - изменение max_workers
Write-Host "`n3. Тест изменения max_workers (не должно стирать API ключ)" -ForegroundColor Yellow
$currentWorkers = $response.providers.arliai.max_workers
$newWorkers = if ($currentWorkers -eq 2) { 5 } else { 2 }
$body = @{
    action = "update_provider"
    data = @{
        name = "arliai"
        max_workers = $newWorkers
    }
} | ConvertTo-Json -Depth 10

try {
    Write-Host "   Отправка запроса: max_workers $currentWorkers → $newWorkers"
    $updateResponse = Invoke-RestMethod -Uri "$baseUrl/api/workers/config/update" -Method POST -Body $body -ContentType "application/json" -TimeoutSec 10
    Write-Host "   ✅ Ответ: $($updateResponse.message)" -ForegroundColor Green
    
    Start-Sleep -Seconds 1
    
    $checkResponse = Invoke-RestMethod -Uri "$baseUrl/api/workers/config" -Method GET -TimeoutSec 5
    if ($checkResponse.providers.arliai.max_workers -eq $newWorkers) {
        Write-Host "   ✅ Max workers изменен: $newWorkers" -ForegroundColor Green
    } else {
        Write-Host "   ❌ Max workers не изменился!" -ForegroundColor Red
    }
    if ($checkResponse.providers.arliai.has_api_key) {
        Write-Host "   ✅ API ключ сохранен" -ForegroundColor Green
    } else {
        Write-Host "   ❌ API ключ потерян!" -ForegroundColor Red
    }
} catch {
    Write-Host "   ❌ Ошибка: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host "`n=== Тестирование завершено ===" -ForegroundColor Cyan

