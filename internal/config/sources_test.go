package config

import (
	"net/url"
	"testing"
)

func TestCuratedSources(t *testing.T) {
	sources := CuratedSources()
	if len(sources) == 0 {
		t.Fatal("curated seed list must not be empty")
	}

	seen := map[string]bool{}
	for i, s := range sources {
		if s.Name == "" {
			t.Errorf("source %d has empty Name", i)
		}
		if s.Type != "pdf" && s.Type != "html" {
			t.Errorf("source %d (%s) Type = %q, want pdf or html", i, s.Name, s.Type)
		}
		u, err := url.Parse(s.URL)
		if err != nil {
			t.Errorf("source %d (%s) URL unparseable: %v", i, s.Name, err)
			continue
		}
		if u.Scheme != "https" {
			t.Errorf("source %d (%s) scheme = %q, want https", i, s.Name, u.Scheme)
		}
		if u.Host == "" {
			t.Errorf("source %d (%s) has no host", i, s.Name)
		}
		if seen[s.URL] {
			t.Errorf("duplicate curated URL %q", s.URL)
		}
		seen[s.URL] = true
	}
}
