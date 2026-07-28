package metadata

import (
	"os"
	"testing"
	"time"
)

func TestParsePDFInfo(t *testing.T) {
	full := "Title:          AI Report: 2024 Outlook\n" +
		"Author:         Jane Doe\n" +
		"Creator:        LaTeX with hyperref\n" +
		"Producer:       pdfTeX-1.40\n" +
		"CreationDate:   Wed Jun  1 12:00:00 2022 UTC\n" +
		"ModDate:        Thu Jun  2 08:00:00 2022 UTC\n" +
		"Pages:          42\n" +
		"Encrypted:      no\n" +
		"Page size:      595.276 x 841.89 pts (A4)\n"

	meta := parsePDFInfo(full)
	if meta.Title != "AI Report: 2024 Outlook" {
		t.Errorf("Title = %q, want value with embedded colon preserved", meta.Title)
	}
	if len(meta.Authors) != 1 || meta.Authors[0] != "Jane Doe" {
		t.Errorf("Authors = %v, want [Jane Doe]", meta.Authors)
	}
	if meta.Publisher != "LaTeX with hyperref" {
		t.Errorf("Publisher = %q, want first of Creator/Producer", meta.Publisher)
	}
	if meta.PageCount != 42 {
		t.Errorf("PageCount = %d, want 42", meta.PageCount)
	}
	if meta.PublishedAt == nil {
		t.Fatal("PublishedAt = nil, want CreationDate parsed")
	}
	if got := meta.PublishedAt.Format("2006-01-02"); got != "2022-06-01" {
		t.Errorf("PublishedAt = %s, want CreationDate 2022-06-01 (ModDate must not override)", got)
	}
}

func TestParsePDFInfoPartialAndMalformed(t *testing.T) {
	cases := []struct {
		name   string
		output string
		check  func(t *testing.T, m DocMeta)
	}{
		{
			name:   "empty output",
			output: "",
			check: func(t *testing.T, m DocMeta) {
				if m.Title != "" || m.Authors != nil || m.PageCount != 0 || m.PublishedAt != nil {
					t.Errorf("want zero DocMeta, got %+v", m)
				}
			},
		},
		{
			name:   "producer used when no creator",
			output: "Producer: Ghostscript 9.5\n",
			check: func(t *testing.T, m DocMeta) {
				if m.Publisher != "Ghostscript 9.5" {
					t.Errorf("Publisher = %q, want Ghostscript 9.5", m.Publisher)
				}
			},
		},
		{
			name:   "moddate used when no creationdate",
			output: "ModDate: Wed Jun  1 12:00:00 2022 UTC\n",
			check: func(t *testing.T, m DocMeta) {
				if m.PublishedAt == nil {
					t.Error("PublishedAt = nil, want ModDate fallback")
				}
			},
		},
		{
			name:   "empty author skipped",
			output: "Author:\nTitle: X\n",
			check: func(t *testing.T, m DocMeta) {
				if m.Authors != nil {
					t.Errorf("Authors = %v, want nil for empty value", m.Authors)
				}
			},
		},
		{
			name:   "non-numeric pages ignored",
			output: "Pages: many\n",
			check: func(t *testing.T, m DocMeta) {
				if m.PageCount != 0 {
					t.Errorf("PageCount = %d, want 0", m.PageCount)
				}
			},
		},
		{
			name:   "lines without colon skipped",
			output: "garbage line\nTitle: Kept\n",
			check: func(t *testing.T, m DocMeta) {
				if m.Title != "Kept" {
					t.Errorf("Title = %q, want Kept", m.Title)
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.check(t, parsePDFInfo(c.output))
		})
	}
}

func TestParsePDFDate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"double space single digit day", "Wed Jun  1 12:00:00 2022 UTC", "2022-06-01"},
		{"single space double digit day", "Thu Jun 12 09:30:00 2025 UTC", "2025-06-12"},
		{"iso with offset", "2024-05-06T10:11:12+02:00", "2024-05-06"},
		{"plain date", "2024-05-06", "2024-05-06"},
		{"garbage", "not a date", ""},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parsePDFDate(c.in)
			if c.want == "" {
				if got != nil {
					t.Errorf("parsePDFDate(%q) = %v, want nil", c.in, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("parsePDFDate(%q) = nil, want %s", c.in, c.want)
			}
			if got.Format("2006-01-02") != c.want {
				t.Errorf("parsePDFDate(%q) = %s, want %s", c.in, got.Format(time.RFC3339), c.want)
			}
		})
	}
}

func TestWriteTempFile(t *testing.T) {
	body := []byte("%PDF-1.4 fake body")
	path, err := WriteTempFile(body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(path) }()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Errorf("temp file content = %q, want %q", got, body)
	}
}
