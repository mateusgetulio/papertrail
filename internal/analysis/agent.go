package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mateusgetulio/papertrail/internal/llm"
	"github.com/mateusgetulio/papertrail/internal/scoring"
	"github.com/mateusgetulio/papertrail/internal/store"
)

// Agent runs the full 6-step LLM analysis workflow for a single document.
type Agent struct {
	llm llm.Client
	db  *pgxpool.Pool
}

func NewAgent(llmClient llm.Client, db *pgxpool.Pool) *Agent {
	return &Agent{llm: llmClient, db: db}
}

// Analyse processes one extracted document through the full pipeline.
// Returns (ideasStored, error).
func (a *Agent) Analyse(ctx context.Context, doc store.DocRecord) (int, error) {
	log := slog.With("doc_id", doc.ID, "url", doc.CanonicalURL)

	// Load chunks
	chunks, err := store.GetChunksForDocument(ctx, a.db, doc.ID)
	if err != nil {
		return 0, fmt.Errorf("load chunks: %w", err)
	}
	if len(chunks) == 0 {
		log.Warn("no chunks, skipping")
		return 0, nil
	}
	excerpts := make([]string, len(chunks))
	for i, ch := range chunks {
		excerpts[i] = ch.Excerpt
	}
	log.Info("loaded chunks", "count", len(chunks))

	// Step 1: Triage. Sample chunks spread across the whole document rather
	// than just the first few — for long reports the opening pages are cover,
	// table of contents, and foreword, which made genuine opportunities get
	// rejected as "too vague".
	sample := spreadSample(excerpts, 8)
	triage, err := a.llm.Triage(ctx, fmt.Sprintf("url=%s title=%s", doc.CanonicalURL, doc.Title), sample)
	if err != nil {
		return 0, fmt.Errorf("triage: %w", err)
	}
	log.Info("triage", "relevant", triage.Relevant, "reason", triage.Reason)

	if !triage.Relevant {
		if err := store.UpdateDocumentStateAnalysed(ctx, a.db, doc.ID, "analyzed"); err != nil {
			log.Warn("state update failed", "err", err)
		}
		return 0, nil
	}

	// Step 2: Extract signals
	// Feed chunks in batches of 20 to avoid token limits
	var allSignals []llm.DisruptionSignal
	for start := 0; start < len(excerpts); start += 20 {
		end := start + 20
		if end > len(excerpts) {
			end = len(excerpts)
		}
		batch := excerpts[start:end]
		sigs, err := a.llm.ExtractSignals(ctx, batch, doc.CanonicalURL)
		if err != nil {
			log.Warn("signal extraction failed", "batch_start", start, "err", err)
			continue
		}
		// Adjust chunk_refs to global index
		for i := range sigs {
			for j := range sigs[i].ChunkRefs {
				sigs[i].ChunkRefs[j] += start
			}
		}
		allSignals = append(allSignals, sigs...)
	}
	log.Info("signals extracted", "count", len(allSignals))

	if len(allSignals) == 0 {
		_ = store.UpdateDocumentStateAnalysed(ctx, a.db, doc.ID, "analyzed")
		return 0, nil
	}

	// Persist signals
	for _, sig := range allSignals {
		// Embed signal summary
		embeds, _ := a.llm.Embed(ctx, []string{sig.Summary})
		var emb []float32
		if len(embeds) > 0 {
			emb = embeds[0]
		}
		_, err := store.InsertDisruptionSignal(ctx, a.db,
			doc.ID, sig.Summary, sig.SignalType, sig.PainPoint,
			sig.Industries, sig.Regions, sig.ChunkRefs,
			sig.Confidence, emb,
		)
		if err != nil {
			log.Warn("signal insert failed", "err", err)
		}
	}

	// Step 3: Generate ideas — pass all excerpts so the model can produce grounded citations
	ideas, err := a.llm.GenerateIdeas(ctx, allSignals, excerpts, doc.CanonicalURL)
	if err != nil {
		return 0, fmt.Errorf("generate ideas: %w", err)
	}
	log.Info("ideas generated", "count", len(ideas))

	if len(ideas) == 0 {
		_ = store.UpdateDocumentStateAnalysed(ctx, a.db, doc.ID, "analyzed")
		return 0, nil
	}

	// Step 4: Critique / reject
	critiques, err := a.llm.CritiqueIdeas(ctx, ideas)
	if err != nil {
		log.Warn("critique failed, keeping all ideas", "err", err)
		// Fail open: keep all ideas if critique fails
		critiques = make([]llm.CritiqueResult, len(ideas))
		for i := range critiques {
			critiques[i] = llm.CritiqueResult{IdeaIndex: i, Keep: true, Reason: "critique unavailable"}
		}
	}
	kept := filterKept(ideas, critiques)
	log.Info("after critique", "kept", len(kept), "rejected", len(ideas)-len(kept))

	// Load existing idea embeddings for dedup
	existing, err := store.GetExistingIdeaEmbeddings(ctx, a.db, 500)
	if err != nil {
		log.Warn("failed to load existing embeddings", "err", err)
	}

	stored := 0
	for _, idea := range kept {
		ideaLog := log.With("idea", idea.IdeaName)

		// Step 5a: Score (LLM sub-scores on evidence chunks)
		evidenceChunks := resolveChunkExcerpts(excerpts, idea)
		sub, err := a.llm.Score(ctx, idea, evidenceChunks)
		if err != nil {
			ideaLog.Warn("scoring failed", "err", err)
			// Use sub-scores already in idea struct if available
			sub = subScoresFromIdea(idea)
		}
		sub.AvgSourceTrustWeight = trustWeight(doc.TrustTier)
		sub.DistinctSourceDocs = 1

		// Step 5b: Deterministic overall score
		overallScore := scoring.Compute(sub)

		// Step 5c: Label
		label, err := a.llm.LabelIdea(ctx, idea)
		if err != nil {
			ideaLog.Warn("labeling failed", "err", err)
			label = "not_suitable"
		}

		// Step 5d: Embed idea pitch for dedup
		embeds, err := a.llm.Embed(ctx, []string{idea.OneSentencePitch})
		if err != nil {
			ideaLog.Warn("idea embed failed", "err", err)
		}
		var ideaEmb []float32
		if len(embeds) > 0 {
			ideaEmb = embeds[0]
		}

		// Step 5e: Dedup check
		isDup := false
		for _, ex := range existing {
			if IsDuplicate(ideaEmb, ex.Embedding, DedupThreshold) {
				isDup = true
				ideaLog.Info("duplicate idea, skipping", "similar_to", ex.ID)
				break
			}
		}
		if isDup {
			continue
		}

		// Step 5f: Persist idea
		salesMotion := normaliseSalesMotion(idea.SalesMotion)
		ideaID, err := store.InsertSaaSIdeaCandidate(ctx, a.db,
			idea.IdeaName, idea.OneSentencePitch, idea.DisruptionDriver,
			idea.PainPoint, idea.TargetCustomer, idea.BuyerPersona,
			salesMotion, label,
			map[string]int{
				"high_ticket_potential": idea.HighTicketPotential,
				"mass_market_potential": idea.MassMarketPotential,
				"technical_feasibility": idea.TechnicalFeasibility,
				"market_urgency":        idea.MarketUrgency,
				"competition_risk":      idea.CompetitionRisk,
				"data_availability":     idea.DataAvailability,
				"mvp_complexity":        idea.MVPComplexity,
			},
			idea.WhyItMightWork, idea.WhyItMightFail,
			idea.PossibleMVP, idea.First10Customers,
			idea.ValidationQuestions,
			idea.Industries, idea.CountriesOrRegions,
			ideaEmb,
		)
		if err != nil {
			ideaLog.Warn("insert idea failed", "err", err)
			continue
		}

		_ = store.InsertIdeaSourceDoc(ctx, a.db, ideaID, doc.ID)

		// Persist evidence/citations
		for _, cit := range idea.Citations {
			var chunkID *int64
			if cit.ChunkIndex >= 0 && cit.ChunkIndex < len(chunks) {
				id := chunks[cit.ChunkIndex].ID
				chunkID = &id
			}
			_ = store.InsertEvidence(ctx, a.db,
				ideaID, doc.ID, chunkID,
				cit.Excerpt, cit.Excerpt, doc.CanonicalURL,
			)
		}

		// Persist ranking score
		components := map[string]any{
			"criteria":        sub.Criteria,
			"generic_risk":    sub.GenericRisk,
			"consulting_risk": sub.ConsultingRisk,
			"trust_weight":    sub.AvgSourceTrustWeight,
		}
		_ = store.InsertRankingScore(ctx, a.db, ideaID, overallScore, components, 1, sub.AvgSourceTrustWeight)

		// Initial review status
		_ = store.InsertReviewStatus(ctx, a.db, ideaID)

		// Add to dedup set so later ideas in the same doc are also checked
		existing = append(existing, struct {
			ID        int64
			Embedding []float32
		}{ID: ideaID, Embedding: ideaEmb})

		ideaLog.Info("stored idea", "id", ideaID, "score", overallScore, "label", label)
		stored++
	}

	_ = store.UpdateDocumentStateAnalysed(ctx, a.db, doc.ID, "analyzed")
	log.Info("analysis complete", "ideas_stored", stored)
	return stored, nil
}

// filterKept returns only ideas with keep=true from the critique step.
// spreadSample picks up to k excerpts evenly spread across the slice, so triage
// sees the whole document instead of only its opening pages.
func spreadSample(excerpts []string, k int) []string {
	if len(excerpts) <= k {
		return excerpts
	}
	out := make([]string, 0, k)
	step := float64(len(excerpts)) / float64(k)
	for i := 0; i < k; i++ {
		out = append(out, excerpts[int(float64(i)*step)])
	}
	return out
}

func filterKept(ideas []llm.CandidateIdea, critiques []llm.CritiqueResult) []llm.CandidateIdea {
	keepSet := make(map[int]bool)
	for _, c := range critiques {
		if c.Keep {
			keepSet[c.IdeaIndex] = true
		}
	}
	// If critique returned nothing useful, keep all
	if len(keepSet) == 0 {
		return ideas
	}
	var out []llm.CandidateIdea
	for i, idea := range ideas {
		if keepSet[i] {
			out = append(out, idea)
		}
	}
	return out
}

// resolveChunkExcerpts returns the excerpts for chunks referenced in an idea's citations.
func resolveChunkExcerpts(excerpts []string, idea llm.CandidateIdea) []string {
	seen := map[int]struct{}{}
	var out []string
	for _, cit := range idea.Citations {
		if cit.ChunkIndex >= 0 && cit.ChunkIndex < len(excerpts) {
			if _, ok := seen[cit.ChunkIndex]; !ok {
				out = append(out, excerpts[cit.ChunkIndex])
				seen[cit.ChunkIndex] = struct{}{}
			}
		}
	}
	if len(out) == 0 {
		// Fallback: first 3 excerpts
		n := 3
		if len(excerpts) < n {
			n = len(excerpts)
		}
		out = excerpts[:n]
	}
	return out
}

// subScoresFromIdea fills SubScores from the idea's pre-set sub-score fields.
func subScoresFromIdea(idea llm.CandidateIdea) llm.SubScores {
	return llm.SubScores{
		Criteria: [14]int{
			1, idea.MarketUrgency, idea.HighTicketPotential, 5,
			1, idea.CompetitionRisk, 5, idea.MVPComplexity,
			idea.DataAvailability, 5, 5, idea.HighTicketPotential,
			idea.MVPComplexity, 5,
		},
		GenericRisk:    3,
		ConsultingRisk: 3,
	}
}

func trustWeight(tier string) float64 {
	switch strings.ToUpper(tier) {
	case "A":
		return 1.0
	case "B":
		return 0.9
	case "C":
		return 0.8
	case "D":
		return 0.7
	default:
		return 0.6
	}
}

// normaliseSalesMotion maps LLM free-text sales_motion values to the DB enum.
func normaliseSalesMotion(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.Contains(s, "enterprise"):
		return "enterprise"
	case strings.Contains(s, "smb"), strings.Contains(s, "self-serve"), strings.Contains(s, "self serve"):
		return "SMB"
	case strings.Contains(s, "market"):
		return "marketplace"
	case strings.Contains(s, "developer"), strings.Contains(s, "dev"):
		return "developer-led"
	default:
		return "mass-market"
	}
}
