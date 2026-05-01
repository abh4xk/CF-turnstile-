@echo off
echo ⚡ CF Turnstile Solver - One-Click Setup
echo.

REM Check if Go is installed
go version >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ Go is not installed. Please install Go from https://golang.org/dl/
    pause
    exit /b 1
)

echo ✅ Go is installed

REM Install Redis if not present
if not exist "C:\Program Files\Redis\redis-server.exe" (
    echo 📦 Installing Redis...
    winget install Redis.Redis
    if %errorlevel% neq 0 (
        echo ❌ Redis installation failed. Please install Redis manually.
        pause
        exit /b 1
    )
    echo ✅ Redis installed
) else (
    echo ✅ Redis already installed
)

REM Install Go dependencies
echo 📦 Installing Go dependencies...
go mod tidy
if %errorlevel% neq 0 (
    echo ❌ Failed to install Go dependencies
    pause
    exit /b 1
)
echo ✅ Go dependencies installed

REM Install Playwright browsers
echo 📦 Installing Playwright browsers...
go run github.com/playwright-community/playwright-go/cmd/playwright install
if %errorlevel% neq 0 (
    echo ❌ Failed to install Playwright browsers
    pause
    exit /b 1
)
echo ✅ Playwright browsers installed

REM Start Redis server
echo 🗄️ Starting Redis server on IPv4...
start /B "C:\Program Files\Redis\redis-server.exe" --bind 127.0.0.1 --port 6379
timeout /t 3 >nul

REM Run the application
echo 🚀 Starting CF Turnstile Solver...
echo.
echo Server will be available at: http://localhost:5073
echo Press Ctrl+C to stop the server
echo.
go run "turnstile solver.go"

pause
