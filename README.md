# ⚡ CF Turnstile Solver

A high-performance Go-based Cloudflare Turnstile CAPTCHA solver with intelligent tab management.

## 🚀 Quick Start

### Single Command Setup & Run

```bash
git clone https://github.com/abh4xk/CF-turnstile-.git && cd CF-turnstile- && make run
```

That's it! The application will automatically:
- Install all dependencies
- Start Redis server
- Initialize browser pool
- Launch the API server

## 📋 Prerequisites

- **Go 1.19+** - [Download Go](https://golang.org/dl/)
- **Windows** (optimized for Windows, works on other platforms too)

## 🛠️ Manual Setup

If you prefer manual setup:

```bash
# 1. Clone the repository
git clone https://github.com/abh4xk/CF-turnstile-.git
cd CF-turnstile-

# 2. Install dependencies
make deps

# 3. Start Redis
make redis

# 4. Run the application
make run
```

## 📖 Usage

### Professional API Endpoints

#### Create Task (POST)
```bash
curl -X POST http://localhost:5073/createTask \
  -H "Content-Type: application/json" \
  -d '{
    "clientKey": "YOUR_API_KEY",
    "task": {
      "type": "TurnstileTask",
      "websiteURL": "https://example.com",
      "websiteKey": "0x4AAAAAAABkUYJ2ABcZqyJ",
      "action": "login",
      "cdata": "optional_data"
    }
  }'
```

**Response:**
```json
{
  "errorId": 0,
  "taskId": 407533072
}
```

#### Get Task Result (POST)
```bash
curl -X POST http://localhost:5073/getTaskResult \
  -H "Content-Type: application/json" \
  -d '{
    "clientKey": "YOUR_API_KEY",
    "taskId": 407533072
  }'
```

**Response (Processing):**
```json
{
  "errorId": 0,
  "status": "processing"
}
```

**Response (Ready):**
```json
{
  "errorId": 0,
  "status": "ready",
  "solution": {
    "token": "1.0x4AAA..."
  }
}
```

### Legacy Endpoints (Still Supported)
- `GET /turnstile?url=URL&sitekey=KEY`
- `GET /result?id=TASK_ID`

### API Documentation
Visit `http://localhost:5073` for interactive API documentation.

## ⚙️ Configuration

The application uses the following default configuration:

| Setting | Value | Description |
|---------|-------|-------------|
| Browsers | 2 | Number of Chrome browsers |
| Tabs per Browser | 10 | Tabs per browser (total workers = 20) |
| Max Retries | 1 | Maximum retry attempts |
| Timeout | 30s | Solve timeout |
| Redis | localhost:6379 | Redis server |
| Port | 5073 | API server port |

## 🎯 Features

- **Intelligent Tab Management**: Each task uses pre-existing tabs
- **Automatic Tab Closure**: Tabs close immediately after token generation
- **Tab Recreation**: Automatic tab recreation to maintain worker pool
- **Redis Integration**: Efficient task management and result caching
- **Low CPU Optimization**: Optimized Chrome settings for minimal resource usage
- **High Performance**: 20 concurrent workers for fast solving
- **REST API**: Clean RESTful API for easy integration

## 🔧 Available Commands

```bash
make help          # Show all available commands
make deps          # Install Go dependencies
make redis         # Start Redis server
make run           # Run the application
make build         # Build the application
make clean         # Clean build artifacts
make test          # Run tests
```

## 📁 Project Structure

```
CF-turnstile-/
├── turnstile solver.go    # Main application
├── go.mod                # Go module file
├── go.sum                # Dependency checksums
├── Makefile              # Build automation
├── setup.bat             # Windows setup script
└── README.md             # This file
```

## 🔍 Tab Management Behavior

1. **Initialization**: Creates 20 ready tabs (2 browsers × 10 tabs)
2. **Task Processing**: Each task uses an existing ready tab
3. **Token Generation**: Tab closes immediately after providing token
4. **Tab Recreation**: New tab created to maintain pool size
5. **Resource Efficiency**: No hanging tabs or memory leaks

## 📊 Performance

- **Concurrent Workers**: 20 parallel solving instances
- **Response Time**: ~3-10 seconds per solve
- **Success Rate**: High success rate with retry mechanism
- **Resource Usage**: Optimized for low CPU and memory usage

## 🐛 Troubleshooting

### Port Already in Use
```bash
netstat -ano | findstr :5073
taskkill /PID <PID> /F
```

### Redis Connection Issues
```bash
make redis-stop
make redis
```

### Browser Initialization Issues
```bash
make clean
make deps
make run
```

## 📝 API Response Examples

### Submit Task Response
```json
{
  "errorId": 0,
  "taskId": "uuid-here"
}
```

### Success Response
```json
{
  "errorId": 0,
  "status": "ready",
  "solution": {
    "token": "turnstile-token-here"
  }
}
```

### Processing Response
```json
{
  "status": "processing"
}
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## 📄 License

This project is private and proprietary.

## 🆘 Support

For issues and support, please create an issue in the repository.

---

**⚡ Built with Go, Playwright, and Redis for maximum performance!**
