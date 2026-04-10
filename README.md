# Crawler API

A high-performance Go REST API that crawls web applications (SPA, SSR, PWA) using headless Chrome. It captures fully-rendered HTML and saves it to disk.

## Features
- **Headless Chrome**: Uses `chromedp` to render JavaScript.
- **Concurrent Crawling**: Pages are crawled in parallel with a configurable concurrency limit (semaphore).
- **Graceful Failures**: Individual page errors don't stop the entire crawl.
- **Configurable**: Timeouts, depth, and concurrency are easily adjustable.

## Getting Started

### Prerequisites
- Go 1.21+
- Chrome/Chromium installed

### Setup
```bash
go mod tidy
```

### Run
```bash
go run main.go
```
The API will be available at `http://localhost:8080`.

## API Endpoints

### 1. Health Check
`GET /health`
- Returns `{"status": "ok"}`

### 2. Crawl
`POST /crawl`
- **Body**: `{"url": "https://example.com", "depth": 1}`
- **Parameters**:
  - `url` (required): The starting URL.
  - `depth` (optional): Max recursion depth (1-3, default 1).

**Example Request:**
```bash
curl -X POST http://localhost:8080/crawl \
  -H "Content-Type: application/json" \
  -d '{"url":"https://cmlabs.co/en-id","depth":1}'
```

## Configuration
Settings can be modified in `config/config.go` via the `Default()` function:
- `MaxConcurrency`: Max simultaneous browser tabs (default: 5).
- `PageTimeout`: Timeout per page (default: 30s).
- `RenderSleep`: Extra wait for JS frameworks (default: 2s).
- `OutputDir`: Where HTML files are saved (default: `./output`).

## Testing
```bash
go test ./...
```
