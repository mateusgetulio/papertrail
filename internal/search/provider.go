package search

import "context"

// Result is a single search hit from any provider.
type Result struct {
	URL         string
	Title       string
	Description string
	SourceDomain string
}

// Provider is the interface all search backends implement.
// Implement internal/search/brave/brave.go for the Brave Search API.
type Provider interface {
	Search(ctx context.Context, query string, count int) ([]Result, error)
	Name() string
}
