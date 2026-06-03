// Package agents implements the 4-agent disruption→opportunity pipeline as a
// thin orchestration layer over the existing Paper Trail packages:
//
//	Agent 1 Document Operator  -> wraps discovery + ingest (fetch/extract/chunk)
//	Agent 2 Opportunity Analyst -> wraps analysis (LLM workflow + scoring)
//	Agent 3 Human Report        -> net-new deterministic Markdown/JSON report
//	Agent 4 Chain Quality Gate  -> net-new issue triage + continue/stop decision
//
// Each agent emits a JSON-serializable output struct matching the documented
// schema (see docs/agents.md) plus a list of reported issues. The pipeline
// orchestrator (internal/pipeline) runs them in order and lets the quality gate
// halt the chain when a blocker is detected.
package agents

// Issue codes. The quality gate maps each code to a severity class, so agents
// only need to report a code + message; the gate decides whether it blocks.
const (
	IssueMissingEnv       = "missing_env"
	IssueNoDocuments      = "no_documents_prepared"
	IssueDataAccess       = "cannot_access_prepared_data"
	IssueFetchPartial     = "some_fetches_failed"
	IssueLowExtraction    = "low_extraction_quality"
	IssueMissingPubDate   = "missing_publication_date"
	IssueDedupFailure     = "deduplication_failure"
	IssueNoCandidates     = "no_candidates_produced"
	IssueMissingCitations = "missing_citations"
	IssueInvalidScore     = "invalid_score"
	IssueDBWrite          = "db_write_failed"
	IssueReportNoSources  = "report_without_sources"
	IssueReportFormatting = "report_formatting"
	IssueStageFailed      = "stage_failed"
)

// AgentIssue is a single problem reported by an agent for the quality gate.
type AgentIssue struct {
	Code        string `json:"code"`
	SourceAgent string `json:"source_agent"`
	Message     string `json:"message"`
	Context     string `json:"context,omitempty"`
}

// --- Agent 1: Document Operator ---

type DocumentOperatorOutput struct {
	Agent             string        `json:"agent"`
	RunID             string        `json:"run_id"`
	DocumentsPrepared []PreparedDoc `json:"documents_prepared"`
	RejectedSources   []RejectedSrc `json:"rejected_sources"`
	IssuesReported    []AgentIssue  `json:"issues_reported"`
}

type PreparedDoc struct {
	DocumentID                 string   `json:"document_id"`
	Title                      string   `json:"title"`
	SourceName                 string   `json:"source_name"`
	SourceDomain               string   `json:"source_domain"`
	SourceURL                  string   `json:"source_url"`
	DocumentType               string   `json:"document_type"`
	PublicationDate            string   `json:"publication_date"`
	FetchDate                  string   `json:"fetch_date"`
	IndustriesDetected         []string `json:"industries_detected"`
	CountriesOrRegionsDetected []string `json:"countries_or_regions_detected"`
	SourceTrustScore           int      `json:"source_trust_score"`
	ComplianceStatus           string   `json:"compliance_status"`
	ComplianceNotes            string   `json:"compliance_notes"`
	TextChunksLocation         string   `json:"text_chunks_location"`
	SummaryPreview             string   `json:"summary_preview"`
	Citations                  []string `json:"citations"`
}

type RejectedSrc struct {
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

// --- Agent 2: Opportunity Analyst ---

type OpportunityAnalystOutput struct {
	Agent            string            `json:"agent"`
	RunID            string            `json:"run_id"`
	RankedCandidates []RankedCandidate `json:"ranked_candidates"`
	Discarded        []DiscardedCand   `json:"discarded_candidates"`
	IssuesReported   []AgentIssue      `json:"issues_reported"`
}

type RankedCandidate struct {
	CandidateID          string   `json:"candidate_id"`
	IdeaName             string   `json:"idea_name"`
	OneSentencePitch     string   `json:"one_sentence_pitch"`
	Category             string   `json:"category"`
	SourceDocuments      []string `json:"source_documents"`
	Industries           []string `json:"industries"`
	CountriesOrRegions   []string `json:"countries_or_regions"`
	DisruptionDriver     string   `json:"disruption_driver"`
	PainPoint            string   `json:"pain_point"`
	TargetCustomer       string   `json:"target_customer"`
	BuyerPersona         string   `json:"buyer_persona"`
	SalesMotion          string   `json:"sales_motion"`
	HighTicketPotential  int      `json:"high_ticket_potential"`
	MassMarketPotential  int      `json:"mass_market_potential"`
	TechnicalFeasibility int      `json:"technical_feasibility"`
	MarketUrgency        int      `json:"market_urgency"`
	CompetitionRisk      int      `json:"competition_risk"`
	DataAvailability     int      `json:"data_availability"`
	MVPComplexity        int      `json:"mvp_complexity"`
	OverallScore         int      `json:"overall_score"`
	WhyItMightWork       string   `json:"why_it_might_work"`
	WhyItMightFail       string   `json:"why_it_might_fail"`
	PossibleMVP          string   `json:"possible_mvp"`
	First10Customers     string   `json:"first_10_customers"`
	ValidationQuestions  []string `json:"validation_questions"`
	Citations            []string `json:"citations"`
}

type DiscardedCand struct {
	IdeaName        string `json:"idea_name"`
	ReasonDiscarded string `json:"reason_discarded"`
}

// --- Agent 3: Human Report ---

type HumanReportOutput struct {
	Agent                  string       `json:"agent"`
	RunID                  string       `json:"run_id"`
	ReportMarkdownLocation string       `json:"report_markdown_location"`
	ReportJSONLocation     string       `json:"report_json_location"`
	TopHighTicketIdeas     []string     `json:"top_high_ticket_ideas"`
	TopMassMarketIdeas     []string     `json:"top_mass_market_ideas"`
	RecommendedNextAction  string       `json:"recommended_next_action"`
	IssuesReported         []AgentIssue `json:"issues_reported"`
}

// --- Agent 4: Chain Quality Gate ---

type ChainQualityGateOutput struct {
	Agent             string        `json:"agent"`
	RunID             string        `json:"run_id"`
	Decision          string        `json:"decision"` // continue | continue_with_warnings | stop_for_fix
	Blockers          []Blocker     `json:"blockers"`
	Warnings          []Warning     `json:"warnings"`
	NiceToHaveBacklog []BacklogItem `json:"nice_to_have_backlog"`
	FinalNotes        string        `json:"final_notes"`
}

type Blocker struct {
	Issue          string `json:"issue"`
	SourceAgent    string `json:"source_agent"`
	WhyBlocking    string `json:"why_blocking"`
	RecommendedFix string `json:"recommended_fix"`
}

type Warning struct {
	Issue               string `json:"issue"`
	SourceAgent         string `json:"source_agent"`
	RecommendedFollowUp string `json:"recommended_follow_up"`
}

type BacklogItem struct {
	Issue       string `json:"issue"`
	SourceAgent string `json:"source_agent"`
	Priority    string `json:"priority"` // low | medium
}

// Decision constants.
const (
	DecisionContinue         = "continue"
	DecisionContinueWithWarn = "continue_with_warnings"
	DecisionStopForFix       = "stop_for_fix"
)
