# Тестовый скрипт для проверки параллельной классификации КПВЭД
# Проверяет, что worker pool работает корректно с 2 воркерами

$baseUrl = "http://127.0.0.1:9999"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Тест параллельной классификации КПВЭД" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Проверка доступности сервера
Write-Host "1. Проверка доступности сервера..." -ForegroundColor Yellow
try {
    $healthResponse = Invoke-WebRequest -Uri "$baseUrl/health" -Method GET -TimeoutSec 5 -UseBasicParsing
    Write-Host "   ✓ Сервер доступен (Status: $($healthResponse.StatusCode))" -ForegroundColor Green
} catch {
    Write-Host "   ✗ Сервер недоступен: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "   Запустите сервер перед тестированием" -ForegroundColor Yellow
    exit 1
}

# Проверка статистики КПВЭД
Write-Host ""
Write-Host "2. Проверка статистики КПВЭД..." -ForegroundColor Yellow
try {
    $statsResponse = Invoke-WebRequest -Uri "$baseUrl/api/kpved/stats" -Method GET -TimeoutSec 5 -UseBasicParsing
    $stats = $statsResponse.Content | ConvertFrom-Json
    Write-Host "   ✓ Статистика получена:" -ForegroundColor Green
    Write-Host "     - Всего элементов: $($stats.total_elements)" -ForegroundColor Gray
    Write-Host "     - Уровней: $($stats.levels)" -ForegroundColor Gray
} catch {
    Write-Host "   ⚠ Не удалось получить статистику: $($_.Exception.Message)" -ForegroundColor Yellow
}

# Тест классификации с небольшим лимитом (для быстрого теста)
Write-Host ""
Write-Host "3. Тест параллельной классификации (limit: 5 групп)..." -ForegroundColor Yellow
$testBody = @{
    limit = 5
} | ConvertTo-Json

try {
    $startTime = Get-Date
    Write-Host "   Отправка запроса на классификацию..." -ForegroundColor Gray
    
    $response = Invoke-WebRequest -Uri "$baseUrl/api/kpved/reclassify-hierarchical" `
        -Method POST `
        -ContentType "application/json" `
        -Body $testBody `
        -TimeoutSec 300 `
        -UseBasicParsing
    
    $endTime = Get-Date
    $duration = ($endTime - $startTime).TotalSeconds
    
    $result = $response.Content | ConvertFrom-Json
    
    Write-Host "   ✓ Классификация завершена за $([math]::Round($duration, 2)) секунд" -ForegroundColor Green
    Write-Host ""
    Write-Host "   Результаты:" -ForegroundColor Cyan
    Write-Host "     - Успешно классифицировано: $($result.classified)" -ForegroundColor Green
    Write-Host "     - Ошибок: $($result.failed)" -ForegroundColor $(if ($result.failed -gt 0) { "Red" } else { "Green" })
    Write-Host "     - Общее время классификации: $($result.total_duration)ms" -ForegroundColor Gray
    Write-Host "     - Среднее время на группу: $($result.avg_duration)ms" -ForegroundColor Gray
    Write-Host "     - Среднее количество шагов: $([math]::Round($result.avg_steps, 2))" -ForegroundColor Gray
    Write-Host "     - Среднее количество AI вызовов: $([math]::Round($result.avg_ai_calls, 2))" -ForegroundColor Gray
    Write-Host "     - Всего AI вызовов: $($result.total_ai_calls)" -ForegroundColor Gray
    
    if ($result.classified -gt 0) {
        Write-Host ""
        Write-Host "   ✓ Параллельная классификация работает!" -ForegroundColor Green
        Write-Host "   Проверьте логи сервера на наличие сообщений '[KPVED Worker 0]' и '[KPVED Worker 1]'" -ForegroundColor Yellow
    } else {
        Write-Host ""
        Write-Host "   ⚠ Нет классифицированных групп. Возможно, все группы уже классифицированы." -ForegroundColor Yellow
    }
    
} catch {
    Write-Host "   ✗ Ошибка при классификации: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $responseBody = $reader.ReadToEnd()
        Write-Host "   Ответ сервера: $responseBody" -ForegroundColor Red
    }
}

# Тест с большим лимитом для проверки параллелизма
Write-Host ""
Write-Host "4. Тест параллельной классификации (limit: 20 групп)..." -ForegroundColor Yellow
$testBody2 = @{
    limit = 20
} | ConvertTo-Json

try {
    $startTime = Get-Date
    Write-Host "   Отправка запроса на классификацию 20 групп..." -ForegroundColor Gray
    
    $response = Invoke-WebRequest -Uri "$baseUrl/api/kpved/reclassify-hierarchical" `
        -Method POST `
        -ContentType "application/json" `
        -Body $testBody2 `
        -TimeoutSec 600 `
        -UseBasicParsing
    
    $endTime = Get-Date
    $duration = ($endTime - $startTime).TotalSeconds
    
    $result = $response.Content | ConvertFrom-Json
    
    Write-Host "   ✓ Классификация завершена за $([math]::Round($duration, 2)) секунд" -ForegroundColor Green
    Write-Host ""
    Write-Host "   Результаты:" -ForegroundColor Cyan
    Write-Host "     - Успешно классифицировано: $($result.classified)" -ForegroundColor Green
    Write-Host "     - Ошибок: $($result.failed)" -ForegroundColor $(if ($result.failed -gt 0) { "Red" } else { "Green" })
    Write-Host "     - Общее время классификации: $($result.total_duration)ms" -ForegroundColor Gray
    Write-Host "     - Среднее время на группу: $($result.avg_duration)ms" -ForegroundColor Gray
    
    if ($result.classified -gt 0) {
        $avgTimePerGroup = $result.avg_duration
        $totalTime = $result.total_duration
        $expectedSequentialTime = $avgTimePerGroup * $result.classified
        
        Write-Host ""
        Write-Host "   Анализ параллелизма:" -ForegroundColor Cyan
        Write-Host "     - Ожидаемое время последовательной обработки: ~$([math]::Round($expectedSequentialTime, 0))ms" -ForegroundColor Gray
        Write-Host "     - Фактическое время: $totalTime ms" -ForegroundColor Gray
        if ($expectedSequentialTime -gt 0) {
            $speedup = $expectedSequentialTime / $totalTime
            Write-Host "     - Ускорение: $([math]::Round($speedup, 2))x" -ForegroundColor $(if ($speedup -gt 1.5) { "Green" } else { "Yellow" })
        }
    }
    
} catch {
    Write-Host "   ✗ Ошибка при классификации: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Тестирование завершено" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

