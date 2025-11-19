# Скрипт для тестирования всех основных API endpoints
$baseUrl = "http://localhost:9999"
$results = @()

function Test-Endpoint {
    param(
        [string]$Name,
        [string]$Method,
        [string]$Url,
        [string]$Body = $null,
        [hashtable]$Headers = @{}
    )
    
    Write-Host "`nТестирование: $Name" -ForegroundColor Cyan
    Write-Host "  $Method $Url" -ForegroundColor Gray
    
    try {
        $params = @{
            Uri = $Url
            Method = $Method
            TimeoutSec = 7
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
        $status = "SUCCESS"
        $statusCode = $response.StatusCode
        $content = $response.Content | ConvertFrom-Json -ErrorAction SilentlyContinue
        
        Write-Host "  ✓ Status: $statusCode" -ForegroundColor Green
        
        $results += [PSCustomObject]@{
            Name = $Name
            Method = $Method
            Url = $Url
            Status = $status
            StatusCode = $statusCode
            Response = $content
        }
        
        return $true
    }
    catch {
        $status = "FAILED"
        $statusCode = $_.Exception.Response.StatusCode.value__
        $errorMsg = $_.Exception.Message
        
        Write-Host "  ✗ Status: $statusCode - $errorMsg" -ForegroundColor Red
        
        $results += [PSCustomObject]@{
            Name = $Name
            Method = $Method
            Url = $Url
            Status = $status
            StatusCode = $statusCode
            Error = $errorMsg
        }
        
        return $false
    }
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Тестирование API Endpoints" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# Health check
Test-Endpoint -Name "Health Check" -Method "GET" -Url "$baseUrl/api/v1/health"

# Database endpoints
Test-Endpoint -Name "Database Info" -Method "GET" -Url "$baseUrl/api/database/info"
Test-Endpoint -Name "Databases List" -Method "GET" -Url "$baseUrl/api/databases/list"

# Normalization endpoints
Test-Endpoint -Name "Normalization Status" -Method "GET" -Url "$baseUrl/api/normalization/status"
Test-Endpoint -Name "Normalization Stats" -Method "GET" -Url "$baseUrl/api/normalization/stats"
Test-Endpoint -Name "Normalization Config" -Method "GET" -Url "$baseUrl/api/normalization/config"
Test-Endpoint -Name "Normalization Databases" -Method "GET" -Url "$baseUrl/api/normalization/databases"

# KPVED endpoints
Test-Endpoint -Name "KPVED Stats" -Method "GET" -Url "$baseUrl/api/kpved/stats"
Test-Endpoint -Name "KPVED Hierarchy" -Method "GET" -Url "$baseUrl/api/kpved/hierarchy"

# Quality endpoints
Test-Endpoint -Name "Quality Stats" -Method "GET" -Url "$baseUrl/api/quality/stats"
Test-Endpoint -Name "Quality Analyze Status" -Method "GET" -Url "$baseUrl/api/quality/analyze/status"

# Classification endpoints
Test-Endpoint -Name "Get Classifiers" -Method "GET" -Url "$baseUrl/api/classification/classifiers"
Test-Endpoint -Name "Get Strategies" -Method "GET" -Url "$baseUrl/api/classification/strategies"
Test-Endpoint -Name "Get Available Strategies" -Method "GET" -Url "$baseUrl/api/classification/available"

# Reclassification endpoints
Test-Endpoint -Name "Reclassification Status" -Method "GET" -Url "$baseUrl/api/reclassification/status"

# Monitoring endpoints
Test-Endpoint -Name "Monitoring Metrics" -Method "GET" -Url "$baseUrl/api/monitoring/metrics"

# Workers endpoints
Test-Endpoint -Name "Worker Config" -Method "GET" -Url "$baseUrl/api/workers/config"
Test-Endpoint -Name "Worker Providers" -Method "GET" -Url "$baseUrl/api/workers/providers"
Test-Endpoint -Name "Worker Models" -Method "GET" -Url "$baseUrl/api/workers/models"
Test-Endpoint -Name "ArliAI Status" -Method "GET" -Url "$baseUrl/api/workers/arliai/status"

# Clients endpoints
Test-Endpoint -Name "Clients List" -Method "GET" -Url "$baseUrl/api/clients"

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
$results | Format-Table -AutoSize

# Сохраняем результаты в файл
$results | ConvertTo-Json -Depth 5 | Out-File -FilePath "api_test_results.json" -Encoding UTF8
Write-Host "`nРезультаты сохранены в api_test_results.json" -ForegroundColor Green

