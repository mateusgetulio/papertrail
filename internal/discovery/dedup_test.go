package discovery

import (
	"bytes"
	"testing"
)

func TestCanonical(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "lowercases scheme host and path",
			raw:  "HTTP://EXAMPLE.COM/Some/Path",
			want: "http://example.com/some/path",
		},
		{
			name: "strips fragment",
			raw:  "https://example.com/page#section-2",
			want: "https://example.com/page",
		},
		{
			name: "removes default https port",
			raw:  "https://example.com:443/x",
			want: "https://example.com/x",
		},
		{
			name: "removes default http port",
			raw:  "http://example.com:80/x",
			want: "http://example.com/x",
		},
		{
			name: "keeps non-default port",
			raw:  "https://example.com:8443/x",
			want: "https://example.com:8443/x",
		},
		{
			name: "empty path becomes root",
			raw:  "https://example.com",
			want: "https://example.com/",
		},
		{
			name: "collapses double slashes",
			raw:  "https://example.com/a//b",
			want: "https://example.com/a/b",
		},
		{
			name: "sorts query params by key",
			raw:  "https://example.com/s?z=1&a=2&m=3",
			want: "https://example.com/s?a=2&m=3&z=1",
		},
		{
			name: "preserves value order within a repeated key",
			raw:  "https://example.com/s?z=1&a=2&a=1",
			want: "https://example.com/s?a=2&a=1&z=1",
		},
		{
			name: "everything at once",
			raw:  "HTTPS://Example.COM:443/Docs//Report?b=2&a=1#top",
			want: "https://example.com/docs/report?a=1&b=2",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Canonical(c.raw)
			if err != nil {
				t.Fatalf("Canonical(%q) unexpected error: %v", c.raw, err)
			}
			if got != c.want {
				t.Errorf("Canonical(%q) = %q, want %q", c.raw, got, c.want)
			}
		})
	}
}

func TestCanonicalInvalidURL(t *testing.T) {
	if _, err := Canonical("://missing-scheme"); err == nil {
		t.Error("expected error for unparseable URL")
	}
}

func TestCanonicalIsIdempotent(t *testing.T) {
	once, err := Canonical("HTTPS://Example.COM:443/Docs//Report?b=2&a=1#top")
	if err != nil {
		t.Fatal(err)
	}
	twice, err := Canonical(once)
	if err != nil {
		t.Fatal(err)
	}
	if once != twice {
		t.Errorf("second pass changed the URL: %q -> %q", once, twice)
	}
}

func TestURLHash(t *testing.T) {
	a := URLHash("https://example.com/a")
	b := URLHash("https://example.com/a")
	c := URLHash("https://example.com/b")
	if len(a) != 32 {
		t.Errorf("hash length = %d, want 32", len(a))
	}
	if !bytes.Equal(a, b) {
		t.Error("same input must produce the same hash")
	}
	if bytes.Equal(a, c) {
		t.Error("different inputs must produce different hashes")
	}
}

func TestDeduplicator(t *testing.T) {
	d := NewDeduplicator()
	if d.IsDuplicate("https://example.com/a") {
		t.Error("first sighting must not be a duplicate")
	}
	if !d.IsDuplicate("https://example.com/a") {
		t.Error("second sighting must be a duplicate")
	}
	if d.IsDuplicate("https://example.com/b") {
		t.Error("a different URL must not be a duplicate")
	}
}
