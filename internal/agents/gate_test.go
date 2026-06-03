package agents

import "testing"

func TestDecide_NoIssuesContinues(t *testing.T) {
	out := Decide("run-1", nil)
	if out.Decision != DecisionContinue {
		t.Fatalf("want %s, got %s", DecisionContinue, out.Decision)
	}
	if len(out.Blockers)+len(out.Warnings)+len(out.NiceToHaveBacklog) != 0 {
		t.Fatalf("expected no classified issues")
	}
}

func TestDecide_WarningContinuesWithWarnings(t *testing.T) {
	issues := []AgentIssue{{Code: IssueFetchPartial, SourceAgent: "document_operator", Message: "2 pdfs failed"}}
	out := Decide("r", issues)
	if out.Decision != DecisionContinueWithWarn {
		t.Fatalf("want %s, got %s", DecisionContinueWithWarn, out.Decision)
	}
	if len(out.Warnings) != 1 {
		t.Fatalf("want 1 warning, got %d", len(out.Warnings))
	}
}

func TestDecide_BlockerStops(t *testing.T) {
	issues := []AgentIssue{{Code: IssueMissingCitations, SourceAgent: "opportunity_analyst", Message: "no citations"}}
	out := Decide("r", issues)
	if out.Decision != DecisionStopForFix {
		t.Fatalf("want %s, got %s", DecisionStopForFix, out.Decision)
	}
	if len(out.Blockers) != 1 || out.Blockers[0].RecommendedFix == "" {
		t.Fatalf("expected one blocker with a recommended fix, got %+v", out.Blockers)
	}
}

func TestDecide_NiceToHaveIsBacklogNotWarning(t *testing.T) {
	issues := []AgentIssue{{Code: IssueMissingPubDate, SourceAgent: "document_operator", Message: "missing dates"}}
	out := Decide("r", issues)
	if out.Decision != DecisionContinue {
		t.Fatalf("nice-to-have should still continue, got %s", out.Decision)
	}
	if len(out.NiceToHaveBacklog) != 1 || len(out.Warnings) != 0 {
		t.Fatalf("expected backlog item, no warning; got %+v", out)
	}
}

func TestDecide_BlockerBeatsWarning(t *testing.T) {
	issues := []AgentIssue{
		{Code: IssueFetchPartial, SourceAgent: "document_operator", Message: "warn"},
		{Code: IssueNoCandidates, SourceAgent: "opportunity_analyst", Message: "block"},
	}
	out := Decide("r", issues)
	if out.Decision != DecisionStopForFix {
		t.Fatalf("a blocker must override a warning, got %s", out.Decision)
	}
}

func TestDecide_UnknownCodeDefaultsToWarning(t *testing.T) {
	out := Decide("r", []AgentIssue{{Code: "totally_unknown", SourceAgent: "x", Message: "?"}})
	if out.Decision != DecisionContinueWithWarn || len(out.Warnings) != 1 {
		t.Fatalf("unknown codes should be non-fatal warnings, got %+v", out)
	}
}
