# Скрипт для публикации проекта на GitHub
# Использование: .\publish_to_github.ps1 -RepoName "HttpServer" -IsPublic $true

param(
    [string]$RepoName = "HttpServer",
    [bool]$IsPublic = $true,
    [string]$GitHubToken = $env:GITHUB_TOKEN
)

Write-Host "Публикация проекта на GitHub..." -ForegroundColor Green

# Проверка наличия токена
if (-not $GitHubToken) {
    Write-Host "`nGitHub токен не найден. Выполните следующие шаги:" -ForegroundColor Yellow
    Write-Host "1. Создайте Personal Access Token на https://github.com/settings/tokens" -ForegroundColor Cyan
    Write-Host "   (нужны права: repo)" -ForegroundColor Cyan
    Write-Host "2. Установите переменную окружения:" -ForegroundColor Cyan
    Write-Host "   `$env:GITHUB_TOKEN = 'ваш_токен'" -ForegroundColor Cyan
    Write-Host "3. Или передайте токен как параметр:" -ForegroundColor Cyan
    Write-Host "   .\publish_to_github.ps1 -GitHubToken 'ваш_токен'" -ForegroundColor Cyan
    Write-Host "`nИли создайте репозиторий вручную на https://github.com/new" -ForegroundColor Yellow
    Write-Host "Затем выполните:" -ForegroundColor Yellow
    Write-Host "  git remote add origin https://github.com/ВАШ_USERNAME/$RepoName.git" -ForegroundColor Cyan
    Write-Host "  git branch -M main" -ForegroundColor Cyan
    Write-Host "  git push -u origin main" -ForegroundColor Cyan
    exit 1
}

# Проверка наличия git
if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    Write-Host "Ошибка: Git не установлен!" -ForegroundColor Red
    exit 1
}

# Проверка наличия curl
if (-not (Get-Command curl -ErrorAction SilentlyContinue)) {
    Write-Host "Ошибка: curl не установлен!" -ForegroundColor Red
    exit 1
}

# Создание репозитория через GitHub API
Write-Host "`nСоздание репозитория на GitHub..." -ForegroundColor Green

$body = @{
    name = $RepoName
    description = "HTTP Server для приема данных из 1С и нормализации номенклатуры"
    private = -not $IsPublic
} | ConvertTo-Json

$headers = @{
    "Authorization" = "token $GitHubToken"
    "Accept" = "application/vnd.github.v3+json"
    "Content-Type" = "application/json"
}

try {
    $response = Invoke-RestMethod -Uri "https://api.github.com/user/repos" -Method Post -Headers $headers -Body $body
    
    Write-Host "Репозиторий создан: $($response.html_url)" -ForegroundColor Green
    
    # Добавление remote
    $remoteUrl = $response.clone_url
    Write-Host "`nДобавление remote origin..." -ForegroundColor Green
    
    # Проверка существования remote
    $existingRemote = git remote get-url origin 2>$null
    if ($existingRemote) {
        Write-Host "Remote origin уже существует: $existingRemote" -ForegroundColor Yellow
        $overwrite = Read-Host "Перезаписать? (y/n)"
        if ($overwrite -eq "y") {
            git remote set-url origin $remoteUrl
        } else {
            Write-Host "Используется существующий remote" -ForegroundColor Yellow
            $remoteUrl = $existingRemote
        }
    } else {
        git remote add origin $remoteUrl
    }
    
    # Переименование ветки в main (если нужно)
    $currentBranch = git branch --show-current
    if ($currentBranch -ne "main") {
        Write-Host "Переименование ветки в main..." -ForegroundColor Green
        git branch -M main
    }
    
    # Push
    Write-Host "`nОтправка кода на GitHub..." -ForegroundColor Green
    git push -u origin main
    
    Write-Host "`nПроект успешно опубликован!" -ForegroundColor Green
    Write-Host "URL: $($response.html_url)" -ForegroundColor Cyan
    
} catch {
    Write-Host "`nОшибка при создании репозитория:" -ForegroundColor Red
    Write-Host $_.Exception.Message -ForegroundColor Red
    
    if ($_.Exception.Response.StatusCode -eq 422) {
        Write-Host "`nРепозиторий с таким именем уже существует." -ForegroundColor Yellow
        Write-Host "Используйте другое имя или удалите существующий репозиторий." -ForegroundColor Yellow
    } elseif ($_.Exception.Response.StatusCode -eq 401) {
        Write-Host "`nНеверный токен доступа. Проверьте токен и повторите попытку." -ForegroundColor Yellow
    }
    
    Write-Host "`nСоздайте репозиторий вручную на https://github.com/new" -ForegroundColor Yellow
    Write-Host "Затем выполните:" -ForegroundColor Yellow
    Write-Host "  git remote add origin https://github.com/ВАШ_USERNAME/$RepoName.git" -ForegroundColor Cyan
    Write-Host "  git branch -M main" -ForegroundColor Cyan
    Write-Host "  git push -u origin main" -ForegroundColor Cyan
}




