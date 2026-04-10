package crawler

import (
	"strings"
	"testing"
	"time"
)

// ── DefaultOptions ────────────────────────────────────────────────────────────

func TestDefaultOptions(t *testing.T) {
	t.Parallel()

	opts := DefaultOptions()

	if opts.PageTimeout != 30*time.Second {
		t.Errorf("PageTimeout = %v; want 30s", opts.PageTimeout)
	}
	if opts.RenderSleep != 2*time.Second {
		t.Errorf("RenderSleep = %v; want 2s", opts.RenderSleep)
	}
	if opts.MaxFilenameLen != 150 {
		t.Errorf("MaxFilenameLen = %d; want 150", opts.MaxFilenameLen)
	}
	if opts.MaxConcurrency != 5 {
		t.Errorf("MaxConcurrency = %d; want 5", opts.MaxConcurrency)
	}
}

// ── URLToFilename ─────────────────────────────────────────────────────────────

func TestURLToFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		url    string
		maxLen int
		want   string
	}{
		{
			name:   "https domain only",
			url:    "https://example.com",
			maxLen: 150,
			want:   "example.com",
		},
		{
			name:   "http scheme stripped",
			url:    "http://example.com",
			maxLen: 150,
			want:   "example.com",
		},
		{
			name:   "path separators replaced",
			url:    "http://example.com/foo/bar",
			maxLen: 150,
			want:   "example.com_foo_bar",
		},
		{
			name:   "query string replaced",
			url:    "https://example.com/search?q=hello&page=2",
			maxLen: 150,
			want:   "example.com_search_q_hello_page_2",
		},
		{
			name:   "trailing slash trimmed",
			url:    "https://example.com/",
			maxLen: 150,
			want:   "example.com",
		},
		{
			name:   "equals sign in query",
			url:    "https://example.com?key=value",
			maxLen: 150,
			want:   "example.com_key_value",
		},
		{
			name:   "empty after stripping returns index",
			url:    "https://",
			maxLen: 150,
			want:   "index",
		},
		{
			name:   "http:// only returns index",
			url:    "http://",
			maxLen: 150,
			want:   "index",
		},
		{
			name:   "truncated to maxLen",
			url:    "https://example.com/" + strings.Repeat("a", 200),
			maxLen: 20,
			// "example.com_" = 12 chars, plus 8 "a" = 20 total
			want: "example.com_" + strings.Repeat("a", 8),
		},
		{
			name:   "zero maxLen disables truncation",
			url:    "https://example.com/" + strings.Repeat("a", 200),
			maxLen: 0,
			want:   "example.com_" + strings.Repeat("a", 200),
		},
		{
			name:   "deep nested path",
			url:    "https://example.com/a/b/c/d",
			maxLen: 150,
			want:   "example.com_a_b_c_d",
		},
	}

	for _, tc := range tests {
		tc := tc // capture range var
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := URLToFilename(tc.url, tc.maxLen)
			if got != tc.want {
				t.Errorf("URLToFilename(%q, %d)\n  got  %q\n  want %q",
					tc.url, tc.maxLen, got, tc.want)
			}
		})
	}
}
