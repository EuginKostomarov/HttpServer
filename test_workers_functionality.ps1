# Скрипт для тестирования функциональности воркеров через API
$baseUrl = "http://localhost:9999"
$results = @()

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Тестирование функциональности воркеров" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

function Test-Endpoint {
    param(
        [string]$Name,
        [string]$Method,
        [string]$Url,
        [string]$Body = $null,
        [hashtable]$Headers = @{}
    )
    
    Write-Host "Тестирование: $Name" -ForegroundColor Cyan
    Write-Host "  $Method $Url" -ForegroundColor Gray
    
    try {
        $params = @{
            Uri = $Url
            Method = $Method
            TimeoutSec = 15
            UseBasicParsing = $true
        }
        
        if ($Headers.Count -gt 0) {
            $params.Headers = $Headers
        }
        
        if ($Body) {
            $params.Body = $Body
            $params.ContentType = "application/json"
        }
        
        $response = Invoke-WebRequest @params
        $statusCode = $response.StatusCode
        $content = $response.Content | ConvertFrom-Json -ErrorAction SilentlyContinue
        
        Write-Host "  ✓ Status: $statusCode" -ForegroundColor Green
        
        $results += [PSCustomObject]@{
            Name = $Name
            Method = $Method
            Url = $Url
            Status = "SUCCESS"
            StatusCode = $statusCode
            Response = $content
        }
        
        return $content
    }
    catch {
        $statusCode = if ($_.Exception.Response) { $_.Exception.Response.StatusCode.value__ } else { "N/A" }
        $errorMsg = $_.Exception.Message
        
        Write-Host "  ✗ Status: $statusCode - $errorMsg" -ForegroundColor Red
        
        $results += [PSCustomObject]@{
            Name = $Name
            Method = $Method
            Url = $Url
            Status = "FAILED"
            StatusCode = $statusCode
            Error = $errorMsg
        }
        
        return $null
    }
}

# Тест 1: Получение списка моделей
Write-Host "`n[1] Тест загрузки моделей" -ForegroundColor Yellow
$modelsResponse = Test-Endpoint -Name "Get Models" -Method "GET" -Url "$baseUrl/api/workers/models"

if ($modelsResponse -and $modelsResponse.success) {
    $modelsCount = if ($modelsResponse.data.models) { $modelsResponse.data.models.Count } else { 0 }
    Write-Host "  Найдено моделей: $modelsCount" -ForegroundColor Green
    
    if ($modelsCount -gt 0) {
        Write-Host "  Первые модели:" -ForegroundColor Gray
        $modelsResponse.data.models | Select-Object -First 3 | ForEach-Object {
            Write-Host "    - $($_.name) (enabled: $($_.enabled), priority: $($_.priority))" -ForegroundColor Gray
        }
    }
}

# Тест 2: Получение списка провайдеров
Write-Host "`n[2] Тест получения провайдеров" -ForegroundColor Yellow
$providersResponse = Test-Endpoint -Name "Get Providers" -Method "GET" -Url "$baseUrl/api/workers/providers"

if ($providersResponse -and $providersResponse.success) {
    $providersCount = if ($providersResponse.data.providers) { $providersResponse.data.providers.Count } else { 0 }
    Write-Host "  Найдено провайдеров: $providersCount" -ForegroundColor Green
    
    if ($providersCount -gt 0) {
        Write-Host "  Провайдеры:" -ForegroundColor Gray
        $providersResponse.data.providers | ForEach-Object {
            Write-Host "    - $($_.name) (enabled: $($_.enabled), priority: $($_.priority), max_workers: $($_.max_workers))" -ForegroundColor Gray
        }
    }
}

# Тест 3: Обновление приоритета провайдера (если есть провайдер arliai)
Write-Host "`n[3] Тест обновления приоритета провайдера" -ForegroundColor Yellow
if ($providersResponse -and $providersResponse.data.providers) {
    $arliaiProvider = $providersResponse.data.providers | Where-Object { $_.name -eq "arliai" } | Select-Object -First 1
    
    if ($arliaiProvider) {
        $currentPriority = $arliaiProvider.priority
        $newPriority = if ($currentPriority -eq 1) { 2 } else { 1 }
        
        Write-Host "  Текущий приоритет arliai: $currentPriority" -ForegroundColor Gray
        Write-Host "  Устанавливаем новый приоритет: $newPriority" -ForegroundColor Gray
        
        $updateBody = @{
            action = "update_provider"
            data = @{
                name = "arliai"
                priority = $newPriority
            }
        } | ConvertTo-Json
        
        $updateResponse = Test-Endpoint -Name "Update Provider Priority" -Method "POST" -Url "$baseUrl/api/workers/config/update" -Body $updateBody
        
        if ($updateResponse) {
            Write-Host "  ✓ Приоритет обновлен" -ForegroundColor Green
            
            # Возвращаем обратно
            Start-Sleep -Seconds 1
            $restoreBody = @{
                action = "update_provider"
                data = @{
                    name = "arliai"
                    priority = $currentPriority
                }
            } | ConvertTo-Json
            
            Test-Endpoint -Name "Restore Provider Priority" -Method "POST" -Url "$baseUrl/api/workers/config/update" -Body $restoreBody | Out-Null
            Write-Host "  ✓ Приоритет восстановлен" -ForegroundColor Green
        }
    } else {
        Write-Host "  ⚠ Провайдер arliai не найден" -ForegroundColor Yellow
    }
} else {
    Write-Host "  ⚠ Не удалось получить список провайдеров" -ForegroundColor Yellow
}

# Тест 4: Обновление приоритета модели (если есть модели)
Write-Host "`n[4] Тест обновления приоритета модели" -ForegroundColor Yellow
if ($modelsResponse -and $modelsResponse.data.models) {
    $firstModel = $modelsResponse.data.models | Where-Object { $_.enabled -eq $true } | Select-Object -First 1
    
    if ($firstModel) {
        $currentPriority = $firstModel.priority
        $newPriority = if ($currentPriority -eq 1) { 2 } else { 1 }
        
        Write-Host "  Модель: $($firstModel.name)" -ForegroundColor Gray
        Write-Host "  Текущий приоритет: $currentPriority" -ForegroundColor Gray
        Write-Host "  Устанавливаем новый приоритет: $newPriority" -ForegroundColor Gray
        
        $updateBody = @{
            action = "update_model"
            data = @{
                provider = $firstModel.provider
                name = $firstModel.name
                priority = $newPriority
            }
        } | ConvertTo-Json
        
        $updateResponse = Test-Endpoint -Name "Update Model Priority" -Method "POST" -Url "$baseUrl/api/workers/config/update" -Body $updateBody
        
        if ($updateResponse) {
            Write-Host "  ✓ Приоритет модели обновлен" -ForegroundColor Green
            
            # Возвращаем обратно
            Start-Sleep -Seconds 1
            $restoreBody = @{
                action = "update_model"
                data = @{
                    provider = $firstModel.provider
                    name = $firstModel.name
                    priority = $currentPriority
                }
            } | ConvertTo-Json
            
            Test-Endpoint -Name "Restore Model Priority" -Method "POST" -Url "$baseUrl/api/workers/config/update" -Body $restoreBody | Out-Null
            Write-Host "  ✓ Приоритет модели восстановлен" -ForegroundColor Green
        }
    } else {
        Write-Host "  ⚠ Нет включенных моделей для тестирования" -ForegroundColor Yellow
    }
} else {
    Write-Host "  ⚠ Не удалось получить список моделей" -ForegroundColor Yellow
}

# Тест 5: Включение/выключение модели
Write-Host "`n[5] Тест включения/выключения модели" -ForegroundColor Yellow
if ($modelsResponse -and $modelsResponse.data.models) {
    $testModel = $modelsResponse.data.models | Select-Object -First 1
    
    if ($testModel) {
        $currentEnabled = $testModel.enabled
        $newEnabled = -not $currentEnabled
        
        Write-Host "  Модель: $($testModel.name)" -ForegroundColor Gray
        Write-Host "  Текущее состояние: $currentEnabled" -ForegroundColor Gray
        Write-Host "  Устанавливаем: $newEnabled" -ForegroundColor Gray
        
        $updateBody = @{
            action = "update_model"
            data = @{
                provider = $testModel.provider
                name = $testModel.name
                enabled = $newEnabled
            }
        } | ConvertTo-Json
        
        $updateResponse = Test-Endpoint -Name "Toggle Model" -Method "POST" -Url "$baseUrl/api/workers/config/update" -Body $updateBody
        
        if ($updateResponse) {
            Write-Host "  ✓ Состояние модели обновлено" -ForegroundColor Green
            
            # Возвращаем обратно
            Start-Sleep -Seconds 1
            $restoreBody = @{
                action = "update_model"
                data = @{
                    provider = $testModel.provider
                    name = $testModel.name
                    enabled = $currentEnabled
                }
            } | ConvertTo-Json
            
            Test-Endpoint -Name "Restore Model State" -Method "POST" -Url "$baseUrl/api/workers/config/update" -Body $restoreBody | Out-Null
            Write-Host "  ✓ Состояние модели восстановлено" -ForegroundColor Green
        }
    } else {
        Write-Host "  ⚠ Нет моделей для тестирования" -ForegroundColor Yellow
    }
} else {
    Write-Host "  ⚠ Не удалось получить список моделей" -ForegroundColor Yellow
}

# Тест 6: Обновление max_workers провайдера
Write-Host "`n[6] Тест обновления max_workers провайдера" -ForegroundColor Yellow
if ($providersResponse -and $providersResponse.data.providers) {
    $arliaiProvider = $providersResponse.data.providers | Where-Object { $_.name -eq "arliai" } | Select-Object -First 1
    
    if ($arliaiProvider) {
        $currentWorkers = $arliaiProvider.max_workers
        $newWorkers = if ($currentWorkers -eq 2) { 3 } else { 2 }
        
        Write-Host "  Текущее max_workers arliai: $currentWorkers" -ForegroundColor Gray
        Write-Host "  Устанавливаем новое max_workers: $newWorkers" -ForegroundColor Gray
        
        $updateBody = @{
            action = "update_provider"
            data = @{
                name = "arliai"
                max_workers = $newWorkers
            }
        } | ConvertTo-Json
        
        $updateResponse = Test-Endpoint -Name "Update Provider Max Workers" -Method "POST" -Url "$baseUrl/api/workers/config/update" -Body $updateBody
        
        if ($updateResponse) {
            Write-Host "  ✓ Max workers обновлено" -ForegroundColor Green
            
            # Возвращаем обратно
            Start-Sleep -Seconds 1
            $restoreBody = @{
                action = "update_provider"
                data = @{
                    name = "arliai"
                    max_workers = $currentWorkers
                }
            } | ConvertTo-Json
            
            Test-Endpoint -Name "Restore Provider Max Workers" -Method "POST" -Url "$baseUrl/api/workers/config/update" -Body $restoreBody | Out-Null
            Write-Host "  ✓ Max workers восстановлено" -ForegroundColor Green
        }
    } else {
        Write-Host "  ⚠ Провайдер arliai не найден" -ForegroundColor Yellow
    }
} else {
    Write-Host "  ⚠ Не удалось получить список провайдеров" -ForegroundColor Yellow
}

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "Результаты тестирования" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

$successCount = ($results | Where-Object { $_.Status -eq "SUCCESS" }).Count
$failedCount = ($results | Where-Object { $_.Status -eq "FAILED" }).Count
$totalCount = $results.Count

Write-Host "`nВсего протестировано: $totalCount" -ForegroundColor White
Write-Host "Успешно: $successCount" -ForegroundColor Green
Write-Host "Ошибок: $failedCount" -ForegroundColor $(if ($failedCount -gt 0) { "Red" } else { "Green" })

Write-Host "`nДетальные результаты:" -ForegroundColor Cyan
$results | Format-Table -AutoSize Name, Method, Status, StatusCode

# Сохраняем результаты
$results | ConvertTo-Json -Depth 5 | Out-File -FilePath "test_workers_results.json" -Encoding UTF8
Write-Host "`nРезультаты сохранены в test_workers_results.json" -ForegroundColor Green

