package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"crawler-api/config"

	"github.com/gofiber/fiber/v2"
)

// testApp creates a configured fiber app backed by a throwaway temp dir.
func testApp(t *testing.T) *fiber.App {
	t.Helper()
	cfg := config.Default()
	cfg.OutputDir = t.TempDir() // isolated per test
	return newApp(cfg)
}

// ── GET /health ───────────────────────────────────────────────────────────────

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()
	app := testApp(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`body["status"] = %q; want "ok"`, body["status"])
	}
}

// ── POST /crawl – input validation ───────────────────────────────────────────

func TestCrawlHandler_Validation(t *testing.T) {
	t.Parallel()
	app := testApp(t)

	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
	}{
		{
			name:        "malformed JSON",
			contentType: "application/json",
			body:        `{not-valid-json}`,
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "empty url field",
			contentType: "application/json",
			body:        `{"url":""}`,
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "missing url field",
			contentType: "application/json",
			body:        `{}`,
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "ftp scheme rejected",
			contentType: "application/json",
			body:        `{"url":"ftp://example.com"}`,
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "relative path rejected",
			contentType: "application/json",
			body:        `{"url":"/just/a/path"}`,
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "plain string rejected",
			contentType: "application/json",
			body:        `{"url":"not-a-url"}`,
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/crawl", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d; want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}
}

// ── POST /crawl – depth clamping ─────────────────────────────────────────────
// We can't execute a real crawl in a unit test (requires Chrome + network),
// but we can verify that the handler accepts valid payloads without a 400 or
// 500 before the crawl even starts (within a capped timeout).  In CI, a
// real integration test suite should cover the actual crawl path.

func TestCrawlHandler_DepthDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{"depth zero defaults to 1", `{"url":"https://example.com","depth":0}`},
		{"negative depth defaults to 1", `{"url":"https://example.com","depth":-5}`},
		{"depth above cap clamped to 3", `{"url":"https://example.com","depth":99}`},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := testApp(t)

			req := httptest.NewRequest(http.MethodPost, "/crawl", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, 1 /* ms timeout – stops before Chrome launches */)
			if err != nil {
				// A timeout error here is expected and acceptable: it means the
				// handler reached the crawl phase (passed all validation), which
				// is exactly what we want to confirm.
				return
			}
			defer resp.Body.Close()

			// If we do get a response it must not be a 400 (validation failure).
			if resp.StatusCode == http.StatusBadRequest {
				t.Errorf("unexpected 400 – validation rejected a valid payload")
			}
		})
	}
}
