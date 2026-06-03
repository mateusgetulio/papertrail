package fetch

import (
	"bytes"
	"strings"
)

// ContentKind classifies a fetched document.
type ContentKind string

const (
	KindPDF     ContentKind = "pdf"
	KindHTML    ContentKind = "html"
	KindUnknown ContentKind = "unknown"
)

// Detect classifies the content type of a fetch result.
// Prefers magic-byte sniffing over Content-Type header, which can lie.
func Detect(result *Result) ContentKind {
	if len(result.Body) >= 4 && bytes.HasPrefix(result.Body, []byte("%PDF")) {
		return KindPDF
	}
	ct := strings.ToLower(result.ContentType)
	if strings.Contains(ct, "pdf") {
		return KindPDF
	}
	if strings.Contains(ct, "html") || strings.Contains(ct, "text") {
		return KindHTML
	}
	// Sniff for HTML markers
	head := strings.ToLower(string(result.Body[:min(len(result.Body), 512)]))
	if strings.Contains(head, "<!doctype html") || strings.Contains(head, "<html") {
		return KindHTML
	}
	return KindUnknown
}
