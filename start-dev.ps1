# PowerShell скрипт для запуска серверов в режиме разработки с автоматической пересборкой
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Запуск системы в режиме разработки" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Проверяем наличие Go
$goExists = Get-Command go -ErrorAction SilentlyContinue
if (-not $goExists) {
    Write-Host "[ОШИБКА] Go не найден в PATH" -ForegroundColor Red
    Write-Host "Установите Go и добавьте его в PATH"
    Read-Host "Нажмите Enter для выхода"
    exit 1
}

# Проверяем наличие Node.js
$npmExists = Get-Command npm -ErrorAction SilentlyContinue
if (-not $npmExists) {
    Write-Host "[ОШИБКА] Node.js не найден в PATH" -ForegroundColor Red
    Write-Host "Установите Node.js и добавьте его в PATH"
    Read-Host "Нажмите Enter для выхода"
    exit 1
}

# Переходим в директорию проекта
Set-Location $PSScriptRoot

# Проверяем наличие папки frontend
$frontendDir = Join-Path $PSScriptRoot "frontend"
if (-not (Test-Path $frontendDir)) {
    Write-Host "[ОШИБКА] Папка frontend не найдена: $frontendDir" -ForegroundColor Red
    Read-Host "Нажмите Enter для выхода"
    exit 1
}

# Проверяем наличие package.json в frontend
$packageJson = Join-Path $frontendDir "package.json"
if (-not (Test-Path $packageJson)) {
    Write-Host "[ОШИБКА] package.json не найден в папке frontend: $packageJson" -ForegroundColor Red
    Read-Host "Нажмите Enter для выхода"
    exit 1
}

# Проверяем наличие .air.toml
$airConfig = Join-Path $PSScriptRoot ".air.toml"
if (-not (Test-Path $airConfig)) {
    Write-Host "[ОШИБКА] Конфигурация .air.toml не найдена" -ForegroundColor Red
    Write-Host "Убедитесь, что файл .air.toml существует в корне проекта"
    Read-Host "Нажмите Enter для выхода"
    exit 1
}

# Сохраняем полный путь для использования в новых процессах
$projectPath = (Resolve-Path $PSScriptRoot).Path
$frontendPath = (Resolve-Path $frontendDir).Path

# Создаем директорию tmp для Air, если её нет
$tmpDir = Join-Path $projectPath "tmp"
if (-not (Test-Path $tmpDir)) {
    New-Item -ItemType Directory -Path $tmpDir | Out-Null
    Write-Host "[INFO] Создана директория tmp для Air" -ForegroundColor Green
}

# Ищем air в стандартных местах
$airPath = $null
$possiblePaths = @(
    "$env:USERPROFILE\go\bin\air.exe",
    "$env:LOCALAPPDATA\go\bin\air.exe",
    "$env:GOPATH\bin\air.exe",
    (Get-Command air -ErrorAction SilentlyContinue).Source
)

foreach ($path in $possiblePaths) {
    if ($path -and (Test-Path $path)) {
        $airPath = $path
        break
    }
}

# Если air не найден, пытаемся использовать go run
if (-not $airPath) {
    Write-Host "[INFO] Air не найден в PATH, будет использован go run" -ForegroundColor Yellow
    $airCommand = "cd '$projectPath'; go run github.com/air-verse/air@latest"
} else {
    Write-Host "[INFO] Найден Air: $airPath" -ForegroundColor Green
    $airCommand = "cd '$projectPath'; & '$airPath'"
}

Write-Host "[1/2] Запуск бэкенда с Air (автоматическая пересборка)..." -ForegroundColor Yellow
Write-Host "     Air будет отслеживать изменения в .go файлах и автоматически пересобирать приложение" -ForegroundColor Cyan
Start-Process powershell -ArgumentList "-NoExit", "-Command", $airCommand -WindowStyle Normal

Start-Sleep -Seconds 3

Write-Host "[2/2] Запуск фронтенда на порту 3000..." -ForegroundColor Yellow
$frontendCommand = "cd '$frontendPath'; npm run dev"
Start-Process powershell -ArgumentList "-NoExit", "-Command", $frontendCommand -WindowStyle Normal

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "Серверы запущены в режиме разработки!" -ForegroundColor Green
Write-Host ""
Write-Host "Бэкенд:  http://localhost:9999" -ForegroundColor Cyan
Write-Host "         (Air автоматически пересоберет при изменении .go файлов)" -ForegroundColor Gray
Write-Host "Фронтенд: http://localhost:3000" -ForegroundColor Cyan
Write-Host "          (Next.js имеет встроенный hot reload)" -ForegroundColor Gray
Write-Host ""
Write-Host "Для остановки закройте окна серверов" -ForegroundColor Yellow
Write-Host "========================================" -ForegroundColor Green
Write-Host ""

Read-Host "Нажмите Enter для выхода"

