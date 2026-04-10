// Package crawler provides headless-browser crawling via chromedp.
package crawler

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// Options configures the behaviour of a single Crawl call.
type Options struct {
	// PageTimeout is the maximum time allowed for navigating one page
	// (including WaitReady + RenderSleep + HTML capture).
	PageTimeout time.Duration

	// RenderSleep is the extra idle time inserted after <body> is ready to
	// let JS frameworks finish hydrating the DOM before capturing HTML.
	RenderSleep time.Duration

	// MaxFilenameLen caps the length of the saved HTML filename (excluding the
	// ".html" extension). Pass 0 to disable truncation.
	MaxFilenameLen int

	// MaxConcurrency is the maximum number of browser tabs (pages) that may
	// be navigated simultaneously. Pass 0 for unlimited concurrency.
	MaxConcurrency int
}

// DefaultOptions returns production-ready Options that match config.Default.
func DefaultOptions() Options {
	return Options{
		PageTimeout:    30 * time.Second,
		RenderSleep:    2 * time.Second,
		MaxFilenameLen: 150,
		MaxConcurrency: 5,
	}
}

// crawlState holds all mutable state shared across concurrent goroutines.
type crawlState struct {
	opts      Options
	outputDir string

	// visited tracks URLs that have been (or are being) fetched. sync.Map
	// performs atomic LoadOrStore without a global lock.
	visited sync.Map

	// mu protects results and firstErr.
	mu       sync.Mutex
	results  []string
	firstErr error

	// sem is a counting semaphore that limits how many browser tabs may be
	// open simultaneously. nil means unlimited.
	sem chan struct{}
}

// newCrawlState constructs a crawlState from the given options.
func newCrawlState(opts Options, outputDir string) *crawlState {
	s := &crawlState{opts: opts, outputDir: outputDir}
	if opts.MaxConcurrency > 0 {
		s.sem = make(chan struct{}, opts.MaxConcurrency)
	}
	return s
}

// Crawl launches a headless browser, navigates to targetURL up to `depth`
// levels of recursion, saves the fully-rendered HTML of every unique page into
// outputDir, and returns the list of saved file paths.
//
// Pages at the same depth level are crawled concurrently, bounded by
// Options.MaxConcurrency. Each page gets its own browser tab.
func Crawl(targetURL string, depth int, outputDir string, opts Options) ([]string, error) {
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	defer cancelAlloc()

	// browserCtx represents the browser process itself (not a tab).
	// Individual tabs are created below via chromedp.NewContext(browserCtx).
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	s := newCrawlState(opts, outputDir)
	s.crawlPage(browserCtx, targetURL, depth)

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.results, s.firstErr
}

// crawlPage visits pageURL, saves its HTML, then fans out goroutines for each
// internal link discovered on the page. It returns only after all descendant
// goroutines have finished.
func (s *crawlState) crawlPage(browserCtx context.Context, pageURL string, depth int) {
	if depth < 0 {
		return
	}
	// LoadOrStore is atomic: only the goroutine that stores wins the race.
	if _, alreadyVisited := s.visited.LoadOrStore(pageURL, true); alreadyVisited {
		return
	}
	// Stop recursing as soon as the first fatal error is recorded.
	if s.hasError() {
		return
	}

	links := s.fetchAndSave(browserCtx, pageURL)

	if depth == 0 || len(links) == 0 {
		return
	}

	// Fan out: crawl all discovered links concurrently.
	var wg sync.WaitGroup
	for _, link := range links {
		link := link // capture loop variable
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.crawlPage(browserCtx, link, depth-1)
		}()
	}
	wg.Wait()
}

// fetchAndSave opens a new browser tab, navigates to pageURL, captures the
// fully-rendered HTML, writes it to disk, and returns extracted internal links.
// It acquires/releases the semaphore so that browser tab count stays bounded.
func (s *crawlState) fetchAndSave(browserCtx context.Context, pageURL string) []string {
	s.acquire()
	defer s.release()

	// Each page gets its own browser tab (chromedp target).
	tabCtx, cancelTab := chromedp.NewContext(browserCtx)
	defer cancelTab()

	pageCtx, cancelPage := context.WithTimeout(tabCtx, s.opts.PageTimeout)
	defer cancelPage()

	var html string
	var links []string

	err := chromedp.Run(pageCtx,
		chromedp.Navigate(pageURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(s.opts.RenderSleep),
		chromedp.OuterHTML("html", &html),
		chromedp.Evaluate(
			`Array.from(document.querySelectorAll('a[href]'))
				.map(a => a.href)
				.filter(h => h.startsWith(window.location.origin))`,
			&links,
		),
	)
	if err != nil {
		// Non-fatal: log and skip — one bad page should not abort the crawl.
		log.Printf("fetchAndSave: chromedp.Run failed for %s: %v", pageURL, err)
		return nil
	}

	filename := URLToFilename(pageURL, s.opts.MaxFilenameLen) + ".html"
	filePath := filepath.Join(s.outputDir, filename)
	if err := os.WriteFile(filePath, []byte(html), 0644); err != nil {
		log.Printf("fetchAndSave: write failed for %s: %v", pageURL, err)
		s.mu.Lock()
		if s.firstErr == nil {
			s.firstErr = fmt.Errorf("fetchAndSave: failed to write %s: %w", filePath, err)
		}
		s.mu.Unlock()
		return nil
	}

	s.mu.Lock()
	s.results = append(s.results, filePath)
	s.mu.Unlock()

	return links
}

// acquire blocks until a semaphore slot is available (or immediately if
// MaxConcurrency is 0).
func (s *crawlState) acquire() {
	if s.sem != nil {
		s.sem <- struct{}{}
	}
}

// release frees a semaphore slot.
func (s *crawlState) release() {
	if s.sem != nil {
		<-s.sem
	}
}

// hasError reports whether a fatal error has already been recorded.
func (s *crawlState) hasError() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.firstErr != nil
}

// URLToFilename converts a URL into a safe, filesystem-friendly filename.
//
// It strips the scheme (http:// / https://), replaces path separators and
// query-string characters (/ ? & =) with underscores, trims leading/trailing
// underscores, and caps the result at maxLen characters (0 = no limit).
// If the resulting string would be empty, "index" is returned.
func URLToFilename(rawURL string, maxLen int) string {
	name := rawURL
	name = strings.TrimPrefix(name, "https://")
	name = strings.TrimPrefix(name, "http://")

	replacer := strings.NewReplacer("/", "_", "?", "_", "&", "_", "=", "_")
	name = replacer.Replace(name)
	name = strings.Trim(name, "_")

	if name == "" {
		return "index"
	}
	if maxLen > 0 && len(name) > maxLen {
		name = name[:maxLen]
	}
	return name
}
