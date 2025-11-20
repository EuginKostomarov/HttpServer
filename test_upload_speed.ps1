# Скрипт для тестирования скорости загрузки файлов
# Использование: .\test_upload_speed.ps1 -FilePath "путь\к\файлу.db" -ClientId 1 -ProjectId 1

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

# Создаем FormData
$boundary = [System.Guid]::NewGuid().ToString()
$LF = "`r`n"

$bodyLines = @()
$bodyLines += "--$boundary"
$bodyLines += "Content-Disposition: form-data; name=`"file`"; filename=`"$($fileInfo.Name)`""
$bodyLines += "Content-Type: application/octet-stream"
$bodyLines += ""
$bodyBytes = [System.Text.Encoding]::UTF8.GetBytes($bodyLines -join $LF + $LF)

# Читаем файл
$fileBytes = [System.IO.File]::ReadAllBytes($FilePath)

# Добавляем файл в тело запроса
$bodyBytes += $fileBytes

# Завершаем boundary
$footerBytes = [System.Text.Encoding]::UTF8.GetBytes($LF + "--$boundary--" + $LF)
$bodyBytes += $footerBytes

# Измеряем время загрузки
$startTime = Get-Date
Write-Host "Начало загрузки: $startTime" -ForegroundColor Cyan

try {
    $response = Invoke-WebRequest -Uri "$BackendUrl/api/clients/$ClientId/projects/$ProjectId/databases" `
        -Method POST `
        -ContentType "multipart/form-data; boundary=$boundary" `
        -Body $bodyBytes `
        -TimeoutSec 600 `
        -UseBasicParsing
    
    $endTime = Get-Date
    $duration = $endTime - $startTime
    $durationSeconds = $duration.TotalSeconds
    
    Write-Host ""
    Write-Host "=== Результаты загрузки ===" -ForegroundColor Green
    Write-Host "Статус: $($response.StatusCode)" -ForegroundColor Green
    Write-Host "Время загрузки: $($duration.TotalMilliseconds) мс ($durationSeconds сек)" -ForegroundColor Green
    
    # Вычисляем скорость
    if ($durationSeconds -gt 0) {
        $speedMBps = [math]::Round($fileSizeMB / $durationSeconds, 2)
        $speedMbps = [math]::Round($speedMBps * 8, 2)
        Write-Host "Скорость загрузки: $speedMBps MB/s ($speedMbps Mbps)" -ForegroundColor Green
    } else {
        Write-Host "Скорость загрузки: очень высокая (< 1 мс)" -ForegroundColor Yellow
    }
    
    # Парсим ответ
    try {
        $responseData = $response.Content | ConvertFrom-Json
        if ($responseData.success) {
            Write-Host ""
            Write-Host "✅ Файл успешно загружен!" -ForegroundColor Green
            if ($responseData.file_path) {
                Write-Host "Путь к файлу: $($responseData.file_path)" -ForegroundColor Cyan
            }
        }
    } catch {
        Write-Host "Ответ сервера: $($response.Content)" -ForegroundColor Yellow
    }
    
} catch {
    $endTime = Get-Date
    $duration = $endTime - $startTime
    
    Write-Host ""
    Write-Host "=== Ошибка загрузки ===" -ForegroundColor Red
    Write-Host "Время до ошибки: $($duration.TotalMilliseconds) мс" -ForegroundColor Red
    Write-Host "Ошибка: $($_.Exception.Message)" -ForegroundColor Red
    
    if ($_.Exception.Response) {
        $statusCode = [int]$_.Exception.Response.StatusCode
        Write-Host "HTTP статус: $statusCode" -ForegroundColor Red
        
        try {
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $responseBody = $reader.ReadToEnd()
            Write-Host "Ответ сервера: $responseBody" -ForegroundColor Yellow
        } catch {
            Write-Host "Не удалось прочитать ответ сервера" -ForegroundColor Yellow
        }
    }
    
    exit 1
}

Write-Host ""
Write-Host "=== Тест завершен ===" -ForegroundColor Cyan

