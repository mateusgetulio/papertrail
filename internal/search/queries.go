package search

import "fmt"

// QueryTemplate expands to a list of search queries from dimensions defined in docs/02 §4.
type QueryTemplate struct {
	Pattern    string // printf pattern with %s placeholders
	Dimensions [][]string
}

// Expand returns all combinations of the template's dimensions.
func (t QueryTemplate) Expand() []string {
	if len(t.Dimensions) == 0 {
		return []string{t.Pattern}
	}
	var out []string
	var recurse func(pos int, parts []string)
	recurse = func(pos int, parts []string) {
		if pos == len(t.Dimensions) {
			args := make([]any, len(parts))
			for i, p := range parts {
				args[i] = p
			}
			out = append(out, fmt.Sprintf(t.Pattern, args...))
			return
		}
		for _, v := range t.Dimensions[pos] {
			recurse(pos+1, append(parts, v))
		}
	}
	recurse(0, nil)
	return out
}

// DefaultTemplates returns the seed query templates (docs/02 §4).
//
// Query budget is deliberately weighted toward sources that pass the compliance
// gate. WEF/OECD PDFs are gated (CDN bot-block / robots.txt), so we do NOT spend
// queries on `site:weforum.org filetype:pdf` etc. — instead we target World Bank
// (the proven-productive source), IMF, and the open IGO/gov/central-bank domains
// in the allowlist, plus broad generic queries, across many industries + themes.
func DefaultTemplates() []QueryTemplate {
	themes := []string{
		"disruption", "digital transformation", "automation", "pain points",
		"emerging technology", "regulatory change", "supply chain", "fraud",
		"cost reduction", "workflow", "interoperability", "financial inclusion",
	}
	industries := []string{
		"healthcare", "finance", "insurance", "manufacturing", "logistics",
		"retail", "energy", "agriculture", "education", "government",
		"real estate", "telecom", "tourism", "construction", "mining",
		"water", "public sector", "trade",
	}
	themesCore := []string{
		"technology", "automation", "digital", "disruption",
		"innovation", "data", "platform", "efficiency",
	}
	years := []string{"2024", "2025", "2026"}

	return []QueryTemplate{
		// World Bank — the source that actually passes the gate; cross industries
		// with core themes for broad coverage.
		{
			Pattern:    `site:worldbank.org filetype:pdf "%s" "%s"`,
			Dimensions: [][]string{industries, themesCore},
		},
		// IMF — digital/finance angle across industries.
		{
			Pattern:    `site:imf.org filetype:pdf "digital" "%s"`,
			Dimensions: [][]string{industries},
		},
		// Newly-allowlisted open IGO / central-bank / gov research.
		{
			Pattern:    `site:bis.org filetype:pdf "%s"`,
			Dimensions: [][]string{themes},
		},
		{
			Pattern:    `site:federalreserve.gov filetype:pdf "%s"`,
			Dimensions: [][]string{themes},
		},
		{
			Pattern:    `site:ec.europa.eu filetype:pdf "%s" "%s"`,
			Dimensions: [][]string{themes, years},
		},
		// Consulting (public, allowlisted B-tier).
		{
			Pattern:    `site:pwc.com filetype:pdf "%s" "report"`,
			Dimensions: [][]string{themes},
		},
		{
			Pattern:    `site:deloitte.com filetype:pdf "%s" "outlook"`,
			Dimensions: [][]string{themes},
		},
		// Broad generic — any domain (allowlist still gates results).
		{
			Pattern:    `"%s" "white paper" filetype:pdf "%s"`,
			Dimensions: [][]string{industries, []string{"disruption", "automation", "inefficiency"}},
		},
		{
			Pattern:    `"manual process" OR "data silos" OR "inefficiency" "industry report" filetype:pdf "%s"`,
			Dimensions: [][]string{industries},
		},
	}
}
