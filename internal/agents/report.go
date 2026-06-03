package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mateusgetulio/papertrail/internal/store"
)

// ReportAgent is Agent 3: it turns the ranked candidates into a human-readable
// Markdown report plus a machine-readable JSON, written under ReportsDir/<run>.
// It performs no LLM calls — the report is rendered deterministically from
// stored fields, so it is free, reproducible, and needs no API key.
type ReportAgent struct {
	ctx *Context
}

func NewReport(ctx *Context) *ReportAgent { return &ReportAgent{ctx: ctx} }

func (a *ReportAgent) Run(c context.Context) HumanReportOutput {
	out := HumanReportOutput{Agent: "human_report", RunID: a.ctx.RunID}
	var issues Issues

	rows, err := store.ListIdeas(c, a.ctx.DB, store.IdeaFilter{Sort: "score"})
	if err != nil {
		issues.Add("human_report", IssueDataAccess, "cannot list candidates: "+err.Error(), "")
		out.IssuesReported = issues.List()
		return out
	}
	var details []*store.IdeaDetail
	for _, r := range rows {
		d, err := store.GetIdeaDetail(c, a.ctx.DB, r.ID)
		if err == nil && d != nil {
			details = append(details, d)
		}
	}
	if len(details) == 0 {
		issues.Add("human_report", IssueNoCandidates, "no candidates available to report on", "")
		out.IssuesReported = issues.List()
		return out
	}

	withCitations := 0
	for _, d := range details {
		if len(citations(d)) > 0 {
			withCitations++
		}
	}
	if withCitations == 0 {
		issues.Add("human_report", IssueReportNoSources, "report would have no citations for any idea", "")
	}

	es := execSummary(details)
	markdown := renderMarkdown(details, es)

	dir := filepath.Join(a.ctx.ReportsDir, a.ctx.RunID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		issues.Add("human_report", IssueStageFailed, "cannot create report directory: "+err.Error(), dir)
		out.IssuesReported = issues.List()
		return out
	}
	mdPath := filepath.Join(dir, "report.md")
	jsonPath := filepath.Join(dir, "report.json")

	if err := os.WriteFile(mdPath, []byte(markdown), 0o644); err != nil {
		issues.Add("human_report", IssueStageFailed, "cannot write markdown report: "+err.Error(), mdPath)
	}

	cands := make([]RankedCandidate, 0, len(details))
	for _, d := range details {
		cands = append(cands, candidateFromDetail(d))
	}
	jsonDoc := map[string]any{
		"run_id":            a.ctx.RunID,
		"executive_summary": es,
		"opportunities":     cands,
	}
	if b, err := json.MarshalIndent(jsonDoc, "", "  "); err == nil {
		if err := os.WriteFile(jsonPath, b, 0o644); err != nil {
			issues.Add("human_report", IssueStageFailed, "cannot write json report: "+err.Error(), jsonPath)
		}
	}

	out.ReportMarkdownLocation = mdPath
	out.ReportJSONLocation = jsonPath
	out.TopHighTicketIdeas = names(es.BestHighTicket)
	out.TopMassMarketIdeas = names(es.BestMassMarket)
	out.RecommendedNextAction = es.RecommendedNextAction
	out.IssuesReported = issues.List()
	a.ctx.Log.Info("report written", "run_id", a.ctx.RunID, "markdown", mdPath, "ideas", len(details))
	return out
}

// execSummaryData holds the picks for the executive summary section.
type execSummaryData struct {
	BestHighTicket        *store.IdeaDetail
	BestMassMarket        *store.IdeaDetail
	FastestMVP            *store.IdeaDetail
	HighestRisk           *store.IdeaDetail
	MostEvidenceBacked    *store.IdeaDetail
	RecommendedNextAction string
}

func execSummary(d []*store.IdeaDetail) execSummaryData {
	es := execSummaryData{
		BestHighTicket: pick(d, func(a, b *store.IdeaDetail) bool {
			return cmp2(a.HighTicketPotential, a.OverallScore, b.HighTicketPotential, b.OverallScore)
		}),
		BestMassMarket: pick(d, func(a, b *store.IdeaDetail) bool {
			return cmp2(a.MassMarketPotential, a.OverallScore, b.MassMarketPotential, b.OverallScore)
		}),
		FastestMVP: pick(d, func(a, b *store.IdeaDetail) bool {
			return cmp2(-a.MVPComplexity, a.OverallScore, -b.MVPComplexity, b.OverallScore)
		}),
		HighestRisk:        pick(d, func(a, b *store.IdeaDetail) bool { return a.OverallScore < b.OverallScore }),
		MostEvidenceBacked: pick(d, func(a, b *store.IdeaDetail) bool { return len(a.Evidence) > len(b.Evidence) }),
	}
	if es.FastestMVP != nil {
		es.RecommendedNextAction = fmt.Sprintf(
			"Validate %q first — it has the fastest path to an MVP. Run its public validation tests before writing code.",
			es.FastestMVP.IdeaName)
	}
	return es
}

// pick returns the element for which better(candidate, current) is true.
func pick(d []*store.IdeaDetail, better func(a, b *store.IdeaDetail) bool) *store.IdeaDetail {
	var best *store.IdeaDetail
	for _, x := range d {
		if best == nil || better(x, best) {
			best = x
		}
	}
	return best
}

// cmp2 compares by a primary then secondary key (both "higher is better").
func cmp2(ap, as, bp, bs int) bool {
	if ap != bp {
		return ap > bp
	}
	return as > bs
}

func names(d *store.IdeaDetail) []string {
	if d == nil {
		return nil
	}
	return []string{d.IdeaName}
}

func renderMarkdown(d []*store.IdeaDetail, es execSummaryData) string {
	var b strings.Builder
	b.WriteString("# Market Disruption SaaS Opportunity Report\n\n")

	b.WriteString("## Executive Summary\n\n")
	b.WriteString(summaryLine("Best high-ticket opportunity", es.BestHighTicket))
	b.WriteString(summaryLine("Best mass-audience opportunity", es.BestMassMarket))
	b.WriteString(summaryLine("Fastest MVP", es.FastestMVP))
	b.WriteString(summaryLine("Highest risk idea", es.HighestRisk))
	b.WriteString(summaryLine("Most evidence-backed idea", es.MostEvidenceBacked))
	b.WriteString("\n")

	b.WriteString("## Ranked Opportunities\n\n")
	for i, x := range d {
		fmt.Fprintf(&b, "### %d. %s  ·  score %d/100\n\n", i+1, x.IdeaName, x.OverallScore)
		field(&b, "Disruption", humanize(x.DisruptionDriver))
		field(&b, "SaaS idea", x.Pitch)
		field(&b, "Target audience", x.TargetCustomer)
		field(&b, "Buyer persona", x.BuyerPersona)
		field(&b, "Market reach", strings.Join(append(append([]string{}, x.Industries...), x.Countries...), ", "))
		field(&b, "High-ticket vs mass-market", fmt.Sprintf("high-ticket %d/10 · mass-market %d/10", x.HighTicketPotential, x.MassMarketPotential))
		field(&b, "Why now", fmt.Sprintf("%s (market urgency %d/10)", humanize(x.DisruptionDriver), x.MarketUrgency))
		field(&b, "MVP version", x.PossibleMVP)
		field(&b, "Implementation difficulty", fmt.Sprintf("complexity %d/10 · technical feasibility %d/10", x.MVPComplexity, x.TechnicalFeasibility))
		field(&b, "Estimated build cost", costRange(x.MVPComplexity))
		field(&b, "Data / integration needs", fmt.Sprintf("data availability %d/10", x.DataAvailability))
		if len(x.ValidationQuestions) > 0 {
			b.WriteString("- **Public validation tests:**\n")
			for _, q := range x.ValidationQuestions {
				fmt.Fprintf(&b, "  - %s\n", q)
			}
		}
		field(&b, "First 10 customer channels", x.First10)
		field(&b, "Risks / why it might fail", fmt.Sprintf("%s (competition risk %d/10)", x.WhyFail, x.CompetitionRisk))
		cs := citations(x)
		if len(cs) == 0 {
			b.WriteString("- **Evidence:** ⚠️ no citations recorded — treat as a weak/unverified lead.\n")
		} else {
			b.WriteString("- **Evidence and citations:**\n")
			for _, e := range x.Evidence {
				if strings.TrimSpace(e.SourceURL) == "" {
					continue
				}
				fmt.Fprintf(&b, "  - %s\n", e.SourceURL)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("## Recommended Next Actions\n\n")
	if es.FastestMVP != nil {
		fmt.Fprintf(&b, "- **Validate first:** %s (fastest MVP path).\n", es.FastestMVP.IdeaName)
	}
	if es.HighestRisk != nil {
		fmt.Fprintf(&b, "- **Do not build yet:** %s (lowest score / highest risk) until evidence strengthens.\n", es.HighestRisk.IdeaName)
	}
	b.WriteString("- **Needs more research:** any idea above with no or thin citations.\n")
	if es.BestHighTicket != nil {
		fmt.Fprintf(&b, "- **Fastest to paid MVP:** %s shows the strongest high-ticket potential.\n", es.BestHighTicket.IdeaName)
	}
	return b.String()
}

func summaryLine(label string, d *store.IdeaDetail) string {
	if d == nil {
		return fmt.Sprintf("- **%s:** —\n", label)
	}
	return fmt.Sprintf("- **%s:** %s (score %d/100)\n", label, d.IdeaName, d.OverallScore)
}

func field(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(b, "- **%s:** %s\n", label, value)
}

// costRange is a rough build-cost band derived from MVP complexity (1..10).
func costRange(complexity int) string {
	switch {
	case complexity <= 0:
		return "unknown"
	case complexity <= 3:
		return "$5k–$20k (simple MVP)"
	case complexity <= 6:
		return "$20k–$75k (moderate build)"
	default:
		return "$75k–$250k+ (heavy build)"
	}
}
