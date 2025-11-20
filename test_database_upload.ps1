# Test database upload to service
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Database Upload Test" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

$baseUrl = "http://localhost:9999"
$filePath = "data\uploads\Выгрузка_Контрагенты_ERPWE_Unknown_Unknown_2025_11_20_13_27_39.db"

# Test 1: Check file existence
Write-Host "[1/5] Checking file..." -ForegroundColor Yellow
if (Test-Path $filePath) {
    $file = Get-Item $filePath
    Write-Host "  OK File found: $($file.Name)" -ForegroundColor Green
    Write-Host "  Size: $([math]::Round($file.Length/1MB, 2)) MB" -ForegroundColor Gray
} else {
    Write-Host "  ERROR File not found: $filePath" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Test 2: Scan directory
Write-Host "[2/5] Scanning data/uploads directory..." -ForegroundColor Yellow
try {
    $scanBody = @{
        paths = @("data/uploads")
    } | ConvertTo-Json
    
    $scanResponse = Invoke-RestMethod -Uri "$baseUrl/api/databases/scan" -Method POST -Body $scanBody -ContentType "application/json" -ErrorAction Stop
    Write-Host "  OK Scan completed successfully" -ForegroundColor Green
    Write-Host "  Found files: $($scanResponse.found_files)" -ForegroundColor Gray
    if ($scanResponse.files) {
        Write-Host "  Files:" -ForegroundColor Gray
        foreach ($file in $scanResponse.files) {
            Write-Host "    - $file" -ForegroundColor Gray
        }
    }
} catch {
    Write-Host "  ERROR Scan failed: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $responseBody = $reader.ReadToEnd()
        Write-Host "  Server response: $responseBody" -ForegroundColor Red
    }
}
Write-Host ""

# Test 3: Get pending databases list
Write-Host "[3/5] Getting pending databases list..." -ForegroundColor Yellow
try {
    $pendingResponse = Invoke-RestMethod -Uri "$baseUrl/api/databases/pending" -Method GET -ErrorAction Stop
    Write-Host "  OK List retrieved successfully" -ForegroundColor Green
    Write-Host "  Total pending databases: $($pendingResponse.total)" -ForegroundColor Gray
    
    if ($pendingResponse.databases -and $pendingResponse.databases.Count -gt 0) {
        Write-Host "  Databases:" -ForegroundColor Gray
        foreach ($db in $pendingResponse.databases) {
            Write-Host "    ID: $($db.id), Name: $($db.file_name), Path: $($db.file_path)" -ForegroundColor Gray
        }
        
        $script:testDbId = $pendingResponse.databases[0].id
        $script:testDb = $pendingResponse.databases[0]
    } else {
        Write-Host "  WARNING No pending databases found" -ForegroundColor Yellow
        $script:testDbId = $null
    }
} catch {
    Write-Host "  ERROR Failed to get list: $($_.Exception.Message)" -ForegroundColor Red
    $script:testDbId = $null
}
Write-Host ""

# Test 4: Get specific database info
if ($script:testDbId) {
    Write-Host "[4/5] Getting database info for ID: $($script:testDbId)..." -ForegroundColor Yellow
    try {
        $dbInfoResponse = Invoke-RestMethod -Uri "$baseUrl/api/databases/pending/$($script:testDbId)" -Method GET -ErrorAction Stop
        Write-Host "  OK Info retrieved successfully" -ForegroundColor Green
        Write-Host "  ID: $($dbInfoResponse.id)" -ForegroundColor Gray
        Write-Host "  File name: $($dbInfoResponse.file_name)" -ForegroundColor Gray
        Write-Host "  Path: $($dbInfoResponse.file_path)" -ForegroundColor Gray
        Write-Host "  Size: $([math]::Round($dbInfoResponse.file_size/1MB, 2)) MB" -ForegroundColor Gray
        Write-Host "  Status: $($dbInfoResponse.status)" -ForegroundColor Gray
    } catch {
        Write-Host "  ERROR Failed to get info: $($_.Exception.Message)" -ForegroundColor Red
    }
    Write-Host ""
} else {
    Write-Host "[4/5] Skipped (no pending databases)" -ForegroundColor Yellow
    Write-Host ""
}

# Test 5: Get clients list
Write-Host "[5/5] Getting clients list..." -ForegroundColor Yellow
try {
    $clientsResponse = Invoke-RestMethod -Uri "$baseUrl/api/clients" -Method GET -ErrorAction Stop
    Write-Host "  OK Clients list retrieved" -ForegroundColor Green
    Write-Host "  Total clients: $($clientsResponse.Count)" -ForegroundColor Gray
    
    if ($clientsResponse -and $clientsResponse.Count -gt 0) {
        Write-Host "  Clients:" -ForegroundColor Gray
        foreach ($client in $clientsResponse) {
            Write-Host "    ID: $($client.id), Name: $($client.name)" -ForegroundColor Gray
        }
    } else {
        Write-Host "  WARNING No clients found (need to create client and project for binding)" -ForegroundColor Yellow
    }
} catch {
    Write-Host "  ERROR Failed to get clients: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Test completed" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
