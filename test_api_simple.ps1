$baseUrl = "http://localhost:9999"
$endpoints = @(
    "api/v1/health",
    "api/database/info",
    "api/databases/list",
    "api/normalization/status",
    "api/normalization/stats",
    "api/kpved/stats",
    "api/quality/stats",
    "api/classification/classifiers",
    "api/workers/config",
    "api/clients"
)

Write-Host "Testing API Endpoints..." -ForegroundColor Cyan
$success = 0
$failed = 0

foreach ($ep in $endpoints) {
    $url = "$baseUrl/$ep"
    try {
        $response = Invoke-WebRequest -Uri $url -TimeoutSec 7 -UseBasicParsing
        Write-Host "SUCCESS: $ep - $($response.StatusCode)" -ForegroundColor Green
        $success++
    }
    catch {
        Write-Host "FAILED: $ep - $($_.Exception.Message)" -ForegroundColor Red
        $failed++
    }
}

Write-Host "`nSummary: $success successful, $failed failed" -ForegroundColor Cyan

