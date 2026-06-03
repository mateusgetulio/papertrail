package compliance

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	robotsTTL = 24 * time.Hour
	userAgent = "PaperTrailResearchBot/0.1 (+https://internal/contact)"
)

// RobotsCache fetches and caches robots.txt per domain with a TTL.
type RobotsCache struct {
	mu      sync.Mutex
	entries map[string]*robotsEntry
	http    *http.Client
}

type robotsEntry struct {
	rules     []robotsRule
	crawlDelay time.Duration
	fetchedAt time.Time
}

type robotsRule struct {
	allow    bool
	pathPfx  string
}

func NewRobotsCache() *RobotsCache {
	return &RobotsCache{
		entries: make(map[string]*robotsEntry),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Allowed returns true if the given URL path is allowed for our user-agent.
// On fetch error it fails closed (returns false) for safety.
func (rc *RobotsCache) Allowed(ctx context.Context, u *url.URL) bool {
	entry, err := rc.get(ctx, u)
	if err != nil {
		return false
	}
	path := u.RequestURI()
	for _, rule := range entry.rules {
		if strings.HasPrefix(path, rule.pathPfx) {
			return rule.allow
		}
	}
	return true
}

// CrawlDelay returns the crawl-delay for the domain, or a safe default.
func (rc *RobotsCache) CrawlDelay(ctx context.Context, u *url.URL) time.Duration {
	entry, err := rc.get(ctx, u)
	if err != nil || entry.crawlDelay == 0 {
		return 3 * time.Second
	}
	return entry.crawlDelay
}

func (rc *RobotsCache) get(ctx context.Context, u *url.URL) (*robotsEntry, error) {
	key := u.Scheme + "://" + u.Host
	rc.mu.Lock()
	e, ok := rc.entries[key]
	rc.mu.Unlock()
	if ok && time.Since(e.fetchedAt) < robotsTTL {
		return e, nil
	}
	return rc.fetch(ctx, key)
}

func (rc *RobotsCache) fetch(ctx context.Context, base string) (*robotsEntry, error) {
	robotsURL := base + "/robots.txt"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := rc.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("robots fetch %s: %w", robotsURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	entry := parseRobots(string(body))
	entry.fetchedAt = time.Now()

	rc.mu.Lock()
	rc.entries[base] = entry
	rc.mu.Unlock()
	return entry, nil
}

// parseRobots is a minimal robots.txt parser scoped to our user-agent and *.
func parseRobots(content string) *robotsEntry {
	entry := &robotsEntry{}
	var inScope bool

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "user-agent:") {
			ua := strings.TrimSpace(line[len("user-agent:"):])
			inScope = ua == "*" ||
				strings.EqualFold(ua, "PaperTrailResearchBot") ||
				strings.EqualFold(ua, "papertrailresearchbot")
			continue
		}
		if !inScope {
			continue
		}
		if strings.HasPrefix(lower, "disallow:") {
			path := strings.TrimSpace(line[len("disallow:"):])
			if path != "" {
				entry.rules = append(entry.rules, robotsRule{allow: false, pathPfx: path})
			}
		} else if strings.HasPrefix(lower, "allow:") {
			path := strings.TrimSpace(line[len("allow:"):])
			if path != "" {
				entry.rules = append(entry.rules, robotsRule{allow: true, pathPfx: path})
			}
		} else if strings.HasPrefix(lower, "crawl-delay:") {
			var secs float64
			fmt.Sscanf(strings.TrimSpace(line[len("crawl-delay:"):]), "%f", &secs)
			entry.crawlDelay = time.Duration(secs * float64(time.Second))
		}
	}
	return entry
}

// RateLimiterRegistry holds per-domain rate limiters.
type RateLimiterRegistry struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

func NewRateLimiterRegistry() *RateLimiterRegistry {
	return &RateLimiterRegistry{limiters: make(map[string]*rate.Limiter)}
}

// Wait blocks until the domain's rate limiter allows a request.
func (r *RateLimiterRegistry) Wait(ctx context.Context, domain string, delay time.Duration) error {
	lim := r.limiterFor(domain, delay)
	return lim.Wait(ctx)
}

func (r *RateLimiterRegistry) limiterFor(domain string, delay time.Duration) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if l, ok := r.limiters[domain]; ok {
		return l
	}
	if delay <= 0 {
		delay = 3 * time.Second
	}
	l := rate.NewLimiter(rate.Every(delay), 1)
	r.limiters[domain] = l
	return l
}
