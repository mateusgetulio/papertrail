package discovery

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"net/url"

	"github.com/mateusgetulio/papertrail/internal/compliance"
	"github.com/mateusgetulio/papertrail/internal/search"
)

// Candidate is a vetted, deduplicated URL ready for ingestion gating.
type Candidate struct {
	URL              string
	CanonicalURL     string
	URLHash          []byte
	Title            string
	Description      string
	Domain           string
	DiscoveredVia    string // provider:query
	ContentTypeGuess string // "pdf" or "html"
	GateDecision     compliance.Decision
}

// Runner orchestrates search → dedup → compliance gate for a set of queries.
type Runner struct {
	provider search.Provider
	gate     compliance.Gate
	dedup    *Deduplicator
}

func NewRunner(provider search.Provider, gate compliance.Gate) *Runner {
	return &Runner{
		provider: provider,
		gate:     gate,
		dedup:    NewDeduplicator(),
	}
}

// Run expands queries, searches, deduplicates, and gates each result.
// Allowed candidates are sent to the out channel; gate failures are logged.
func (r *Runner) Run(ctx context.Context, queries []string, resultsPerQuery int, out chan<- Candidate) error {
	for _, q := range queries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		results, err := r.provider.Search(ctx, q, resultsPerQuery)
		if err != nil {
			slog.Warn("search failed", "query", q, "err", err)
			continue
		}
		slog.Info("search", "provider", r.provider.Name(), "query", q, "hits", len(results))

		for _, result := range results {
			canonical, err := Canonical(result.URL)
			if err != nil {
				slog.Warn("canonicalize failed", "url", result.URL, "err", err)
				continue
			}
			if r.dedup.IsDuplicate(canonical) {
				continue
			}

			parsed, err := url.Parse(canonical)
			if err != nil {
				continue
			}

			decision, err := r.gate.Check(ctx, parsed)
			if err != nil {
				slog.Warn("gate error", "url", canonical, "err", err)
				continue
			}

			c := Candidate{
				URL:              result.URL,
				CanonicalURL:     canonical,
				URLHash:          urlHash(canonical),
				Title:            result.Title,
				Description:      result.Description,
				Domain:           result.SourceDomain,
				DiscoveredVia:    r.provider.Name() + ":" + q,
				ContentTypeGuess: guessContentType(canonical),
				GateDecision:     decision,
			}

			if decision.Allowed {
				out <- c
			} else {
				slog.Info("gated", "url", canonical, "reason", decision.Reason)
			}
		}
	}
	return nil
}

func guessContentType(u string) string {
	lower := url.PathEscape(u)
	if len(u) > 4 && u[len(u)-4:] == ".pdf" {
		return "pdf"
	}
	_ = lower
	return "html"
}

func urlHash(canonical string) []byte {
	h := sha256.Sum256([]byte(canonical))
	return h[:]
}
