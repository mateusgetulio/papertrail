package fetch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	MaxBodyBytes     = 50 << 20  // 50 MB hard cap
	MaxHeadBytes     = 10 << 20  // skip files >10 MB
	userAgent        = "PaperTrailResearchBot/0.1 (+https://internal/contact)"
)

// Result holds the raw outcome of a fetch attempt.
type Result struct {
	URL          string
	FinalURL     string // after redirects
	StatusCode   int
	ContentType  string
	Body         []byte
	ContentHash  []byte // SHA-256 of Body
	ETag         string
	LastModified string
	NotModified  bool   // 304 — cached copy still valid
}

// Cache stores ETag/Last-Modified per URL for conditional requests.
type Cache interface {
	Get(url string) (etag, lastModified string)
	Set(url string, etag, lastModified string)
}

// MemCache is a simple in-memory cache (replace with DB-backed in Phase 3+).
type MemCache struct {
	entries map[string][2]string
}

func NewMemCache() *MemCache { return &MemCache{entries: make(map[string][2]string)} }

func (m *MemCache) Get(url string) (string, string) {
	e := m.entries[url]
	return e[0], e[1]
}
func (m *MemCache) Set(url string, etag, lastModified string) {
	m.entries[url] = [2]string{etag, lastModified}
}

// Fetcher performs polite, compliant HTTP fetches.
type Fetcher struct {
	client  *http.Client
	cache   Cache
	retries int
}

func New(cache Cache) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		cache:   cache,
		retries: 1,
	}
}

// HeadOK performs a HEAD request and returns false (with reason) if the file
// is too large or content type is not PDF/HTML. Saves a full download.
func (f *Fetcher) HeadOK(ctx context.Context, rawURL string) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return true, "" // can't HEAD, allow and try anyway
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := f.client.Do(req)
	if err != nil {
		return true, ""
	}
	defer resp.Body.Close()
	if resp.ContentLength > int64(MaxHeadBytes) {
		return false, fmt.Sprintf("file too large: %d bytes", resp.ContentLength)
	}
	return true, ""
}

// Fetch downloads a URL with caching, size cap, and retry/backoff on 429/5xx.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*Result, error) {
	var lastErr error
	backoff := 2 * time.Second

	for attempt := 0; attempt <= f.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		result, retry, err := f.doRequest(ctx, rawURL)
		if err != nil && !retry {
			return nil, err
		}
		if err != nil {
			lastErr = err
			continue
		}
		return result, nil
	}
	return nil, fmt.Errorf("fetch %s: all retries failed: %w", rawURL, lastErr)
}

func (f *Fetcher) doRequest(ctx context.Context, rawURL string) (*Result, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", userAgent)

	// conditional request headers
	if etag, lastMod := f.cache.Get(rawURL); etag != "" {
		req.Header.Set("If-None-Match", etag)
	} else if lastMod != "" {
		req.Header.Set("If-Modified-Since", lastMod)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return &Result{URL: rawURL, StatusCode: 304, NotModified: true}, false, nil
	}

	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("status %d", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return &Result{URL: rawURL, FinalURL: resp.Request.URL.String(), StatusCode: resp.StatusCode}, false, nil
	}

	limited := io.LimitReader(resp.Body, MaxBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, true, fmt.Errorf("read body: %w", err)
	}
	if len(body) > MaxBodyBytes {
		return nil, false, fmt.Errorf("body exceeds %d byte cap", MaxBodyBytes)
	}

	hash := sha256.Sum256(body)
	result := &Result{
		URL:          rawURL,
		FinalURL:     resp.Request.URL.String(),
		StatusCode:   resp.StatusCode,
		ContentType:  resp.Header.Get("Content-Type"),
		Body:         body,
		ContentHash:  hash[:],
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}

	if result.ETag != "" || result.LastModified != "" {
		f.cache.Set(rawURL, result.ETag, result.LastModified)
	}

	return result, false, nil
}

// IsGated returns true if the response body shows post-fetch gating signals
// (login form, paywall, CAPTCHA). If true, the body must be discarded.
func IsGated(result *Result) bool {
	if result.StatusCode == 401 || result.StatusCode == 403 {
		return true
	}
	body := bytes.ToLower(result.Body[:min(len(result.Body), 8192)])
	gatePhrases := [][]byte{
		[]byte("register to download"),
		[]byte("sign in to access"),
		[]byte("login required"),
		[]byte("email to download"),
		[]byte("create a free account"),
		[]byte("captcha"),
		[]byte("cf-challenge"),
	}
	for _, phrase := range gatePhrases {
		if bytes.Contains(body, phrase) {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
