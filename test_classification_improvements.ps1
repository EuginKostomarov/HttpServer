# Тестовый скрипт для проверки улучшений классификации

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Тестирование улучшений классификации" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

$baseUrl = "http://localhost:9999"
$testCases = @(
    @{
        Name = "mq фасонные элементы /q"
        Category = "стройматериалы"
        ExpectedCode = "25.11.11"
        ExpectedName = "Конструкции"
        WrongCode = "96.09.1"
        WrongName = "Услуги"
        Description = "Фасонные элементы для сэндвич панелей"
    },
    @{
        Name = "aks преобразователь (датчик) давления"
        Category = "оборудование"
        ExpectedCode = "26.51.52"
        ExpectedName = "Приборы"
        WrongCode = "71.20.1"
        WrongName = "Услуги по испытаниям"
        Description = "Преобразователь давления AKS"
    },
    @{
        Name = "helukabel (контрольный) jz-mh"
        Category = "электроника"
        ExpectedCode = "27.32.11"
        ExpectedName = "Кабели"
        WrongCode = "26.12.1"
        WrongName = "Платы"
        Description = "Контрольный кабель HELUKABEL"
    },
    @{
        Name = "mq фасонные элементы ral sv"
        Category = "стройматериалы"
        ExpectedCode = "25.11.11"
        ExpectedName = "Конструкции"
        WrongCode = "32.99.5"
        WrongName = "Прочие изделия"
        Description = "Фасонные элементы для панелей"
    }
)

Write-Host "Тестируем $($testCases.Count) проблемных случаев..." -ForegroundColor Yellow
Write-Host ""

$successCount = 0
$failCount = 0
$results = @()

foreach ($testCase in $testCases) {
    Write-Host "Тест: $($testCase.Name)" -ForegroundColor Cyan
    Write-Host "  Описание: $($testCase.Description)" -ForegroundColor Gray
    Write-Host "  Категория: $($testCase.Category)" -ForegroundColor Gray
    Write-Host ""
    
    $body = @{
        normalized_name = $testCase.Name
        category = $testCase.Category
    } | ConvertTo-Json
    
    try {
        $response = Invoke-RestMethod -Uri "$baseUrl/api/kpved/classify-hierarchical" `
            -Method POST `
            -Body $body `
            -ContentType "application/json" `
            -TimeoutSec 30 `
            -ErrorAction Stop
        
        $result = @{
            TestName = $testCase.Name
            Success = $false
            Code = $response.final_code
            Name = $response.final_name
            Confidence = $response.final_confidence
            ExpectedCode = $testCase.ExpectedCode
            WrongCode = $testCase.WrongCode
            Message = ""
        }
        
        # Проверяем результат
        if ($response.final_code -eq $testCase.ExpectedCode -or 
            $response.final_code.StartsWith($testCase.ExpectedCode.Substring(0, 5))) {
            $result.Success = $true
            $result.Message = "✓ Правильно классифицирован"
            $successCount++
            Write-Host "  ✓ УСПЕХ: $($response.final_code) - $($response.final_name)" -ForegroundColor Green
            Write-Host "    Уверенность: $([math]::Round($response.final_confidence * 100, 1))%" -ForegroundColor Green
        } elseif ($response.final_code -eq $testCase.WrongCode) {
            $result.Message = "✗ ОШИБКА: Классифицирован как $($testCase.WrongName)"
            $failCount++
            Write-Host "  ✗ ОШИБКА: $($response.final_code) - $($response.final_name)" -ForegroundColor Red
            Write-Host "    Ожидалось: $($testCase.ExpectedCode) - $($testCase.ExpectedName)" -ForegroundColor Yellow
            Write-Host "    Получено: $($response.final_code) - $($response.final_name)" -ForegroundColor Red
        } else {
            $result.Message = "? Неожиданный код: $($response.final_code)"
            $failCount++
            Write-Host "  ? НЕОЖИДАННО: $($response.final_code) - $($response.final_name)" -ForegroundColor Yellow
            Write-Host "    Ожидалось: $($testCase.ExpectedCode) - $($testCase.ExpectedName)" -ForegroundColor Yellow
        }
        
        # Показываем шаги классификации, если есть
        if ($response.steps) {
            Write-Host "  Шаги классификации:" -ForegroundColor Gray
            foreach ($step in $response.steps) {
                Write-Host "    - $($step.level_name): $($step.code) ($($step.name)) [уверенность: $([math]::Round($step.confidence * 100, 1))%]" -ForegroundColor Gray
            }
        }
        
        $results += $result
    }
    catch {
        $failCount++
        Write-Host "  ✗ ОШИБКА запроса: $($_.Exception.Message)" -ForegroundColor Red
        $results += @{
            TestName = $testCase.Name
            Success = $false
            Code = "ERROR"
            Name = $_.Exception.Message
            Confidence = 0
            ExpectedCode = $testCase.ExpectedCode
            WrongCode = $testCase.WrongCode
            Message = "Ошибка запроса: $($_.Exception.Message)"
        }
    }
    
    Write-Host ""
    Start-Sleep -Milliseconds 500
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Результаты тестирования" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Успешно: $successCount / $($testCases.Count)" -ForegroundColor $(if ($successCount -eq $testCases.Count) { "Green" } else { "Yellow" })
Write-Host "Ошибок:  $failCount / $($testCases.Count)" -ForegroundColor $(if ($failCount -eq 0) { "Green" } else { "Red" })
Write-Host ""

if ($successCount -eq $testCases.Count) {
    Write-Host "✓ Все тесты пройдены успешно!" -ForegroundColor Green
    Write-Host "Улучшения классификации работают корректно." -ForegroundColor Green
} else {
    Write-Host "⚠ Некоторые тесты не прошли." -ForegroundColor Yellow
    Write-Host "Проверьте логи сервера для деталей." -ForegroundColor Yellow
}

Write-Host ""
Write-Host "Детальные результаты:" -ForegroundColor Cyan
foreach ($result in $results) {
    $color = if ($result.Success) { "Green" } else { "Red" }
    Write-Host "  $($result.Message): $($result.TestName)" -ForegroundColor $color
    if (-not $result.Success) {
        Write-Host "    Код: $($result.Code), Ожидалось: $($result.ExpectedCode)" -ForegroundColor Gray
    }
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan

