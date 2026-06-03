package config

// CuratedSource is a manually-approved seed document or feed for ingestion.
//
// Add ONLY sources whose robots.txt and Terms of Service permit automated
// access. This list is a convenience, not an override: the compliance gate still
// validates every entry at fetch time, so a non-compliant URL here is skipped
// (logged as rejected), never force-fetched. See docs/compliance.md.
type CuratedSource struct {
	Name string
	URL  string
	Type string // "pdf" | "html"
}

// CuratedSources returns the manually-approved seed list. It is intentionally
// small and conservative — grow it with known-good, automation-permitted open
// sources (open-access repositories, government/IGO reports, RSS/landing pages)
// rather than relying solely on search discovery, which is brittle for
// site:filetype:pdf "exact phrase" queries.
func CuratedSources() []CuratedSource {
	return []CuratedSource{
		// Confirmed open-access, automation-permitted (World Bank Documents &
		// Reports). These were validated end-to-end by the ingest pipeline.
		{
			Name: "World Bank — AI and Healthcare in Emerging Markets",
			URL:  "https://documents1.worldbank.org/curated/en/733971606368563566/pdf/artificial-intelligence-and-healthcare-in-emerging-markets.pdf",
			Type: "pdf",
		},
		// Add more here, e.g.:
		//   {Name: "...", URL: "https://www.imf.org/.../report.pdf", Type: "pdf"},
		//   {Name: "...", URL: "https://www.bis.org/publ/....pdf",   Type: "pdf"},
		//   {Name: "...", URL: "https://www.federalreserve.gov/.../report.htm", Type: "html"},
	}
}
