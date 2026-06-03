package agents

import (
	"strings"
	"testing"

	"github.com/mateusgetulio/papertrail/internal/store"
)

// fixtureDetails returns mock ideas so the report logic can be tested without a
// database or live web access.
func fixtureDetails() []*store.IdeaDetail {
	a := &store.IdeaDetail{
		IdeaRow: store.IdeaRow{
			ID: 1, IdeaName: "Alpha Platform", Pitch: "A vertical SaaS for X.",
			Label: "vertical_saas", OverallScore: 70, PainPoint: "pain a",
			Industries: []string{"Healthcare"},
		},
		DisruptionDriver: "demand_shift",
		WhyFail:          "competition", PossibleMVP: "mvp a", First10: "hospitals",
		HighTicketPotential: 9, MassMarketPotential: 2, MVPComplexity: 3,
		Evidence: []store.Evidence{{SourceURL: "https://example.org/a"}, {SourceURL: "https://example.org/a"}},
	}
	b := &store.IdeaDetail{
		IdeaRow: store.IdeaRow{
			ID: 2, IdeaName: "Beta Tool", Pitch: "A mass-market tool.",
			Label: "mass_market", OverallScore: 40,
		},
		DisruptionDriver:    "tech_shift",
		HighTicketPotential: 3, MassMarketPotential: 8, MVPComplexity: 8,
		Evidence: nil, // no citations
	}
	return []*store.IdeaDetail{a, b}
}

func TestExecSummaryPicks(t *testing.T) {
	es := execSummary(fixtureDetails())
	if es.BestHighTicket.ID != 1 {
		t.Errorf("best high-ticket should be idea 1, got %d", es.BestHighTicket.ID)
	}
	if es.BestMassMarket.ID != 2 {
		t.Errorf("best mass-market should be idea 2, got %d", es.BestMassMarket.ID)
	}
	if es.FastestMVP.ID != 1 { // lower MVPComplexity = faster
		t.Errorf("fastest MVP should be idea 1, got %d", es.FastestMVP.ID)
	}
	if es.HighestRisk.ID != 2 { // lowest score
		t.Errorf("highest risk should be idea 2, got %d", es.HighestRisk.ID)
	}
	if es.MostEvidenceBacked.ID != 1 {
		t.Errorf("most evidence-backed should be idea 1, got %d", es.MostEvidenceBacked.ID)
	}
}

func TestRenderMarkdownHasSections(t *testing.T) {
	d := fixtureDetails()
	md := renderMarkdown(d, execSummary(d))
	for _, want := range []string{
		"# Market Disruption SaaS Opportunity Report",
		"## Executive Summary",
		"## Ranked Opportunities",
		"## Recommended Next Actions",
		"Alpha Platform",
		"Beta Tool",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("report markdown missing %q", want)
		}
	}
	// Idea with no citations must be flagged.
	if !strings.Contains(md, "no citations recorded") {
		t.Errorf("expected a weak-evidence warning for the uncited idea")
	}
}

func TestCandidateFromDetailDedupesCitations(t *testing.T) {
	c := candidateFromDetail(fixtureDetails()[0])
	if len(c.Citations) != 1 {
		t.Errorf("duplicate source URLs should dedupe to 1, got %d", len(c.Citations))
	}
	if c.Category != "vertical_saas" || c.OverallScore != 70 {
		t.Errorf("unexpected mapping: %+v", c)
	}
}

func TestHumanize(t *testing.T) {
	cases := map[string]string{
		"smb_saas":     "SMB SaaS",
		"demand_shift": "Demand shift",
		"ai":           "AI",
		"":             "",
	}
	for in, want := range cases {
		if got := humanize(in); got != want {
			t.Errorf("humanize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCostRangeBands(t *testing.T) {
	if !strings.Contains(costRange(2), "5k") {
		t.Errorf("low complexity should be the cheap band")
	}
	if !strings.Contains(costRange(9), "250k") {
		t.Errorf("high complexity should be the expensive band")
	}
}
