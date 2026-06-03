package compliance

import (
	"context"
	"net/url"
	"strings"
)

// gatedPathPrefixes are URL path patterns that signal a login/paywall gate.
var gatedPathPrefixes = []string{
	"/login", "/signin", "/sign-in", "/register", "/signup",
	"/gated", "/download-gate", "/subscribe", "/paywall",
}

// gatedQueryParams signals a gated download.
var gatedQueryParams = []string{"download", "email_gate", "register"}

// PolicyRegistry maps domain → policy flags (from the source table).
// In production this is backed by the DB; here it's a seed for known sources.
type PolicyRegistry struct {
	policies map[string]SourcePolicy
}

type SourcePolicy struct {
	AllowsAutomatedAccess bool
	AllowsExcerptStorage  bool
	TrustTier             string
}

func NewPolicyRegistry() *PolicyRegistry {
	pr := &PolicyRegistry{policies: make(map[string]SourcePolicy)}
	pr.seed()
	return pr
}

func (pr *PolicyRegistry) Policy(domain string) SourcePolicy {
	// exact match first (after stripping www.)
	d := strings.TrimPrefix(domain, "www.")
	if p, ok := pr.policies[d]; ok {
		return p
	}
	// base-domain fallback: match last two labels (e.g. weforum.org covers www3.weforum.org)
	parts := strings.Split(d, ".")
	if len(parts) >= 2 {
		base := parts[len(parts)-2] + "." + parts[len(parts)-1]
		if p, ok := pr.policies[base]; ok {
			return p
		}
	}
	return SourcePolicy{AllowsAutomatedAccess: false, AllowsExcerptStorage: true, TrustTier: "F"}
}

func (pr *PolicyRegistry) seed() {
	// Tier A — Multilateral / Gov / Open-license
	for _, d := range []string{
		"weforum.org", "oecd.org", "worldbank.org", "imf.org",
		"un.org", "europa.eu", "data.gov",
	} {
		pr.policies[d] = SourcePolicy{AllowsAutomatedAccess: true, AllowsExcerptStorage: true, TrustTier: "A"}
	}
	// WEF CDN subdomains block bot PDF downloads (504); treat as no-access
	for _, d := range []string{"www3.weforum.org", "reports.weforum.org"} {
		pr.policies[d] = SourcePolicy{AllowsAutomatedAccess: false, AllowsExcerptStorage: false, TrustTier: "A"}
	}
	// Tier B — Major consulting (public)
	for _, d := range []string{
		"pwc.com", "deloitte.com", "mckinsey.com", "bcg.com",
		"bain.com", "accenture.com", "capgemini.com",
	} {
		pr.policies[d] = SourcePolicy{AllowsAutomatedAccess: true, AllowsExcerptStorage: true, TrustTier: "B"}
	}
	// Tier E — Analyst public pages only (very limited)
	for _, d := range []string{"gartner.com", "forrester.com"} {
		pr.policies[d] = SourcePolicy{AllowsAutomatedAccess: false, AllowsExcerptStorage: true, TrustTier: "E"}
	}
}

// GateImpl is the real compliance gate combining robots, rate-limiting, and policy.
type GateImpl struct {
	robots   *RobotsCache
	rates    *RateLimiterRegistry
	policies *PolicyRegistry
}

func NewGate(robots *RobotsCache, rates *RateLimiterRegistry, policies *PolicyRegistry) *GateImpl {
	return &GateImpl{robots: robots, rates: rates, policies: policies}
}

func (g *GateImpl) Check(ctx context.Context, u *url.URL) (Decision, error) {
	domain := strings.TrimPrefix(u.Hostname(), "www.")

	// 1. Policy check
	policy := g.policies.Policy(domain)
	if !policy.AllowsAutomatedAccess {
		return Decision{Allowed: false, Reason: "source policy: automated access not permitted", RobotsOK: false, Gated: false}, nil
	}

	// 2. Gated path/param signals
	if isGatedURL(u) {
		return Decision{Allowed: false, Reason: "gated URL pattern detected", RobotsOK: true, Gated: true}, nil
	}

	// 3. Content type: only pdf and html
	path := strings.ToLower(u.Path)
	if !strings.HasSuffix(path, ".pdf") && !strings.HasSuffix(path, ".html") &&
		!strings.HasSuffix(path, "/") && !strings.HasSuffix(path, "") {
		// Allow paths without extension (could be HTML)
		if strings.Contains(path, ".") &&
			!strings.HasSuffix(path, ".htm") {
			return Decision{Allowed: false, Reason: "unsupported content type", RobotsOK: true, Gated: false}, nil
		}
	}

	// 4. robots.txt
	robotsOK := g.robots.Allowed(ctx, u)
	if !robotsOK {
		return Decision{Allowed: false, Reason: "robots.txt disallows this path", RobotsOK: false, Gated: false}, nil
	}

	// 5. Rate limit — apply crawl-delay before allowing
	delay := g.robots.CrawlDelay(ctx, u)
	if err := g.rates.Wait(ctx, domain, delay); err != nil {
		return Decision{Allowed: false, Reason: "rate limit wait cancelled: " + err.Error(), RobotsOK: true, Gated: false}, nil
	}

	return Decision{Allowed: true, Reason: "all checks passed", RobotsOK: true, Gated: false}, nil
}

func isGatedURL(u *url.URL) bool {
	path := strings.ToLower(u.Path)
	for _, pfx := range gatedPathPrefixes {
		if strings.HasPrefix(path, pfx) {
			return true
		}
	}
	q := strings.ToLower(u.RawQuery)
	for _, param := range gatedQueryParams {
		if strings.Contains(q, param) {
			return true
		}
	}
	return false
}
