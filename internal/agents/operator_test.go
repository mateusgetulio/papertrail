package agents

import (
	"testing"
)

func TestDocType(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"pdf", "pdf"},
		{"html", "html"},
		{"docx", "other"},
		{"", "other"},
	}
	for _, c := range cases {
		if got := docType(c.in); got != c.want {
			t.Errorf("docType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTrustScore(t *testing.T) {
	cases := []struct {
		tier string
		want int
	}{
		{"A", 6}, {"B", 5}, {"C", 4}, {"D", 3}, {"E", 2}, {"F", 1},
		{"", 1}, {"Z", 1},
	}
	for _, c := range cases {
		if got := trustScore(c.tier); got != c.want {
			t.Errorf("trustScore(%q) = %d, want %d", c.tier, got, c.want)
		}
	}
}

func TestCandidateFromURL(t *testing.T) {
	raw := "https://WWW.WorldBank.org/Curated//Report.PDF#page=2"
	cand, err := candidateFromURL(raw, "pdf")
	if err != nil {
		t.Fatal(err)
	}
	if cand.URL != raw {
		t.Errorf("URL = %q, want the raw URL preserved", cand.URL)
	}
	if cand.CanonicalURL != "https://www.worldbank.org/curated/report.pdf" {
		t.Errorf("CanonicalURL = %q", cand.CanonicalURL)
	}
	if len(cand.URLHash) != 32 {
		t.Errorf("URLHash length = %d, want 32", len(cand.URLHash))
	}
	if cand.Domain != "worldbank.org" {
		t.Errorf("Domain = %q, want www stripped", cand.Domain)
	}
	if cand.DiscoveredVia != "curated" {
		t.Errorf("DiscoveredVia = %q, want curated", cand.DiscoveredVia)
	}
	if cand.ContentTypeGuess != "pdf" {
		t.Errorf("ContentTypeGuess = %q, want pdf", cand.ContentTypeGuess)
	}
}

func TestCandidateFromURLInvalid(t *testing.T) {
	if _, err := candidateFromURL("://bad", "pdf"); err == nil {
		t.Error("expected error for unparseable URL")
	}
}
