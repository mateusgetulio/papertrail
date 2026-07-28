package metadata

import (
	"strings"
	"testing"
	"time"
)

func TestFromHTML(t *testing.T) {
	cases := []struct {
		name          string
		html          string
		wantTitle     string
		wantAuthors   []string
		wantPublisher string
		wantDate      string
		wantLang      string
	}{
		{
			name:      "og title preferred over title tag",
			html:      `<html><head><meta property="og:title" content="OG Title"/><title>Tag Title</title></head></html>`,
			wantTitle: "OG Title",
		},
		{
			name:      "title tag fallback trims whitespace",
			html:      `<html><head><title>  Spaced Title  </title></head></html>`,
			wantTitle: "Spaced Title",
		},
		{
			name:        "author meta",
			html:        `<html><head><meta name="author" content="Jane Doe"/></head></html>`,
			wantAuthors: []string{"Jane Doe"},
		},
		{
			name:          "publisher from og site name",
			html:          `<html><head><meta property="og:site_name" content="Example Press"/></head></html>`,
			wantPublisher: "Example Press",
		},
		{
			name:     "published time RFC3339",
			html:     `<html><head><meta property="article:published_time" content="2024-03-05T10:30:00Z"/></head></html>`,
			wantDate: "2024-03-05T10:30:00Z",
		},
		{
			name:     "date meta in plain date format",
			html:     `<html><head><meta name="date" content="2024-03-05"/></head></html>`,
			wantDate: "2024-03-05T00:00:00Z",
		},
		{
			name:     "article published time preferred over date meta",
			html:     `<html><head><meta property="article:published_time" content="2024-01-01T00:00:00Z"/><meta name="date" content="2020-01-01"/></head></html>`,
			wantDate: "2024-01-01T00:00:00Z",
		},
		{
			name: "unparseable date leaves PublishedAt nil",
			html: `<html><head><meta name="date" content="last Tuesday"/></head></html>`,
		},
		{
			name:      "language from html lang attribute",
			html:      `<html lang="en"><head><title>x</title></head></html>`,
			wantTitle: "x",
			wantLang:  "en",
		},
		{
			name: "empty document yields zero metadata",
			html: ``,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			meta, err := FromHTML(c.html)
			if err != nil {
				t.Fatal(err)
			}
			if meta.Title != c.wantTitle {
				t.Errorf("Title = %q, want %q", meta.Title, c.wantTitle)
			}
			if len(meta.Authors) != len(c.wantAuthors) {
				t.Errorf("Authors = %v, want %v", meta.Authors, c.wantAuthors)
			} else {
				for i := range c.wantAuthors {
					if meta.Authors[i] != c.wantAuthors[i] {
						t.Errorf("Authors[%d] = %q, want %q", i, meta.Authors[i], c.wantAuthors[i])
					}
				}
			}
			if meta.Publisher != c.wantPublisher {
				t.Errorf("Publisher = %q, want %q", meta.Publisher, c.wantPublisher)
			}
			if c.wantDate == "" {
				if meta.PublishedAt != nil {
					t.Errorf("PublishedAt = %v, want nil", meta.PublishedAt)
				}
			} else {
				want, err := time.Parse(time.RFC3339, c.wantDate)
				if err != nil {
					t.Fatal(err)
				}
				if meta.PublishedAt == nil || !meta.PublishedAt.Equal(want) {
					t.Errorf("PublishedAt = %v, want %v", meta.PublishedAt, want)
				}
			}
			if meta.Language != c.wantLang {
				t.Errorf("Language = %q, want %q", meta.Language, c.wantLang)
			}
		})
	}
}

func TestExtractMainText(t *testing.T) {
	cases := []struct {
		name        string
		html        string
		want        string
		wantAbsent  []string
		wantPresent []string
	}{
		{
			name: "prefers main over surrounding chrome",
			html: "<html><body>\n<nav>Site Nav</nav>\n<main>\n<p>Hello</p>\n<p>World</p>\n</main>\n<footer>Footer</footer>\n</body></html>",
			want: "Hello\nWorld",
		},
		{
			name: "prefers article when no main",
			html: "<html><body>\n<header>Head</header>\n<article>\n<p>Body copy</p>\n</article>\n</body></html>",
			want: "Body copy",
		},
		{
			name:        "falls back to body and strips script and style",
			html:        "<html><body>\n<script>var secret = 1;</script>\n<style>.x{color:red}</style>\n<p>Visible text</p>\n</body></html>",
			wantPresent: []string{"Visible text"},
			wantAbsent:  []string{"var secret", "color:red"},
		},
		{
			name:       "aside removed",
			html:       "<html><body>\n<main>\n<p>Keep</p>\n<aside>Related links</aside>\n</main>\n</body></html>",
			want:       "Keep",
			wantAbsent: []string{"Related links"},
		},
		{
			name: "blank lines collapsed",
			html: "<html><body>\n<main>\n<p>One</p>\n\n\n<p>Two</p>\n</main>\n</body></html>",
			want: "One\nTwo",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ExtractMainText(c.html)
			if err != nil {
				t.Fatal(err)
			}
			if c.want != "" && got != c.want {
				t.Errorf("ExtractMainText = %q, want %q", got, c.want)
			}
			for _, s := range c.wantPresent {
				if !strings.Contains(got, s) {
					t.Errorf("output %q missing %q", got, s)
				}
			}
			for _, s := range c.wantAbsent {
				if strings.Contains(got, s) {
					t.Errorf("output %q should not contain %q", got, s)
				}
			}
		})
	}
}
