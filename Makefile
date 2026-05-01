.PHONY: help deps redis redis-stop run build clean test install

# Default target
help:
	@echo "⚡ CF Turnstile Solver - Available Commands:"
	@echo ""
	@echo "make help          - Show this help message"
	@echo "make deps          - Install Go dependencies"
	@echo "make redis         - Start Redis server"
	@echo "make redis-stop    - Stop Redis server"
	@echo "make run           - Run the application"
	@echo "make build         - Build the application"
	@echo "make clean         - Clean build artifacts"
	@echo "make test          - Run tests"
	@echo "make install       - Install system dependencies (Redis, Playwright)"
	@echo ""
	@echo "🚀 Quick Start: git clone https://github.com/abh4xk/CF-turnstile-.git && cd CF-turnstile- && make run"

# Install Go dependencies
deps:
	@echo "📦 Installing Go dependencies..."
	go mod tidy
	go mod download
	@echo "✅ Go dependencies installed"

# Install system dependencies
install:
	@echo "🔧 Installing system dependencies..."
	@echo "Installing Redis..."
	winget install Redis.Redis || echo "Redis already installed or installation failed"
	@echo "Installing Playwright browsers..."
	go run github.com/playwright-community/playwright-go/cmd/playwright install
	@echo "✅ System dependencies installed"

# Start Redis server
redis:
	@echo "🗄️ Starting Redis server..."
	@if not exist "C:\Program Files\Redis\redis-server.exe" (echo "Redis not found. Run 'make install' first." && exit 1)
	@echo "Starting Redis on IPv4 127.0.0.1:6379..."
	start /B "C:\Program Files\Redis\redis-server.exe" --bind 127.0.0.1 --port 6379
	@timeout /t 3 >nul
	@echo "✅ Redis server started on 127.0.0.1:6379"

# Stop Redis server
redis-stop:
	@echo "🛑 Stopping Redis server..."
	taskkill /F /IM redis-server.exe 2>nul || echo "Redis server not running"
	@echo "✅ Redis server stopped"

# Run the application
run: deps redis
	@echo "🚀 Starting CF Turnstile Solver..."
	go run "turnstile solver.go"

# Build the application
build: deps
	@echo "🔨 Building application..."
	go build -o turnstile-solver.exe "turnstile solver.go"
	@echo "✅ Build complete: turnstile-solver.exe"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	go clean
	@if exist turnstile-solver.exe del turnstile-solver.exe
	@echo "✅ Clean complete"

# Run tests
test:
	@echo "🧪 Running tests..."
	go test -v ./...

# One-command setup and run
setup-and-run: install run

# Check if all requirements are met
check:
	@echo "🔍 Checking requirements..."
	@go version >nul 2>&1 && echo "✅ Go is installed" || echo "❌ Go is not installed"
	@if exist "C:\Program Files\Redis\redis-server.exe" (echo "✅ Redis is installed") else (echo "❌ Redis is not installed")
	@echo "✅ Check complete"
