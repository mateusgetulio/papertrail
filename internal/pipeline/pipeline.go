// Package pipeline orchestrates the 4-agent disruption→opportunity chain:
//
//	Document Operator → Opportunity Analyst → Human Report → Chain Quality Gate
//
// Issues reported by each agent are aggregated and evaluated by the quality
// gate. The gate runs after every stage; if it returns stop_for_fix, the
// orchestrator halts before the next stage so a blocker never produces
// downstream output. Every agent's JSON output is written under
// <ReportsDir>/<run_id>/ for auditability.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mateusgetulio/papertrail/internal/agents"
	"github.com/mateusgetulio/papertrail/internal/config"
)

// Options controls a pipeline run.
type Options struct {
	Ingest  bool // Agent 1 runs live search discovery + ingest before inventorying
	Seed    bool // Agent 1 ingests the manually-approved curated source list
	Analyse bool // Agent 2 runs the LLM analysis over freshly-extracted docs
	Limit   int  // per-stage cap (documents / candidates)
}

// Result is the full record of a pipeline run.
type Result struct {
	RunID    string
	Operator agents.DocumentOperatorOutput
	Analyst  agents.OpportunityAnalystOutput
	Report   agents.HumanReportOutput
	Gate     agents.ChainQualityGateOutput
	Stopped  bool // true if the quality gate halted the chain early
}

// Run executes the chain and returns its result.
func Run(ctx context.Context, db *pgxpool.Pool, cfg *config.Config, log *slog.Logger, opts Options) (*Result, error) {
	runID := newRunID()
	dir := filepath.Join(cfg.ReportsDir, runID)
	log = log.With("run_id", runID)
	log.Info("pipeline start", "ingest", opts.Ingest, "analyse", opts.Analyse, "limit", opts.Limit)

	actx := &agents.Context{DB: db, Cfg: cfg, RunID: runID, Log: log, ReportsDir: cfg.ReportsDir}
	res := &Result{RunID: runID}
	var all []agents.AgentIssue

	// Stage 1 — Document Operator
	res.Operator = agents.NewOperator(actx, opts.Ingest, opts.Seed, opts.Limit).Run(ctx)
	writeJSON(log, dir, "01-document-operator.json", res.Operator)
	all = append(all, res.Operator.IssuesReported...)
	if gate := agents.Decide(runID, all); gate.Decision == agents.DecisionStopForFix {
		return finalize(log, dir, res, gate, true), nil
	}

	// Stage 2 — Opportunity Analyst
	res.Analyst = agents.NewAnalyst(actx, opts.Analyse, opts.Limit).Run(ctx)
	writeJSON(log, dir, "02-opportunity-analyst.json", res.Analyst)
	all = append(all, res.Analyst.IssuesReported...)
	if gate := agents.Decide(runID, all); gate.Decision == agents.DecisionStopForFix {
		return finalize(log, dir, res, gate, true), nil
	}

	// Stage 3 — Human Report
	res.Report = agents.NewReport(actx).Run(ctx)
	writeJSON(log, dir, "03-human-report.json", res.Report)
	all = append(all, res.Report.IssuesReported...)

	// Stage 4 — Chain Quality Gate (final)
	gate := agents.Decide(runID, all)
	return finalize(log, dir, res, gate, false), nil
}

func finalize(log *slog.Logger, dir string, res *Result, gate agents.ChainQualityGateOutput, stopped bool) *Result {
	res.Gate = gate
	res.Stopped = stopped
	writeJSON(log, dir, "04-chain-quality-gate.json", gate)
	log.Info("pipeline complete",
		"decision", gate.Decision, "stopped", stopped,
		"blockers", len(gate.Blockers), "warnings", len(gate.Warnings))
	return res
}

func writeJSON(log *slog.Logger, dir, name string, v any) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Warn("cannot create run dir", "dir", dir, "err", err)
		return
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Warn("cannot marshal agent output", "file", name, "err", err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		log.Warn("cannot write agent output", "file", name, "err", err)
	}
}

func newRunID() string {
	return fmt.Sprintf("run-%s", time.Now().UTC().Format("20060102-150405.000"))
}
