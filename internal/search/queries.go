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

// DefaultTemplates returns the seed query templates from docs/02 §4.
func DefaultTemplates() []QueryTemplate {
	themes := []string{
		"disruption", "digital transformation", "industry outlook",
		"automation opportunity", "pain points", "emerging technology",
	}
	years := []string{"2024", "2025", "2026"}
	industries := []string{
		"healthcare", "finance", "manufacturing", "logistics", "retail", "energy",
	}

	return []QueryTemplate{
		{
			Pattern:    `site:weforum.org filetype:pdf "%s" "%s"`,
			Dimensions: [][]string{themes, years},
		},
		{
			Pattern:    `site:oecd.org filetype:pdf "%s" "%s"`,
			Dimensions: [][]string{themes, years},
		},
		{
			Pattern:    `site:worldbank.org filetype:pdf "technology" "%s"`,
			Dimensions: [][]string{industries},
		},
		{
			Pattern:    `site:imf.org filetype:pdf "digital" "%s" "%s"`,
			Dimensions: [][]string{themes, years},
		},
		{
			Pattern:    `site:deloitte.com filetype:pdf "outlook" "%s"`,
			Dimensions: [][]string{years},
		},
		{
			Pattern:    `site:pwc.com filetype:pdf "%s" "report"`,
			Dimensions: [][]string{themes},
		},
		{
			Pattern:    `"market disruption" "white paper" filetype:pdf "%s"`,
			Dimensions: [][]string{industries},
		},
		{
			Pattern:    `"manual process" OR "data silos" "industry report" filetype:pdf "%s"`,
			Dimensions: [][]string{industries},
		},
	}
}
