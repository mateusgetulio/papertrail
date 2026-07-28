package compliance

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestIsGatedURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want bool
	}{
		{"login path", "https://example.com/login", true},
		{"signin path", "https://example.com/signin/next", true},
		{"uppercase path lowered before match", "https://example.com/Sign-In", true},
		{"register path", "https://example.com/register?next=/", true},
		{"paywall path", "https://example.com/paywall", true},
		{"download query param", "https://example.com/report.pdf?download=1", true},
		{"email gate query param", "https://example.com/report?email_gate=true", true},
		{"uppercase query lowered before match", "https://example.com/report?DOWNLOAD=1", true},
		{"plain article", "https://example.com/insights/report.pdf", false},
		{"login not at path start", "https://example.com/blog/login-security", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isGatedURL(mustParse(t, c.url)); got != c.want {
				t.Errorf("isGatedURL(%s) = %v, want %v", c.url, got, c.want)
			}
		})
	}
}

func TestPolicyRegistry(t *testing.T) {
	pr := NewPolicyRegistry()
	cases := []struct {
		name        string
		domain      string
		wantAllowed bool
		wantTier    string
	}{
		{"tier A exact match", "worldbank.org", true, "A"},
		{"www prefix stripped", "www.worldbank.org", true, "A"},
		{"subdomain falls back to base domain", "documents1.worldbank.org", true, "A"},
		{"tier B consulting", "pwc.com", true, "B"},
		{"blocked WEF CDN overrides base allowlist", "www3.weforum.org", false, "A"},
		{"WEF base domain itself allowed", "weforum.org", true, "A"},
		{"tier E analyst denied automation", "gartner.com", false, "E"},
		{"unknown domain defaults to tier F denied", "random-blog.example", false, "F"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := pr.Policy(c.domain)
			if p.AllowsAutomatedAccess != c.wantAllowed {
				t.Errorf("Policy(%s).AllowsAutomatedAccess = %v, want %v", c.domain, p.AllowsAutomatedAccess, c.wantAllowed)
			}
			if p.TrustTier != c.wantTier {
				t.Errorf("Policy(%s).TrustTier = %q, want %q", c.domain, p.TrustTier, c.wantTier)
			}
		})
	}
}

func newTestGate(robots *RobotsCache) *GateImpl {
	if robots == nil {
		robots = NewRobotsCache()
	}
	return NewGate(robots, NewRateLimiterRegistry(), NewPolicyRegistry())
}

func seedGateRobots(rc *RobotsCache, host string, rules []robotsRule) {
	rc.entries["https://"+host] = &robotsEntry{rules: rules, fetchedAt: time.Now()}
}

func TestGateCheckDeniesByPolicy(t *testing.T) {
	g := newTestGate(nil)
	d, err := g.Check(context.Background(), mustParse(t, "https://random-blog.example/post.html"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed {
		t.Error("unknown domain must be denied")
	}
	if !strings.Contains(d.Reason, "source policy") {
		t.Errorf("reason = %q, want a source-policy denial", d.Reason)
	}
}

func TestGateCheckDeniesGatedURL(t *testing.T) {
	g := newTestGate(nil)
	d, err := g.Check(context.Background(), mustParse(t, "https://pwc.com/login"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed || !d.Gated {
		t.Errorf("gated URL should be denied with Gated=true, got %+v", d)
	}
}

func TestGateCheckDeniesUnsupportedContentType(t *testing.T) {
	g := newTestGate(nil)
	d, err := g.Check(context.Background(), mustParse(t, "https://pwc.com/report.docx"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed {
		t.Error("docx must be denied")
	}
	if !strings.Contains(d.Reason, "unsupported content type") {
		t.Errorf("reason = %q, want unsupported content type", d.Reason)
	}
}

func TestGateCheckDeniesRobotsDisallowedPath(t *testing.T) {
	rc := NewRobotsCache()
	seedGateRobots(rc, "pwc.com", []robotsRule{{allow: false, pathPfx: "/secret"}})
	g := newTestGate(rc)

	d, err := g.Check(context.Background(), mustParse(t, "https://pwc.com/secret/report.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed || d.RobotsOK {
		t.Errorf("robots-disallowed path should be denied, got %+v", d)
	}
}

func TestGateCheckAllowsCompliantPDF(t *testing.T) {
	rc := NewRobotsCache()
	seedGateRobots(rc, "deloitte.com", nil)
	g := newTestGate(rc)

	d, err := g.Check(context.Background(), mustParse(t, "https://deloitte.com/insights/report.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Allowed || !d.RobotsOK || d.Gated {
		t.Errorf("compliant PDF should pass all checks, got %+v", d)
	}
}

func TestGateCheckAllowsHTMAndExtensionlessPaths(t *testing.T) {
	rc := NewRobotsCache()
	seedGateRobots(rc, "accenture.com", nil)
	seedGateRobots(rc, "capgemini.com", nil)
	g := newTestGate(rc)

	for _, raw := range []string{
		"https://accenture.com/insights/page.htm",
		"https://capgemini.com/research/annual-review",
	} {
		d, err := g.Check(context.Background(), mustParse(t, raw))
		if err != nil {
			t.Fatal(err)
		}
		if !d.Allowed {
			t.Errorf("%s should pass the content-type check, got %+v", raw, d)
		}
	}
}

func TestGateCheckReportsCancelledRateWait(t *testing.T) {
	rc := NewRobotsCache()
	seedGateRobots(rc, "bain.com", nil)
	g := newTestGate(rc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d, err := g.Check(ctx, mustParse(t, "https://bain.com/report.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed {
		t.Error("cancelled context must deny the fetch")
	}
	if !strings.Contains(d.Reason, "rate limit wait cancelled") {
		t.Errorf("reason = %q, want rate-limit cancellation", d.Reason)
	}
}

func TestAlwaysDenyGate(t *testing.T) {
	d, err := AlwaysDeny{}.Check(context.Background(), mustParse(t, "https://worldbank.org/report.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed {
		t.Error("AlwaysDeny must never allow")
	}
}
