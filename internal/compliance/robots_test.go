package compliance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestParseRobots(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		wantRules []robotsRule
		wantDelay time.Duration
	}{
		{
			name:      "empty file has no rules",
			content:   "",
			wantRules: nil,
		},
		{
			name:      "wildcard disallow",
			content:   "User-agent: *\nDisallow: /private",
			wantRules: []robotsRule{{allow: false, pathPfx: "/private"}},
		},
		{
			name:      "our bot section applies",
			content:   "User-agent: PaperTrailResearchBot\nDisallow: /reports",
			wantRules: []robotsRule{{allow: false, pathPfx: "/reports"}},
		},
		{
			name:      "other bot section ignored",
			content:   "User-agent: Googlebot\nDisallow: /everything",
			wantRules: nil,
		},
		{
			name:    "scope switches between agent sections",
			content: "User-agent: Googlebot\nDisallow: /a\n\nUser-agent: *\nDisallow: /b",
			wantRules: []robotsRule{
				{allow: false, pathPfx: "/b"},
			},
		},
		{
			name:    "allow and disallow both recorded in order",
			content: "User-agent: *\nAllow: /public\nDisallow: /",
			wantRules: []robotsRule{
				{allow: true, pathPfx: "/public"},
				{allow: false, pathPfx: "/"},
			},
		},
		{
			name:      "directives are case-insensitive",
			content:   "USER-AGENT: *\nDISALLOW: /admin",
			wantRules: []robotsRule{{allow: false, pathPfx: "/admin"}},
		},
		{
			name:      "comments and blank lines skipped",
			content:   "# banner\n\nUser-agent: *\n# note\nDisallow: /x",
			wantRules: []robotsRule{{allow: false, pathPfx: "/x"}},
		},
		{
			name:      "empty disallow value produces no rule",
			content:   "User-agent: *\nDisallow:",
			wantRules: nil,
		},
		{
			name:      "rules before any user-agent line ignored",
			content:   "Disallow: /orphan\nUser-agent: *\nDisallow: /x",
			wantRules: []robotsRule{{allow: false, pathPfx: "/x"}},
		},
		{
			name:      "integer crawl-delay",
			content:   "User-agent: *\nCrawl-delay: 5",
			wantDelay: 5 * time.Second,
		},
		{
			name:      "fractional crawl-delay",
			content:   "User-agent: *\nCrawl-delay: 2.5",
			wantDelay: 2500 * time.Millisecond,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			entry := parseRobots(c.content)
			if len(entry.rules) != len(c.wantRules) {
				t.Fatalf("rules = %+v, want %+v", entry.rules, c.wantRules)
			}
			for i, r := range entry.rules {
				if r != c.wantRules[i] {
					t.Errorf("rule %d = %+v, want %+v", i, r, c.wantRules[i])
				}
			}
			if entry.crawlDelay != c.wantDelay {
				t.Errorf("crawlDelay = %v, want %v", entry.crawlDelay, c.wantDelay)
			}
		})
	}
}

func seedCache(rules []robotsRule, delay time.Duration) *RobotsCache {
	rc := NewRobotsCache()
	rc.entries["https://example.com"] = &robotsEntry{
		rules:      rules,
		crawlDelay: delay,
		fetchedAt:  time.Now(),
	}
	return rc
}

func TestAllowedRuleMatching(t *testing.T) {
	cases := []struct {
		name  string
		rules []robotsRule
		path  string
		want  bool
	}{
		{
			name:  "no rules allows everything",
			rules: nil,
			path:  "/anything",
			want:  true,
		},
		{
			name:  "disallowed prefix blocks",
			rules: []robotsRule{{allow: false, pathPfx: "/private"}},
			path:  "/private/report.pdf",
			want:  false,
		},
		{
			name:  "unmatched path allowed by default",
			rules: []robotsRule{{allow: false, pathPfx: "/private"}},
			path:  "/public/report.pdf",
			want:  true,
		},
		{
			name: "first matching rule wins",
			rules: []robotsRule{
				{allow: true, pathPfx: "/private/ok"},
				{allow: false, pathPfx: "/private"},
			},
			path: "/private/ok/file.pdf",
			want: true,
		},
		{
			name: "later disallow still applies elsewhere",
			rules: []robotsRule{
				{allow: true, pathPfx: "/private/ok"},
				{allow: false, pathPfx: "/private"},
			},
			path: "/private/other",
			want: false,
		},
		{
			name:  "query string participates in prefix match",
			rules: []robotsRule{{allow: false, pathPfx: "/search"}},
			path:  "/search?q=x",
			want:  false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rc := seedCache(c.rules, 0)
			u := mustParse(t, "https://example.com"+c.path)
			if got := rc.Allowed(context.Background(), u); got != c.want {
				t.Errorf("Allowed(%s) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestCrawlDelay(t *testing.T) {
	u := mustParse(t, "https://example.com/x")

	rc := seedCache(nil, 7*time.Second)
	if got := rc.CrawlDelay(context.Background(), u); got != 7*time.Second {
		t.Errorf("CrawlDelay = %v, want 7s", got)
	}

	rc = seedCache(nil, 0)
	if got := rc.CrawlDelay(context.Background(), u); got != 3*time.Second {
		t.Errorf("CrawlDelay with no directive = %v, want 3s default", got)
	}
}

func TestRobotsFetchAndCache(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		hits.Add(1)
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /blocked"))
	}))
	defer srv.Close()

	rc := NewRobotsCache()
	blocked := mustParse(t, srv.URL+"/blocked/page")
	open := mustParse(t, srv.URL+"/open/page")

	if rc.Allowed(context.Background(), blocked) {
		t.Error("blocked path should be disallowed")
	}
	if !rc.Allowed(context.Background(), open) {
		t.Error("open path should be allowed")
	}
	if hits.Load() != 1 {
		t.Errorf("robots.txt fetched %d times, want 1 (cached)", hits.Load())
	}
}

func TestAllowedFailsClosedOnFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	rc := NewRobotsCache()
	u := mustParse(t, srv.URL+"/anything")
	if rc.Allowed(context.Background(), u) {
		t.Error("unreachable robots.txt must deny the fetch")
	}
}
