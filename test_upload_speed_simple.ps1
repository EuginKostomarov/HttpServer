# Упрощенный скрипт для тестирования скорости загрузки с помощью curl
# Использование: .\test_upload_speed_simple.ps1 -FilePath "путь\к\файлу.db" -ClientId 1 -ProjectId 1

param(
    [Parameter(Mandatory=$true)]
    [string]$FilePath,
    
    [Parameter(Mandatory=$true)]
    [int]$ClientId,
    
    [Parameter(Mandatory=$true)]
    [int]$ProjectId,
    
    [string]$BackendUrl = "http://localhost:9999"
)

Write-Host "=== Тест скорости загрузки файла ===" -ForegroundColor Cyan
Write-Host "Файл: $FilePath" -ForegroundColor Yellow
Write-Host "Клиент: $ClientId, Проект: $ProjectId" -ForegroundColor Yellow
Write-Host ""

# Проверяем существование файла
if (-not (Test-Path $FilePath)) {
    Write-Host "Ошибка: Файл не найден: $FilePath" -ForegroundColor Red
    exit 1
}

# Получаем размер файла
$fileInfo = Get-Item $FilePath
$fileSizeMB = [math]::Round($fileInfo.Length / 1MB, 2)
$fileSizeBytes = $fileInfo.Length

Write-Host "Размер файла: $fileSizeMB MB ($fileSizeBytes байт)" -ForegroundColor Green
Write-Host ""

# Измеряем время загрузки
$startTime = Get-Date
Write-Host "Начало загрузки: $startTime" -ForegroundColor Cyan

try {
    # Используем curl для загрузки файла
    $response = curl.exe -X POST `
        -F "file=@$FilePath" `
        -F "auto_create=false" `
        -w "`nHTTP_CODE:%{http_code}`nTIME_TOTAL:%{time_total}`nSPEED_DOWNLOAD:%{speed_download}`nSIZE_UPLOAD:%{size_upload}`n" `
        -s -S `
        --max-time 600 `
        "$BackendUrl/api/clients/$ClientId/projects/$ProjectId/databases"
    
    $endTime = Get-Date
    $duration = $endTime - $startTime
    $durationSeconds = $duration.TotalSeconds
    
    Write-Host ""
    Write-Host "=== Результаты загрузки ===" -ForegroundColor Green
    
    # Парсим вывод curl
    $httpCode = ($response | Select-String "HTTP_CODE:(\d+)").Matches.Groups[1].Value
    $timeTotal = ($response | Select-String "TIME_TOTAL:([\d.]+)").Matches.Groups[1].Value
    $speedDownload = ($response | Select-String "SPEED_DOWNLOAD:([\d.]+)").Matches.Groups[1].Value
    $sizeUpload = ($response | Select-String "SIZE_UPLOAD:([\d.]+)").Matches.Groups[1].Value
    
    Write-Host "HTTP статус: $httpCode" -ForegroundColor $(if ($httpCode -eq "200" -or $httpCode -eq "201") { "Green" } else { "Red" })
    Write-Host "Время загрузки (curl): $timeTotal сек" -ForegroundColor Green
    Write-Host "Время загрузки (PowerShell): $([math]::Round($durationSeconds, 3)) сек" -ForegroundColor Green
    
    if ($speedDownload) {
        $speedMBps = [math]::Round([double]$speedDownload / 1024 / 1024, 2)
        Write-Host "Скорость загрузки (curl): $speedMBps MB/s" -ForegroundColor Green
    }
    
    if ($durationSeconds -gt 0) {
        $speedMBps = [math]::Round($fileSizeMB / $durationSeconds, 2)
        $speedMbps = [math]::Round($speedMBps * 8, 2)
        Write-Host "Скорость загрузки (расчетная): $speedMBps MB/s ($speedMbps Mbps)" -ForegroundColor Green
    }
    
    # Парсим JSON ответ
    $jsonResponse = $response | Select-String -Pattern '^\s*\{.*\}\s*$' | ForEach-Object { $_.Line }
    if ($jsonResponse) {
        try {
            $responseData = $jsonResponse | ConvertFrom-Json
            if ($responseData.success) {
                Write-Host ""
                Write-Host "✅ Файл успешно загружен!" -ForegroundColor Green
                if ($responseData.file_path) {
                    Write-Host "Путь к файлу: $($responseData.file_path)" -ForegroundColor Cyan
                }
            }
        } catch {
            Write-Host "Ответ сервера: $jsonResponse" -ForegroundColor Yellow
        }
    }
    
} catch {
    $endTime = Get-Date
    $duration = $endTime - $startTime
    
    Write-Host ""
    Write-Host "=== Ошибка загрузки ===" -ForegroundColor Red
    Write-Host "Время до ошибки: $($duration.TotalMilliseconds) мс" -ForegroundColor Red
    Write-Host "Ошибка: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "=== Тест завершен ===" -ForegroundColor Cyan

