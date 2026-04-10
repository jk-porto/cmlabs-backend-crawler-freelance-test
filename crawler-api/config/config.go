// Package config defines all tuneable parameters for the Crawler API.
package config

import "time"

// Config holds every tuneable parameter for the server and crawler.
// Populate with Default() and override individual fields as needed.
type Config struct {
	// ── Server ───────────────────────────────────────────────────────────────

	// ServerAddr is the TCP address the HTTP server listens on (e.g. ":8080").
	ServerAddr string

	// ReadTimeout is the maximum duration for reading an entire HTTP request.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration for writing an HTTP response.
	// Keep this high enough to accommodate long crawl operations.
	WriteTimeout time.Duration

	// OutputDir is the directory where saved HTML files are written.
	OutputDir string

	// ── Crawler ──────────────────────────────────────────────────────────────

	// PageTimeout is the maximum time allowed for a single page navigation
	// (Navigate + WaitReady + RenderSleep + HTML capture).
	PageTimeout time.Duration

	// RenderSleep is the extra time to wait after the <body> is ready, giving
	// JS frameworks (React, Vue, Angular …) time to finish rendering.
	RenderSleep time.Duration

	// MaxDepth is the hard upper limit on crawl recursion depth accepted from
	// callers. Requests that exceed this value are silently capped.
	MaxDepth int

	// MaxFilenameLen is the maximum number of characters in a saved HTML
	// filename (excluding the ".html" extension).
	MaxFilenameLen int

	// MaxConcurrency is the maximum number of browser tabs that may be open
	// simultaneously during a crawl. 0 means unlimited.
	MaxConcurrency int
}

// Default returns a production-ready Config with conservative, sensible
// values. Override individual fields after calling Default().
func Default() Config {
	return Config{
		ServerAddr:     ":8080",
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   120 * time.Second,
		OutputDir:      "./output",
		PageTimeout:    30 * time.Second,
		RenderSleep:    2 * time.Second,
		MaxDepth:       3,
		MaxFilenameLen: 150,
		MaxConcurrency: 5,
	}
}
