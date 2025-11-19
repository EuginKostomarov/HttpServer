# Скрипт для перезапуска сервера с применением обновлений классификации

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Перезапуск сервера для применения" -ForegroundColor Cyan
Write-Host "улучшений классификации" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Переходим в директорию проекта
Set-Location $PSScriptRoot

# Ищем процесс сервера
Write-Host "[1/4] Поиск запущенного сервера..." -ForegroundColor Yellow
$serverProcesses = Get-Process | Where-Object {
    $_.ProcessName -like "*httpserver*" -or 
    ($_.ProcessName -eq "go" -and $_.MainWindowTitle -like "*server*")
}

if ($serverProcesses) {
    Write-Host "Найдено процессов: $($serverProcesses.Count)" -ForegroundColor Yellow
    foreach ($proc in $serverProcesses) {
        Write-Host "  - PID: $($proc.Id), Имя: $($proc.ProcessName), Запущен: $($proc.StartTime)" -ForegroundColor Gray
    }
    
    Write-Host ""
    $confirm = Read-Host "Остановить эти процессы? (Y/N)"
    if ($confirm -eq "Y" -or $confirm -eq "y") {
        foreach ($proc in $serverProcesses) {
            Write-Host "Остановка процесса PID $($proc.Id)..." -ForegroundColor Yellow
            Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
        }
        Start-Sleep -Seconds 2
        Write-Host "Процессы остановлены" -ForegroundColor Green
    } else {
        Write-Host "Перезапуск отменен" -ForegroundColor Red
        exit 0
    }
} else {
    Write-Host "Запущенный сервер не найден" -ForegroundColor Green
}

# Пересборка проекта
Write-Host ""
Write-Host "[2/4] Пересборка проекта..." -ForegroundColor Yellow
$buildOutput = go build -o httpserver.exe . 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "Ошибка сборки:" -ForegroundColor Red
    Write-Host $buildOutput -ForegroundColor Red
    Read-Host "Нажмите Enter для выхода"
    exit 1
}
Write-Host "Сборка завершена успешно" -ForegroundColor Green

# Проверка наличия API ключа
Write-Host ""
Write-Host "[3/4] Проверка конфигурации..." -ForegroundColor Yellow
$apiKey = $env:ARLIAI_API_KEY
if (-not $apiKey) {
    Write-Host "ВНИМАНИЕ: ARLIAI_API_KEY не установлен!" -ForegroundColor Yellow
    Write-Host "Установите переменную окружения перед запуском:" -ForegroundColor Yellow
    Write-Host '  $env:ARLIAI_API_KEY = "ваш-api-ключ"' -ForegroundColor Cyan
} else {
    Write-Host "API ключ найден (длина: $($apiKey.Length))" -ForegroundColor Green
}

# Запуск сервера
Write-Host ""
Write-Host "[4/4] Запуск сервера..." -ForegroundColor Yellow
Write-Host "Сервер будет запущен в новом окне PowerShell" -ForegroundColor Cyan
Write-Host ""

$serverCommand = "cd '$PSScriptRoot'; if (`$env:ARLIAI_API_KEY) { Write-Host 'API ключ установлен' } else { Write-Host 'ВНИМАНИЕ: API ключ не установлен!' -ForegroundColor Yellow }; .\httpserver.exe"

Start-Process powershell -ArgumentList "-NoExit", "-Command", $serverCommand -WindowStyle Normal

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "Сервер перезапущен!" -ForegroundColor Green
Write-Host ""
Write-Host "Улучшения классификации применены:" -ForegroundColor Cyan
Write-Host "  ✓ Улучшенные промпты с правилами товар/услуга" -ForegroundColor Green
Write-Host "  ✓ Улучшенный keyword-классификатор" -ForegroundColor Green
Write-Host "  ✓ Пост-обработка для исправления ошибок" -ForegroundColor Green
Write-Host "  ✓ Улучшенные промпты AI-классификатора" -ForegroundColor Green
Write-Host ""
Write-Host "Теперь можно запустить классификацию КПВЭД" -ForegroundColor Yellow
Write-Host "через интерфейс управления процессами" -ForegroundColor Yellow
Write-Host "========================================" -ForegroundColor Green
Write-Host ""

Read-Host "Нажмите Enter для выхода"

