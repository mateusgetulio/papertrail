package extract

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// PDF extracts plain text from a PDF file using pdftotext (Poppler).
// Falls back to tesseract OCR if pdftotext returns empty text.
func PDF(ctx context.Context, path string) (string, error) {
	text, err := pdftotext(ctx, path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) != "" {
		return text, nil
	}
	return tesseractFallback(ctx, path)
}

func pdftotext(ctx context.Context, path string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", path, "-")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext: %w (stderr: %s)", err, stderr.String())
	}
	return stdout.String(), nil
}

func tesseractFallback(ctx context.Context, pdfPath string) (string, error) {
	// Convert PDF pages to images then OCR — only invoked for scanned/empty PDFs.
	// Requires: pdftoppm (Poppler) + tesseract.
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "sh", "-c",
		fmt.Sprintf(`pdftoppm -r 200 "%s" /tmp/pt_ocr && tesseract /tmp/pt_ocr*.ppm stdout 2>/dev/null`, pdfPath))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tesseract fallback: %w (stderr: %s)", err, stderr.String())
	}
	return stdout.String(), nil
}

// HTML extracts the main readable text from an HTML string.
// For now uses a simple tag-stripping pass; replace with a readability port for production.
func HTML(raw string) string {
	// Strip tags naively — good enough for well-structured reports.
	// The internal/fetch package should run goquery readability before calling this.
	var out strings.Builder
	inTag := false
	for _, r := range raw {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			out.WriteRune(' ')
		case !inTag:
			out.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(out.String()), " ")
}
