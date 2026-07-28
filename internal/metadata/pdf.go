package metadata

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DocMeta holds extracted document metadata.
type DocMeta struct {
	Title       string
	Authors     []string
	Publisher   string
	PublishedAt *time.Time
	Language    string
	PageCount   int
}

// FromPDF extracts metadata from a PDF file using pdfinfo (Poppler).
// path must be a readable file path; the file should be a temp file that will
// be purged after processing (docs/03 §6 — full text not persisted).
func FromPDF(ctx context.Context, path string) (DocMeta, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "pdfinfo", path)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return DocMeta{}, fmt.Errorf("pdfinfo: %w (stderr: %s)", err, stderr.String())
	}
	return parsePDFInfo(stdout.String()), nil
}

func parsePDFInfo(output string) DocMeta {
	meta := DocMeta{}
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "Title":
			meta.Title = val
		case "Author":
			if val != "" {
				meta.Authors = []string{val}
			}
		case "Creator", "Producer":
			if meta.Publisher == "" {
				meta.Publisher = val
			}
		case "Pages":
			if n, err := strconv.Atoi(val); err == nil {
				meta.PageCount = n
			}
		case "CreationDate", "ModDate":
			if meta.PublishedAt == nil {
				if t := parsePDFDate(val); t != nil {
					meta.PublishedAt = t
				}
			}
		}
	}
	return meta
}

// parsePDFDate handles pdfinfo date formats: "Wed Jun  1 12:00:00 2022 UTC" or ISO variants.
func parsePDFDate(s string) *time.Time {
	layouts := []string{
		"Mon Jan  2 15:04:05 2006 MST",
		"Mon Jan 2 15:04:05 2006 MST",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

// WriteTempFile writes body bytes to a temp file, returning its path.
// The caller MUST call os.Remove(path) after processing (compliance: no full-text retention).
func WriteTempFile(body []byte) (string, error) {
	f, err := os.CreateTemp("", "pt_pdf_*.pdf")
	if err != nil {
		return "", fmt.Errorf("temp file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(body); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("write temp: %w", err)
	}
	return f.Name(), nil
}
