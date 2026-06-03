package llm

import "context"

// Client is the provider-agnostic LLM interface.
// All calls use the single cheap model configured via OPENAI_MODEL.
type Client interface {
	// Triage decides whether a document is relevant to market disruption / pain points.
	Triage(ctx context.Context, docMeta string, sampleChunks []string) (TriageResult, error)

	// ExtractSignals pulls disruption signals from relevant text chunks.
	ExtractSignals(ctx context.Context, chunks []string, sourceURL string) ([]DisruptionSignal, error)

	// GenerateIdeas converts problem statements into SaaS idea candidates.
	// chunks are the evidence text excerpts (indexed) so the model can produce grounded citations.
	// (pre-scoring; OverallScore is always 0 here — computed deterministically later).
	GenerateIdeas(ctx context.Context, signals []DisruptionSignal, chunks []string, sourceURL string) ([]CandidateIdea, error)

	// Score fills in the 1–10 sub-scores for a candidate idea given its evidence chunks.
	Score(ctx context.Context, idea CandidateIdea, chunks []string) (SubScores, error)

	// CritiqueIdeas applies the rejection filters (docs/07 §4) to a list of ideas.
	// Returns one CritiqueResult per idea (keep=false means rejected).
	CritiqueIdeas(ctx context.Context, ideas []CandidateIdea) ([]CritiqueResult, error)

	// LabelIdea assigns exactly one idea_label from the 9-category taxonomy (docs/07 §6).
	LabelIdea(ctx context.Context, idea CandidateIdea) (string, error)

	// Embed returns vectors for a batch of texts (OpenAI text-embedding-3-small by default).
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}
