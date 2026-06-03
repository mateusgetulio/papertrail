package metadata

import (
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// FromHTML extracts metadata from an HTML document string.
func FromHTML(html string) (DocMeta, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return DocMeta{}, err
	}

	meta := DocMeta{}

	// Title: og:title > title tag
	if og := doc.Find(`meta[property="og:title"]`).AttrOr("content", ""); og != "" {
		meta.Title = og
	} else {
		meta.Title = strings.TrimSpace(doc.Find("title").Text())
	}

	// Author
	if author := doc.Find(`meta[name="author"]`).AttrOr("content", ""); author != "" {
		meta.Authors = []string{author}
	}

	// Publisher: og:site_name
	meta.Publisher = doc.Find(`meta[property="og:site_name"]`).AttrOr("content", "")

	// Published date: article:published_time or meta[name=date]
	dateStr := doc.Find(`meta[property="article:published_time"]`).AttrOr("content", "")
	if dateStr == "" {
		dateStr = doc.Find(`meta[name="date"]`).AttrOr("content", "")
	}
	if dateStr != "" {
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if t, err := time.Parse(layout, dateStr); err == nil {
				meta.PublishedAt = &t
				break
			}
		}
	}

	// Language
	meta.Language = doc.Find("html").AttrOr("lang", "")

	return meta, nil
}

// ExtractMainText strips HTML tags and extracts readable body text.
// In production, replace with a proper readability implementation.
func ExtractMainText(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}
	// Remove script/style/nav/header/footer nodes
	doc.Find("script, style, nav, header, footer, aside").Remove()

	// Prefer main/article content
	var text string
	if main := doc.Find("main, article, [role=main]").First(); main.Length() > 0 {
		text = main.Text()
	} else {
		text = doc.Find("body").Text()
	}

	// Normalise whitespace
	lines := strings.Split(text, "\n")
	var out []string
	for _, line := range lines {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return strings.Join(out, "\n"), nil
}
