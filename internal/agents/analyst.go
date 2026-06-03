package agents

import (
	"context"

	"github.com/mateusgetulio/papertrail/internal/analysis"
	"github.com/mateusgetulio/papertrail/internal/llm"
	"github.com/mateusgetulio/papertrail/internal/store"
)

// AnalystAgent is Agent 2: it runs the existing LLM analysis workflow over any
// freshly-extracted documents, then reads the ranked candidates back out of the
// store and maps them to the analyst schema.
type AnalystAgent struct {
	ctx     *Context
	analyse bool // when true, run analysis over extracted docs first
	limit   int
}

func NewAnalyst(ctx *Context, analyse bool, limit int) *AnalystAgent {
	return &AnalystAgent{ctx: ctx, analyse: analyse, limit: limit}
}

func (a *AnalystAgent) Run(c context.Context) OpportunityAnalystOutput {
	out := OpportunityAnalystOutput{Agent: "opportunity_analyst", RunID: a.ctx.RunID}
	var issues Issues
	cfg := a.ctx.Cfg

	if a.analyse {
		if cfg.OpenAIKey == "" {
			issues.Add("opportunity_analyst", IssueMissingEnv, "OPENAI_API_KEY not set; cannot run analysis", "set OPENAI_API_KEY in .env")
		} else {
			llmClient := llm.NewOpenAI(cfg.OpenAIKey, cfg.OpenAIModel)
			agent := analysis.NewAgent(llmClient, a.ctx.DB)
			docs, err := store.GetExtractedDocuments(c, a.ctx.DB, a.limit)
			if err != nil {
				issues.Add("opportunity_analyst", IssueDataAccess, "cannot read extracted documents: "+err.Error(), "")
			} else {
				stored := 0
				for _, doc := range docs {
					n, err := agent.Analyse(c, doc)
					if err != nil {
						issues.Add("opportunity_analyst", IssueStageFailed, "analysis failed for document", err.Error())
						continue
					}
					stored += n
				}
				a.ctx.Log.Info("analyst analysis complete", "run_id", a.ctx.RunID, "ideas_stored", stored)
			}
		}
	}

	ideas, err := store.ListIdeas(c, a.ctx.DB, store.IdeaFilter{Sort: "score"})
	if err != nil {
		issues.Add("opportunity_analyst", IssueDataAccess, "cannot list candidates: "+err.Error(), "")
		out.IssuesReported = issues.List()
		return out
	}

	totalCitations := 0
	for _, row := range ideas {
		detail, err := store.GetIdeaDetail(c, a.ctx.DB, row.ID)
		if err != nil || detail == nil {
			issues.Add("opportunity_analyst", IssueDataAccess, "cannot load candidate detail", "")
			continue
		}
		cand := candidateFromDetail(detail)
		totalCitations += len(cand.Citations)
		out.RankedCandidates = append(out.RankedCandidates, cand)
	}

	if len(out.RankedCandidates) == 0 {
		issues.Add("opportunity_analyst", IssueNoCandidates, "no SaaS opportunity candidates were produced",
			"ingest more documents and re-run analysis")
	} else if totalCitations == 0 {
		// Candidates exist but none trace to a source — unsafe to surface.
		issues.Add("opportunity_analyst", IssueMissingCitations, "candidates exist but none have citations", "")
	}

	out.IssuesReported = issues.List()
	return out
}
