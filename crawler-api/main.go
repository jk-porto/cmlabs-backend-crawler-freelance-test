package main

import (
	"log"
	"net/url"
	"os"
	"time"

	"crawler-api/config"
	"crawler-api/crawler"

	"github.com/gofiber/fiber/v2"
)

func main() {
	cfg := config.Default()

	// Ensure the output directory exists at startup (not per-request).
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		log.Fatalf("failed to create output directory %q: %v", cfg.OutputDir, err)
	}

	app := newApp(cfg)

	log.Printf("Starting %s on %s", "Crawler API", cfg.ServerAddr)
	log.Fatal(app.Listen(cfg.ServerAddr))
}

// newApp builds and returns the GoFiber application. Extracted for testability.
func newApp(cfg config.Config) *fiber.App {
	app := fiber.New(fiber.Config{
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		AppName:      "Crawler API",
	})

	// Simple request logger middleware.
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		log.Printf("%s %s → %d (%s)", c.Method(), c.Path(), c.Response().StatusCode(), time.Since(start))
		return err
	})

	crawlerOpts := crawler.Options{
		PageTimeout:    cfg.PageTimeout,
		RenderSleep:    cfg.RenderSleep,
		MaxFilenameLen: cfg.MaxFilenameLen,
		MaxConcurrency: cfg.MaxConcurrency,
	}

	app.Get("/health", healthHandler)
	app.Post("/crawl", makeCrawlHandler(cfg.OutputDir, cfg.MaxDepth, crawlerOpts))

	return app
}

// ── Handlers ─────────────────────────────────────────────────────────────────

func healthHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

// makeCrawlHandler is a factory that closes over config values, keeping the
// handler signature compatible with fiber.Handler.
func makeCrawlHandler(outputDir string, maxDepth int, opts crawler.Options) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req CrawlRequest
		if err := c.BodyParser(&req); err != nil {
			log.Printf("crawlHandler: malformed body: %v", err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "malformed request body"})
		}

		if req.URL == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "url is required"})
		}

		parsed, err := url.ParseRequestURI(req.URL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "url must be a valid http or https URL"})
		}

		// Default to 1 if missing or zero; cap at maxDepth.
		if req.Depth <= 0 {
			req.Depth = 1
		}
		if req.Depth > maxDepth {
			req.Depth = maxDepth
		}

		savedFiles, err := crawler.Crawl(req.URL, req.Depth, outputDir, opts)
		if err != nil {
			log.Printf("crawlHandler: crawl failed: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(CrawlResponse{
			SavedFiles: savedFiles,
			Count:      len(savedFiles),
			BaseURL:    req.URL,
		})
	}
}

// ── Request / Response types ─────────────────────────────────────────────────

// CrawlRequest is the JSON body accepted by POST /crawl.
type CrawlRequest struct {
	URL   string `json:"url"`
	Depth int    `json:"depth"`
}

// CrawlResponse is the JSON body returned by a successful POST /crawl.
type CrawlResponse struct {
	SavedFiles []string `json:"saved_files"`
	Count      int      `json:"count"`
	BaseURL    string   `json:"base_url"`
}
