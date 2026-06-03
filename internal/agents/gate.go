package agents

// Agent 4 — Chain Quality Gate.
//
// Reviews the issues reported by the other agents, classifies each one, and
// decides whether the pipeline should continue, continue with warnings, or stop
// for a fix. The gate is the authority on severity: agents only report a code,
// and the gate maps it to a class via issueClass below.

// issueClass maps an issue code to one of: blocker, warning, nice_to_have.
// Unknown codes default to "warning" (conservative — surfaced but not fatal).
var issueClass = map[string]string{
	IssueMissingEnv:       "blocker",
	IssueNoDocuments:      "blocker",
	IssueDataAccess:       "blocker",
	IssueNoCandidates:     "blocker",
	IssueMissingCitations: "blocker",
	IssueInvalidScore:     "blocker",
	IssueDBWrite:          "blocker",
	IssueReportNoSources:  "blocker",
	IssueDedupFailure:     "blocker",
	IssueStageFailed:      "blocker",

	IssueFetchPartial:  "warning",
	IssueLowExtraction: "warning",

	IssueMissingPubDate:   "nice_to_have",
	IssueReportFormatting: "nice_to_have",
}

// whyBlocking gives a short rationale per blocking code for the fix request.
var whyBlocking = map[string]string{
	IssueMissingEnv:       "the pipeline cannot run without required configuration",
	IssueNoDocuments:      "downstream analysis has nothing to work with",
	IssueDataAccess:       "an agent cannot read the data a previous stage produced",
	IssueNoCandidates:     "there are no opportunities to report on",
	IssueMissingCitations: "ungrounded output is unsafe — every signal must trace to a source",
	IssueInvalidScore:     "ranking is meaningless without valid scores",
	IssueDBWrite:          "results are not being persisted",
	IssueReportNoSources:  "a report without citations cannot be trusted or published",
	IssueDedupFailure:     "duplicate documents would distort the ranking",
	IssueStageFailed:      "a pipeline stage failed to complete",
}

// recommendedFix gives a short remediation hint per blocking code.
var recommendedFix = map[string]string{
	IssueMissingEnv:       "set the required environment variables (see .env.example) and re-run",
	IssueNoDocuments:      "run discovery/ingest (pipeline --ingest) or widen the source list in internal/config/sources",
	IssueDataAccess:       "verify the database is reachable and migrations are applied",
	IssueNoCandidates:     "ingest more documents, then re-run analysis",
	IssueMissingCitations: "re-run analysis; ensure evidence rows are written for each idea",
	IssueInvalidScore:     "check the scoring rubric and the analysis output contract",
	IssueDBWrite:          "check DATABASE_URL, connectivity, and schema migrations",
	IssueReportNoSources:  "regenerate the report once evidence/citations are present",
	IssueDedupFailure:     "inspect the URL/embedding dedup thresholds",
	IssueStageFailed:      "see the issue context and logs for the failing stage",
}

// Decide runs the quality gate over the aggregated issues and returns the
// decision output. It is a pure function — easy to unit-test with fixtures.
func Decide(runID string, issues []AgentIssue) ChainQualityGateOutput {
	out := ChainQualityGateOutput{
		Agent: "chain_quality_gate",
		RunID: runID,
	}
	for _, is := range issues {
		switch issueClass[is.Code] {
		case "blocker":
			out.Blockers = append(out.Blockers, Blocker{
				Issue:          is.Message,
				SourceAgent:    is.SourceAgent,
				WhyBlocking:    orDefault(whyBlocking[is.Code], "blocks reliable output"),
				RecommendedFix: orDefault(recommendedFix[is.Code], "investigate and resolve before re-running"),
			})
		case "nice_to_have":
			out.NiceToHaveBacklog = append(out.NiceToHaveBacklog, BacklogItem{
				Issue:       is.Message,
				SourceAgent: is.SourceAgent,
				Priority:    "low",
			})
		default: // warning
			out.Warnings = append(out.Warnings, Warning{
				Issue:               is.Message,
				SourceAgent:         is.SourceAgent,
				RecommendedFollowUp: "review after the run; not fatal",
			})
		}
	}

	switch {
	case len(out.Blockers) > 0:
		out.Decision = DecisionStopForFix
		out.FinalNotes = "Blocking issues detected — fix and re-run before trusting output."
	case len(out.Warnings) > 0:
		out.Decision = DecisionContinueWithWarn
		out.FinalNotes = "Completed with non-blocking warnings."
	default:
		out.Decision = DecisionContinue
		out.FinalNotes = "No issues detected."
	}
	return out
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
