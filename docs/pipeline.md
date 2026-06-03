# Pipeline

The orchestrator ([`internal/pipeline`](../internal/pipeline)) runs the four
agents in order and lets the quality gate stop the chain on a blocker:

```
Document Operator → Opportunity Analyst → Human Report → Chain Quality Gate
```

## Run it locally

```bash
# Default: no live web/LLM calls. Inventories existing documents, reports on the
# candidates already in the DB, and runs the quality gate. Free.
go run ./cmd/papertrail pipeline --limit 10

# Grow the corpus: Agent 1 does live, compliance-gated discovery + ingest
# (uses Brave + OpenAI), Agent 2 analyses the new documents (uses OpenAI).
go run ./cmd/papertrail pipeline --ingest --analyse --limit 20
```

Flags: `--ingest` (live discovery+ingest in Agent 1), `--analyse` (LLM analysis
in Agent 2), `--limit N` (per-stage cap).

## Run IDs, logs, outputs
Every run gets a `run_id` (e.g. `run-20260603-150405.123`). All logs are tagged
with it, and every agent's output is written to:

```
<REPORTS_DIR>/<run_id>/
  01-document-operator.json
  02-opportunity-analyst.json
  03-human-report.json
  04-chain-quality-gate.json
  report.md          # human-readable report (Agent 3)
  report.json        # machine-readable report (Agent 3)
```

`REPORTS_DIR` defaults to `generated_reports/` and is **gitignored**.

## How blockers stop the chain
After each stage the orchestrator runs the quality gate over the issues so far.
If the decision is `stop_for_fix`, it halts **before** the next stage, writes the
gate output, and exits — so a blocker never produces downstream output (e.g. an
empty or uncited report). The final gate decision is always written to
`04-chain-quality-gate.json`.

Decisions:
- `continue` — no issues.
- `continue_with_warnings` — non-blocking issues; output is usable, follow up later.
- `stop_for_fix` — at least one blocker; fix and re-run.

## Testing without live web access
The new logic is covered by unit tests with in-memory fixtures (no DB or network):

```bash
go test ./internal/agents/
```

`gate_test.go` covers the decision matrix; `report_test.go` covers the executive
summary picks, Markdown rendering, citation handling, and field mapping.

## Adding new sources
Discovery uses search templates in
[`internal/search/queries.go`](../internal/search/queries.go) and per-domain
policies in [`internal/compliance`](../internal/compliance). To widen coverage,
add query templates and/or approved source policies there — Agent 1 picks them
up automatically. See [compliance.md](compliance.md) for what is allowed.
