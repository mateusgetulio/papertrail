package agents

import (
	"strconv"
	"strings"

	"github.com/mateusgetulio/papertrail/internal/store"
)

// candidateFromDetail maps a stored idea detail to the analyst/report schema.
// Shared by Agent 2 (analyst output) and Agent 3 (report JSON) so the two views
// never drift.
func candidateFromDetail(d *store.IdeaDetail) RankedCandidate {
	cites := citations(d)
	return RankedCandidate{
		CandidateID:          strconv.FormatInt(d.ID, 10),
		IdeaName:             d.IdeaName,
		OneSentencePitch:     d.Pitch,
		Category:             d.Label,
		SourceDocuments:      cites,
		Industries:           d.Industries,
		CountriesOrRegions:   d.Countries,
		DisruptionDriver:     d.DisruptionDriver,
		PainPoint:            d.PainPoint,
		TargetCustomer:       d.TargetCustomer,
		BuyerPersona:         d.BuyerPersona,
		SalesMotion:          d.SalesMotion,
		HighTicketPotential:  d.HighTicketPotential,
		MassMarketPotential:  d.MassMarketPotential,
		TechnicalFeasibility: d.TechnicalFeasibility,
		MarketUrgency:        d.MarketUrgency,
		CompetitionRisk:      d.CompetitionRisk,
		DataAvailability:     d.DataAvailability,
		MVPComplexity:        d.MVPComplexity,
		OverallScore:         d.OverallScore,
		WhyItMightWork:       d.WhyWork,
		WhyItMightFail:       d.WhyFail,
		PossibleMVP:          d.PossibleMVP,
		First10Customers:     d.First10,
		ValidationQuestions:  d.ValidationQuestions,
		Citations:            cites,
	}
}

// citations returns the unique, non-empty source URLs backing an idea.
func citations(d *store.IdeaDetail) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range d.Evidence {
		u := strings.TrimSpace(e.SourceURL)
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// humanize turns an enum/driver value ("smb_saas", "demand_shift") into a
// readable label, preserving a few common acronyms.
func humanize(s string) string {
	if s == "" {
		return ""
	}
	acronyms := map[string]string{
		"saas": "SaaS", "smb": "SMB", "ai": "AI", "mvp": "MVP", "regtech": "RegTech",
	}
	words := strings.Fields(strings.ReplaceAll(s, "_", " "))
	for i := range words {
		lc := strings.ToLower(words[i])
		if ac, ok := acronyms[lc]; ok {
			words[i] = ac
		} else if i == 0 {
			words[i] = strings.ToUpper(lc[:1]) + lc[1:]
		} else {
			words[i] = lc
		}
	}
	return strings.Join(words, " ")
}
