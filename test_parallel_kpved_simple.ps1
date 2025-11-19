# Простой тест для проверки параллельной классификации КПВЭД
# Проверяет базовую функциональность worker pool

$baseUrl = "http://127.0.0.1:9999"

Write-Host "Тест параллельной классификации КПВЭД" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host ""

# Проверка доступности сервера
Write-Host "Проверка доступности сервера..." -ForegroundColor Yellow
try {
    $null = Invoke-WebRequest -Uri "$baseUrl/health" -Method GET -TimeoutSec 5 -UseBasicParsing -ErrorAction Stop
    Write-Host "✓ Сервер доступен" -ForegroundColor Green
} catch {
    Write-Host "✗ Сервер недоступен. Запустите сервер перед тестированием." -ForegroundColor Red
    exit 1
}

# Тест с небольшим лимитом
Write-Host ""
Write-Host "Тест классификации (limit: 3 группы)..." -ForegroundColor Yellow
$body = @{ limit = 3 } | ConvertTo-Json

try {
    $startTime = Get-Date
    $response = Invoke-WebRequest -Uri "$baseUrl/api/kpved/reclassify-hierarchical" `
        -Method POST `
        -ContentType "application/json" `
        -Body $body `
        -TimeoutSec 120 `
        -UseBasicParsing
    
    $duration = ((Get-Date) - $startTime).TotalSeconds
    $result = $response.Content | ConvertFrom-Json
    
    Write-Host "✓ Классификация завершена за $([math]::Round($duration, 2)) сек" -ForegroundColor Green
    Write-Host "  Успешно: $($result.classified), Ошибок: $($result.failed)" -ForegroundColor $(if ($result.failed -eq 0) { "Green" } else { "Yellow" })
    
    if ($result.classified -gt 0) {
        Write-Host "  Среднее время на группу: $($result.avg_duration)ms" -ForegroundColor Gray
        Write-Host "  Всего AI вызовов: $($result.total_ai_calls)" -ForegroundColor Gray
        Write-Host ""
        Write-Host "✓ Параллельная классификация работает!" -ForegroundColor Green
        Write-Host "  Проверьте логи сервера на наличие '[KPVED Worker 0]' и '[KPVED Worker 1]'" -ForegroundColor Yellow
    }
} catch {
    Write-Host "✗ Ошибка: $($_.Exception.Message)" -ForegroundColor Red
}

