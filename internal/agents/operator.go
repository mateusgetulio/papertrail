package agents

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mateusgetulio/papertrail/internal/compliance"
	"github.com/mateusgetulio/papertrail/internal/discovery"
	"github.com/mateusgetulio/papertrail/internal/fetch"
	"github.com/mateusgetulio/papertrail/internal/ingest"
	"github.com/mateusgetulio/papertrail/internal/llm"
	"github.com/mateusgetulio/papertrail/internal/search"
	bravepkg "github.com/mateusgetulio/papertrail/internal/search/brave"
	"github.com/mateusgetulio/papertrail/internal/store"
)

// OperatorAgent is Agent 1: it (optionally) discovers + ingests new public
// documents using the existing compliance-gated pipeline, then inventories the
// prepared documents for the analyst. It never bypasses access controls — all
// fetching goes through internal/compliance.
type OperatorAgent struct {
	ctx    *Context
	ingest bool // when true, run live discovery + ingest before inventorying
	limit  int
}

func NewOperator(ctx *Context, ingest bool, limit int) *OperatorAgent {
	return &OperatorAgent{ctx: ctx, ingest: ingest, limit: limit}
}

func (a *OperatorAgent) Run(c context.Context) DocumentOperatorOutput {
	out := DocumentOperatorOutput{Agent: "document_operator", RunID: a.ctx.RunID}
	var issues Issues

	if a.ingest {
		if a.ctx.Cfg.BraveAPIKey == "" {
			issues.Add("document_operator", IssueMissingEnv, "BRAVE_API_KEY not set; cannot run discovery", "set BRAVE_API_KEY in .env")
		} else {
			n, rejected := a.discoverAndIngest(c, &issues)
			out.RejectedSources = rejected
			a.ctx.Log.Info("operator ingest complete", "run_id", a.ctx.RunID, "ingested", n)
		}
	}

	docs, err := store.GetPreparedDocuments(c, a.ctx.DB, a.limit)
	if err != nil {
		issues.Add("document_operator", IssueDataAccess, "cannot read prepared documents: "+err.Error(), "")
		out.IssuesReported = issues.List()
		return out
	}

	missingDates := 0
	for _, d := range docs {
		pd := PreparedDoc{
			DocumentID:                 strconv.FormatInt(d.ID, 10),
			Title:                      d.Title,
			SourceName:                 d.SourceName,
			SourceDomain:               d.SourceDomain,
			SourceURL:                  d.CanonicalURL,
			DocumentType:               docType(d.ContentType),
			FetchDate:                  d.DiscoveredAt.Format("2006-01-02"),
			IndustriesDetected:         []string{},
			CountriesOrRegionsDetected: []string{},
			SourceTrustScore:           trustScore(d.TrustTier),
			ComplianceStatus:           "allowed",
			ComplianceNotes:            fmt.Sprintf("trust tier %s; passed compliance gate on ingest", d.TrustTier),
			TextChunksLocation:         fmt.Sprintf("db://document_chunk?document_id=%d (%d chunks)", d.ID, d.ChunkCount),
			Citations:                  []string{d.CanonicalURL},
		}
		if d.PublishedAt != nil {
			pd.PublicationDate = d.PublishedAt.Format("2006-01-02")
		} else {
			missingDates++
		}
		out.DocumentsPrepared = append(out.DocumentsPrepared, pd)
	}

	if len(out.DocumentsPrepared) == 0 {
		issues.Add("document_operator", IssueNoDocuments, "no prepared documents available for analysis",
			"run `papertrail pipeline --ingest` or expand the source list")
	}
	if missingDates > 0 {
		issues.Add("document_operator", IssueMissingPubDate,
			fmt.Sprintf("%d document(s) missing a publication date", missingDates), "")
	}

	out.IssuesReported = issues.List()
	return out
}

// discoverAndIngest replicates the discover→ingest flow over the existing
// compliance-gated packages and returns the count stored plus any per-candidate
// rejections.
func (a *OperatorAgent) discoverAndIngest(c context.Context, issues *Issues) (int, []RejectedSrc) {
	cfg := a.ctx.Cfg
	llmClient := llm.NewOpenAI(cfg.OpenAIKey, cfg.OpenAIModel)
	provider := bravepkg.New(cfg.BraveAPIKey)

	robots := compliance.NewRobotsCache()
	rates := compliance.NewRateLimiterRegistry()
	policies := compliance.NewPolicyRegistry()
	gate := compliance.NewGate(robots, rates, policies)
	runner := discovery.NewRunner(provider, gate)

	var expanded []string
	for _, t := range search.DefaultTemplates() {
		expanded = append(expanded, t.Expand()...)
	}

	discCtx, cancel := context.WithCancel(c)
	defer cancel()
	candidates := make(chan discovery.Candidate, 16)
	go func() {
		defer close(candidates)
		_ = runner.Run(discCtx, expanded, 3, candidates)
	}()

	fetcher := fetch.New(fetch.NewMemCache())
	pipeline := ingest.New(fetcher, gate, policies, llmClient, a.ctx.DB)

	var rejected []RejectedSrc
	ingested := 0
	for cand := range candidates {
		if ingested >= a.limit {
			cancel()
			go func() { //nolint:errcheck // drain so the producer can exit
				for range candidates {
				}
			}()
			break
		}
		stored, err := pipeline.Run(c, cand)
		if err != nil {
			rejected = append(rejected, RejectedSrc{URL: cand.CanonicalURL, Reason: err.Error()})
			issues.Add("document_operator", IssueFetchPartial, "ingest failed for "+cand.CanonicalURL, err.Error())
			continue
		}
		if stored {
			ingested++
		}
	}
	return ingested, rejected
}

// docType maps the content_type enum to the agent schema's document_type.
func docType(ct string) string {
	switch ct {
	case "pdf":
		return "pdf"
	case "html":
		return "html"
	default:
		return "other"
	}
}

// trustScore maps a trust tier (A..F) to a 1..6 numeric score.
func trustScore(tier string) int {
	switch tier {
	case "A":
		return 6
	case "B":
		return 5
	case "C":
		return 4
	case "D":
		return 3
	case "E":
		return 2
	default:
		return 1
	}
}
