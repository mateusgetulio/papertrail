# Agents

Paper Trail's analysis runs as a 4-agent chain. The agents are a thin Go
orchestration layer over the existing packages — Agents 1 and 2 **wrap** the
discovery/ingest/analysis code that already powers the `discover`, `ingest`, and
`analyse` CLI commands; Agents 3 and 4 are new.

Code lives in [`internal/agents`](../internal/agents); the orchestrator is in
[`internal/pipeline`](../internal/pipeline).

| # | Agent | Package role | Wraps / new |
|---|-------|--------------|-------------|
| 1 | Document Operator | discover → fetch → extract → chunk → inventory | wraps `discovery`, `compliance`, `fetch`, `ingest` |
| 2 | Opportunity Analyst | LLM workflow → score → rank | wraps `analysis`, `scoring` |
| 3 | Human Report | deterministic Markdown + JSON report | new |
| 4 | Chain Quality Gate | issue triage → continue/stop decision | new |

## Agent 1 — Document Operator (`operator.go`)
Optionally runs live, compliance-gated discovery + ingestion (`--ingest`), then
inventories every prepared document (state `extracted`/`analyzed`) and emits
`DocumentOperatorOutput`. All fetching goes through `internal/compliance` — it
never bypasses robots.txt, paywalls, or ToS. Reports issues such as
`no_documents_prepared`, `some_fetches_failed`, `missing_publication_date`.

## Agent 2 — Opportunity Analyst (`analyst.go`)
Optionally runs the existing LLM analysis (`--analyse`) over freshly-extracted
documents, then reads the ranked candidates back from the store and maps them to
`OpportunityAnalystOutput`. Reports `no_candidates_produced`,
`missing_citations`, `cannot_access_prepared_data`.

## Agent 3 — Human Report (`report.go`)
Reads the ranked candidates and produces a human-readable `report.md` plus a
machine-readable `report.json` under `<REPORTS_DIR>/<run_id>/`. **No LLM call** —
the report is rendered deterministically from stored fields, so it is free,
reproducible, and needs no API key. Builds the executive summary (best
high-ticket, best mass-market, fastest MVP, highest risk, most evidence-backed),
the 16-field per-opportunity sections, and recommended next actions.

## Agent 4 — Chain Quality Gate (`gate.go`)
Pure function `Decide(runID, issues)` that classifies every reported issue as a
**blocker**, **warning**, or **nice-to-have** (see `issueClass`), then returns a
decision: `continue`, `continue_with_warnings`, or `stop_for_fix`. The gate — not
the reporting agent — is the authority on severity.

## How agents pass data
Agents do **not** pass large payloads between each other in memory; they share
the Postgres database (the existing source of truth). Each agent returns a small,
JSON-serializable output struct (the schemas in `types.go`) plus a list of
reported issues. The orchestrator writes each output to
`<REPORTS_DIR>/<run_id>/0N-<agent>.json` and aggregates the issues for the gate.

## How issues are reported
Each agent embeds an `Issues` collector (`issues.go`) and calls
`Add(agent, code, message, context)`. Issue **codes** are constants in
`types.go`; the gate maps each code to a severity. Adding a new issue type =
add a constant + an entry in `gate.go`'s `issueClass` map.
