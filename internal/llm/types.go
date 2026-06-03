package llm

// CandidateIdea is the structured output contract (docs/08).
// All integer sub-scores are 1–10; OverallScore is 1–100 (computed by scoring.Compute, not the model).
type CandidateIdea struct {
	IdeaName            string     `json:"idea_name"`
	OneSentencePitch    string     `json:"one_sentence_pitch"`
	SourceDocuments     []string   `json:"source_documents"`
	Industries          []string   `json:"industries"`
	CountriesOrRegions  []string   `json:"countries_or_regions"`
	DisruptionDriver    string     `json:"disruption_driver"`
	PainPoint           string     `json:"pain_point"`
	TargetCustomer      string     `json:"target_customer"`
	BuyerPersona        string     `json:"buyer_persona"`
	SalesMotion         string     `json:"sales_motion"`
	HighTicketPotential int        `json:"high_ticket_potential"`
	MassMarketPotential int        `json:"mass_market_potential"`
	TechnicalFeasibility int       `json:"technical_feasibility"`
	MarketUrgency       int        `json:"market_urgency"`
	CompetitionRisk     int        `json:"competition_risk"`
	DataAvailability    int        `json:"data_availability"`
	MVPComplexity       int        `json:"mvp_complexity"`
	OverallScore        int        `json:"overall_score"`
	WhyItMightWork      string     `json:"why_it_might_work"`
	WhyItMightFail      string     `json:"why_it_might_fail"`
	PossibleMVP         string     `json:"possible_mvp"`
	First10Customers    string     `json:"first_10_customers"`
	ValidationQuestions []string   `json:"validation_questions"`
	Citations           []Citation `json:"citations"`
}

type Citation struct {
	DocumentID  string `json:"document_id"`
	SourceURL   string `json:"source_url"`
	Title       string `json:"title"`
	Publisher   string `json:"publisher"`
	PublishedAt string `json:"published_at"`
	Excerpt     string `json:"excerpt"`
	ChunkIndex  int    `json:"chunk_index"`
}

// DisruptionSignal is extracted from a document chunk before idea generation.
type DisruptionSignal struct {
	Summary    string   `json:"summary"`
	SignalType string   `json:"signal_type"`
	PainPoint  string   `json:"pain_point"`
	Industries []string `json:"industries"`
	Regions    []string `json:"regions"`
	ChunkRefs  []int    `json:"chunk_refs"`
	Confidence float64  `json:"confidence"`
}

// TriageResult reports whether a document is worth analysing.
type TriageResult struct {
	Relevant bool   `json:"relevant"`
	Reason   string `json:"reason"`
	Topics   []string `json:"topics"`
}

// CritiqueResult is the per-idea output of the critique/reject step.
type CritiqueResult struct {
	IdeaIndex int    `json:"idea_index"`
	Keep      bool   `json:"keep"`
	Reason    string `json:"reason"`
}

// SubScores holds the raw 1–10 per-criterion values from the LLM, plus
// metadata needed for the deterministic rubric (docs/06).
type SubScores struct {
	// Criteria[0..13] map to the 14 weighted criteria in docs/06 §2.
	Criteria [14]int

	// Penalty criteria (docs/06 §4)
	GenericRisk    int
	ConsultingRisk int

	// For trust & frequency adjustments
	AvgSourceTrustWeight float64
	DistinctSourceDocs   int
}
