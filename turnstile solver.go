package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/playwright-community/playwright-go"
	"github.com/redis/go-redis/v9"
)

// Configuration - MATCHES PYTHON
var CONFIG = struct {
	NumBrowsers         int
	TabsPerBrowser      int
	MaxRetries          int
	Timeout             int
	MaxQueueSize        int
	RedisHost           string
	RedisPort           int
	RedisDB             int
	ResultTTL           time.Duration
	BrowserRecycleAfter int
}{
	NumBrowsers:         2,
	TabsPerBrowser:      10,
	MaxRetries:          1,
	Timeout:             30,
	MaxQueueSize:        3000,
	RedisHost:           "localhost",
	RedisPort:           6379,
	RedisDB:             0,
	ResultTTL:           1800 * time.Second,
	BrowserRecycleAfter: 100,
}

type SolveTask struct {
	TaskID      string   `json:"task_id"`
	URL         string   `json:"url"`
	Sitekey     string   `json:"sitekey"`
	Proxy       *string  `json:"proxy"`
	Action      *string  `json:"action"`
	Cdata       *string  `json:"cdata"`
	Status      string   `json:"status"`
	Token       *string  `json:"token"`
	Error       *string  `json:"error"`
	Attempts    int      `json:"attempts"`
	CreatedAt   float64  `json:"created_at"`
	CompletedAt *float64 `json:"completed_at"`
}

type BrowserWorker struct {
	WorkerID   int
	BrowserID  int
	Browser    playwright.Browser
	Context    playwright.BrowserContext
	Page       playwright.Page
	IsRunning  bool
	SolveCount int
	TabReady   bool
	sync.Mutex
}

var (
	taskQueue   chan *SolveTask
	redisClient *redis.Client
	stats       = struct {
		Total   int
		Success int
		Failed  int
		sync.RWMutex
	}{}
	browserPool   []*BrowserWorker
	inMemoryTasks sync.Map
	pw            *playwright.Playwright
)

func initRedis() {
	redisClient = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", CONFIG.RedisHost, CONFIG.RedisPort),
		DB:   CONFIG.RedisDB,
	})
	log.Println("✅ Redis connected")
}

func (w *BrowserWorker) initialize() error {
	contextOptions := playwright.BrowserNewContextOptions{
		UserAgent: playwright.String("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"),
		ExtraHttpHeaders: map[string]string{
			"sec-ch-ua": `"Google Chrome";v="137", "Chromium";v="137", "Not/A)Brand";v="24"`,
		},
		Viewport: &playwright.Size{Width: 200, Height: 40},
	}

	var err error
	w.Context, err = w.Browser.NewContext(contextOptions)
	if err != nil {
		return err
	}

	w.Page, err = w.Context.NewPage()
	if err != nil {
		return err
	}

	w.Page.Goto("about:blank")
	w.TabReady = true
	return nil
}

func (w *BrowserWorker) recreateTab() {
	w.Lock()
	defer w.Unlock()

	if w.Page != nil {
		w.Page.Close()
		w.Page = nil
	}

	// Create new page
	var err error
	w.Page, err = w.Context.NewPage()
	if err != nil {
		log.Printf("Failed to recreate page for worker %d: %v", w.WorkerID, err)
		return
	}

	w.Page.Goto("about:blank")
	w.TabReady = true
}

func (w *BrowserWorker) recycle() {
	w.Lock()
	defer w.Unlock()

	if w.Page != nil {
		w.Page.Close()
	}
	if w.Context != nil {
		w.Context.Close()
	}

	w.initialize()
	w.SolveCount = 0
}

func (w *BrowserWorker) cleanup() {
	w.Lock()
	defer w.Unlock()

	if w.Page != nil {
		w.Page.Close()
	}
	if w.Context != nil {
		w.Context.Close()
	}
}

func (w *BrowserWorker) loadCaptchaOverlay(sitekey string, action string, cdata string) error {
	actionScript := ""
	if action != "" {
		actionScript = fmt.Sprintf("captchaDiv.setAttribute('data-action', '%s');", action)
	}

	cdataScript := ""
	if cdata != "" {
		cdataScript = fmt.Sprintf("captchaDiv.setAttribute('data-cdata', '%s');", cdata)
	}

	script := fmt.Sprintf(`
		if (!document.querySelector('#captcha-overlay')) {
			const overlay = document.createElement('div');
			overlay.id = 'captcha-overlay';
			overlay.style.cssText = 'position:fixed;top:0;left:0;width:100vw;height:100vh;background:#000;display:flex;justify-content:center;align-items:center;z-index:999999';
			const captchaDiv = document.createElement('div');
			captchaDiv.className = 'cf-turnstile';
			captchaDiv.setAttribute('data-sitekey', '%s');
			%s
			%s
			overlay.appendChild(captchaDiv);
			document.body.appendChild(overlay);
			if (!window.turnstileScriptLoaded) {
				const script = document.createElement('script');
				script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js';
				script.async = true;
				document.head.appendChild(script);
				window.turnstileScriptLoaded = true;
			}
		}
	`, sitekey, actionScript, cdataScript)

	_, err := w.Page.Evaluate(script)
	return err
}

func (w *BrowserWorker) originalClickStrategies() {
	strategies := []string{
		".cf-turnstile",
		"iframe[src*=\"turnstile\"]",
		"[data-sitekey]",
	}

	for _, selector := range strategies {
		w.Page.Click(selector, playwright.PageClickOptions{Timeout: playwright.Float(1000)})
	}
	w.Page.Evaluate("document.querySelector('.cf-turnstile')?.click()")
}

func (w *BrowserWorker) solveTurnstile(task *SolveTask) bool {
	w.Lock()
	defer w.Unlock()

	if !w.TabReady || w.Page == nil {
		errMsg := "Tab not ready"
		task.Status = "failed"
		task.Error = &errMsg
		return false
	}

	// Navigate to target URL
	_, err := w.Page.Goto(task.URL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(15000),
	})

	if err != nil {
		errMsg := fmt.Sprintf("Navigation error: %v", err)
		task.Status = "failed"
		task.Error = &errMsg
		w.TabReady = false
		return false
	}

	// Load overlay immediately
	action := ""
	if task.Action != nil {
		action = *task.Action
	}
	cdata := ""
	if task.Cdata != nil {
		cdata = *task.Cdata
	}
	w.loadCaptchaOverlay(task.Sitekey, action, cdata)

	// Optimized polling - check every 5ms for faster detection
	for attempt := 0; attempt < 600; attempt++ {
		tokenResult, _ := w.Page.Evaluate(`() => {
			const input = document.querySelector('input[name="cf-turnstile-response"]');
			return input?.value?.length > 100 ? input.value : null;
		}`)

		if tokenStr, ok := tokenResult.(string); ok && len(tokenStr) > 100 {
			task.Token = &tokenStr
			task.Status = "success"
			w.SolveCount++

			// Close tab after getting token
			w.Page.Close()
			w.Page = nil
			w.TabReady = false

			return true
		}

		// Click strategies every 50 attempts
		if attempt%50 == 10 {
			w.Page.Click(".cf-turnstile", playwright.PageClickOptions{Timeout: playwright.Float(500)})
		}

		time.Sleep(5 * time.Millisecond)
	}

	// Close tab on timeout
	w.Page.Close()
	w.Page = nil
	w.TabReady = false

	errMsg := "Timeout"
	task.Status = "failed"
	task.Error = &errMsg
	return false
}

func workerLoop(worker *BrowserWorker) {
	consecutiveFailures := 0

	for worker.IsRunning {
		select {
		case task := <-taskQueue:
			// Only process if we have a ready tab
			if !worker.TabReady {
				// Put task back in queue and recreate tab
				select {
				case taskQueue <- task:
				default:
				}
				worker.recreateTab()
				continue
			}

			task.Status = "solving"
			task.Attempts++

			success := worker.solveTurnstile(task)

			if success {
				stats.Lock()
				stats.Success++
				stats.Unlock()
				consecutiveFailures = 0
			} else {
				stats.Lock()
				stats.Failed++
				stats.Unlock()
				consecutiveFailures++

				if task.Attempts < CONFIG.MaxRetries {
					task.Status = "pending"
					select {
					case taskQueue <- task:
					default:
					}
					continue
				}
			}

			now := float64(time.Now().UnixNano()) / 1e9
			task.CompletedAt = &now
			saveTaskToRedis(task)

			// Recreate tab after each solve (whether success or failure)
			worker.recreateTab()

			if consecutiveFailures > 10 || worker.SolveCount >= CONFIG.BrowserRecycleAfter {
				worker.recycle()
				consecutiveFailures = 0
			}
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func saveTaskToRedis(task *SolveTask) {
	inMemoryTasks.Store(task.TaskID, task)
	data, _ := json.Marshal(task)
	redisClient.SetEx(context.Background(), "task:"+task.TaskID, string(data), CONFIG.ResultTTL)
}

func getTaskFromRedis(taskID string) *SolveTask {
	if val, ok := inMemoryTasks.Load(taskID); ok {
		return val.(*SolveTask)
	}

	val, err := redisClient.Get(context.Background(), "task:"+taskID).Result()
	if err != nil {
		return nil
	}

	var task SolveTask
	json.Unmarshal([]byte(val), &task)
	return &task
}

func initializeBrowserPool() error {
	var err error
	pw, err = playwright.Run()
	if err != nil {
		return fmt.Errorf("could not start playwright: %v", err)
	}

	totalTabs := CONFIG.NumBrowsers * CONFIG.TabsPerBrowser
	log.Printf("🚀 Initializing %d Chrome browser(s) with %d tabs each (%d total workers) - LOW CPU MODE...", CONFIG.NumBrowsers, CONFIG.TabsPerBrowser, totalTabs)

	workerID := 1
	for browserID := 1; browserID <= CONFIG.NumBrowsers; browserID++ {
		log.Printf("[Browser %d] Creating Chrome browser (LOW CPU)...", browserID)

		browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(false),
			Args: []string{
				"--disable-blink-features=AutomationControlled",
				"--no-sandbox",
				"--disable-setuid-sandbox",
				"--disable-dev-shm-usage",
				"--disable-extensions",
				"--disable-plugins",
				"--disable-gpu",
				"--disable-software-rasterizer",
				"--disable-background-networking",
				"--disable-background-timer-throttling",
				"--disable-backgrounding-occluded-windows",
				"--disable-breakpad",
				"--disable-component-extensions-with-background-pages",
				"--disable-features=TranslateUI,BlinkGenPropertyTrees",
				"--disable-ipc-flooding-protection",
				"--disable-renderer-backgrounding",
			},
		})

		if err != nil {
			return fmt.Errorf("could not launch browser: %v", err)
		}

		for tabNum := 0; tabNum < CONFIG.TabsPerBrowser; tabNum++ {
			worker := &BrowserWorker{
				WorkerID:  workerID,
				BrowserID: browserID,
				Browser:   browser,
				IsRunning: true,
			}
			err := worker.initialize()
			if err != nil {
				log.Printf("Failed to initialize worker %d: %v", workerID, err)
			}
			browserPool = append(browserPool, worker)
			workerID++
		}
	}

	log.Printf("✅ Chrome browser pool ready (LOW CPU): %d browser(s) × %d tabs = %d workers", CONFIG.NumBrowsers, CONFIG.TabsPerBrowser, len(browserPool))
	return nil
}

func startWorkers() {
	for _, worker := range browserPool {
		go workerLoop(worker)
	}
	log.Printf("✅ Started %d workers", len(browserPool))
}

func cleanupBrowserPool() {
	log.Println("🧹 Cleaning up Chrome browser pool...")
	for _, worker := range browserPool {
		worker.IsRunning = false
		worker.cleanup()
	}
	if pw != nil {
		pw.Stop()
	}
	log.Println("✅ Chrome browser pool cleaned up")
}

func turnstileHandler(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	sitekey := r.URL.Query().Get("sitekey")
	actionStr := r.URL.Query().Get("action")
	cdataStr := r.URL.Query().Get("cdata")

	w.Header().Set("Content-Type", "application/json")

	if url == "" || sitekey == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errorId":          1,
			"errorCode":        "ERROR_WRONG_PAGEURL",
			"errorDescription": "Both 'url' and 'sitekey' are required",
		})
		return
	}

	taskID := uuid.New().String()

	var action, cdata *string
	if actionStr != "" {
		action = &actionStr
	}
	if cdataStr != "" {
		cdata = &cdataStr
	}

	task := &SolveTask{
		TaskID:    taskID,
		URL:       url,
		Sitekey:   sitekey,
		Action:    action,
		Cdata:     cdata,
		Status:    "pending",
		CreatedAt: float64(time.Now().UnixNano()) / 1e9,
	}

	saveTaskToRedis(task)

	select {
	case taskQueue <- task:
		stats.Lock()
		stats.Total++
		stats.Unlock()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errorId": 0,
			"taskId":  taskID,
		})
	default:
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errorId":          1,
			"errorCode":        "ERROR_UNKNOWN",
			"errorDescription": "Task queue is full",
		})
	}
}

func resultHandler(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("id")
	w.Header().Set("Content-Type", "application/json")

	if taskID == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errorId":          1,
			"errorCode":        "ERROR_WRONG_CAPTCHA_ID",
			"errorDescription": "Invalid task ID/Request parameter",
		})
		return
	}

	task := getTaskFromRedis(taskID)
	if task == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errorId":          1,
			"errorCode":        "ERROR_CAPTCHA_UNSOLVABLE",
			"errorDescription": "Task not found",
		})
		return
	}

	if task.Status == "pending" || task.Status == "solving" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "processing",
		})
		return
	}

	if task.Status == "failed" || task.Token == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errorId":          1,
			"errorCode":        "ERROR_CAPTCHA_UNSOLVABLE",
			"errorDescription": "Workers could not solve the Captcha",
		})
		return
	}

	if task.Token != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errorId": 0,
			"status":  "ready",
			"solution": map[string]interface{}{
				"token": *task.Token,
			},
		})
		return
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "%s", `
		<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<title>⚡ LOW CPU Chrome Turnstile Solver API</title>
			<script src="https://cdn.tailwindcss.com"></script>
		</head>
		<body class="bg-gray-900 text-gray-200 min-h-screen flex items-center justify-center">
			<div class="bg-gray-800 p-8 rounded-lg shadow-md max-w-2xl w-full border border-green-500">
				<h1 class="text-3xl font-bold mb-6 text-center text-green-500">⚡ LOW CPU Chrome Turnstile Solver API</h1>

				<p class="mb-4 text-gray-300">To use the turnstile service, send a GET request to 
				   <code class="bg-green-700 text-white px-2 py-1 rounded">/turnstile</code> with the following query parameters:</p>

				<ul class="list-disc pl-6 mb-6 text-gray-300">
					<li><strong>url</strong>: The URL where Turnstile is to be validated</li>
					<li><strong>sitekey</strong>: The site key for Turnstile</li>
				</ul>

				<div class="bg-gray-700 p-4 rounded-lg mb-6 border border-green-500">
					<p class="font-semibold mb-2 text-green-400">Example usage:</p>
					<code class="text-sm break-all text-green-300">/turnstile?url=https://example.com&sitekey=sitekey</code>
				</div>

				<div class="bg-gray-700 p-4 rounded-lg mb-6">
					<p class="text-gray-200 font-semibold mb-3">⚡ LOW CPU Optimizations</p>
					<div class="space-y-2 text-sm">
						<p class="text-gray-300">
							🖥️ <strong>Mode:</strong> LOW CPU Chrome (blocked images/css/fonts)
						</p>
						<p class="text-gray-300">
							🚀 <strong>Port:</strong> 5073
						</p>
						<p class="text-gray-300">
							⚡ <strong>Performance:</strong> Fast polling + Go routines
						</p>
						<p class="text-gray-300">
							🎯 <strong>Language:</strong> Golang rewrite
						</p>
					</div>
				</div>
			</div>
		</body>
		</html>
	`)
}

func main() {
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("🎯 Chrome Turnstile Solver - GO VERSION API")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("⚙️  Browsers: %d\n", CONFIG.NumBrowsers)
	fmt.Printf("📑 Tabs per Browser: %d\n", CONFIG.TabsPerBrowser)
	fmt.Printf("👥 Total Workers: %d\n", CONFIG.NumBrowsers*CONFIG.TabsPerBrowser)
	fmt.Printf("🔄 Max Retries: %d\n", CONFIG.MaxRetries)
	fmt.Printf("⏱️  Timeout: %ds\n", CONFIG.Timeout)
	fmt.Printf("📊 Max Queue Size: %d\n", CONFIG.MaxQueueSize)
	fmt.Printf("🗄️  Redis: %s:%d\n", CONFIG.RedisHost, CONFIG.RedisPort)
	fmt.Println("🖥️  Mode: MAXIMUM SPEED (overlay first, no-delay polling, aggressive clicks)")
	fmt.Println(strings.Repeat("=", 70))

	taskQueue = make(chan *SolveTask, CONFIG.MaxQueueSize)

	initRedis()

	err := initializeBrowserPool()
	if err != nil {
		log.Fatalf("Fatal error initializing browser pool: %v", err)
	}
	defer cleanupBrowserPool()

	startWorkers()

	http.HandleFunc("/turnstile", turnstileHandler)
	http.HandleFunc("/result", resultHandler)
	http.HandleFunc("/", indexHandler)

	fmt.Printf("⚡ Starting LOW CPU server on http://0.0.0.0:5073\n")
	log.Fatal(http.ListenAndServe("0.0.0.0:5073", nil))
}
