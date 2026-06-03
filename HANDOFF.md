# Paper Trail — Handoff to Claude Code

## Project overview
**Paper Trail** is a private, internal Go CLI tool that:
1. **Discovers** public research PDFs/HTML (World Bank, OECD, WEF, etc.) via Brave Search
2. **Ingests** them (fetch → gate → detect → extract → chunk → embed)
3. **Analyses** them with an LLM pipeline (triage → signals → ideas → critique → score → store)
4. **Presents** scored SaaS idea candidates in a review dashboard (Phase 4 — **not yet built**)

---

## Current state (Phases 0–3 complete, builds clean)

```
go build ./...   ✓ (no errors)
```

### CLI subcommands
```bash
go run ./cmd/papertrail discover --limit 5
go run ./cmd/papertrail ingest   --limit 3
go run ./cmd/papertrail analyse  --limit 10
```

### DB state (Postgres 15 + pgvector)
- 4 documents ingested (state: `analyzed`)
- 2 SaaS idea candidates stored with scores + evidence + review_status

```
id=1  AI-Powered Health Data Management Platform  label=vertical_saas  score=55
id=2  Telemedicine Training Platform              label=vertical_saas  score=41
```

---

## Repo layout

```
/Users/mateusvieira/papertrail/
├── cmd/papertrail/main.go          # CLI entry point (spike|discover|ingest|analyse)
├── docs/
│   ├── 04-architecture.md          # System design
│   ├── 06-scoring-rubric.md        # Deterministic scoring weights
│   ├── 07-agent-workflow.md        # LLM 6-step workflow spec
│   ├── 08-output-format.md         # JSON output contract
│   └── 09-mvp-backlog.md           # Phase checklist
├── internal/
│   ├── analysis/
│   │   ├── agent.go                # 6-step LLM workflow per document
│   │   └── cluster.go             # Cosine similarity dedup (threshold 0.86)
│   ├── chunk/chunk.go             # Text chunker
│   ├── compliance/                # robots.txt + rate limiter + policy registry
│   ├── config/config.go           # Loads .env (DATABASE_URL, OPENAI_KEY, etc.)
│   ├── discovery/                 # Brave search + URL dedup + candidate runner
│   ├── extract/pdf.go             # pdftotext bridge
│   ├── fetch/                     # HTTP fetcher + content-type detect + gate signals
│   ├── ingest/pipeline.go         # Ingestion orchestrator (fetch→embed→store)
│   ├── llm/
│   │   ├── client.go              # LLM interface
│   │   ├── openai.go              # OpenAI impl (gpt-4o-mini)
│   │   └── types.go               # CandidateIdea, DisruptionSignal, etc.
│   ├── metadata/                  # PDF info + HTML goquery extraction
│   ├── scoring/rubric.go          # Deterministic weighted scoring (1-100)
│   ├── search/                    # SearchProvider interface + Brave impl + queries
│   └── store/
│       ├── analysis.go            # DB ops: signals, ideas, evidence, ranking, review
│       ├── db.go                  # pgxpool setup
│       └── document.go            # DB ops: documents, chunks, fetch audits
└── internal/store/migrations/
    ├── 001_init.up.sql             # Full schema
    ├── 002_query_log_idx.up.sql
    ├── 003_signal_meta.up.sql      # Added industries/regions/chunk_refs to disruption_signal
    └── 004_idea_text_cols.up.sql   # Added pain_point/industries/countries to saas_idea_candidate
```

---

## Environment (.env)
```
DATABASE_URL=postgres://...
OPENAI_KEY=sk-...
OPENAI_MODEL=gpt-4o-mini
BRAVE_API_KEY=...
```

---

## Phase 4 — Review Dashboard (what to build next)

From `docs/09-mvp-backlog.md`:
> - **[M] Review queue UI:** ranked list, filters (label/industry/score), idea detail with citations/excerpts + score breakdown.
> - **[S] Reviewer actions:** approve / reject / edit / merge duplicates / request evidence → `review_status`.
> - **[S] CSV export;** **[M] Notion + Airtable export** via their APIs.
> - **[S] Rubric calibration loop:** capture reviewer decisions for weight tuning.
> - **Exit criteria:** a reviewer can go from queue → approved ideas → exported.

### Suggested stack
**Go HTTP server + HTMX + TailwindCSS CDN** — single binary, no JS build step.

Use `net/http` (or `chi` router) for the backend, `html/template` for server-rendered pages, HTMX for partial updates (approve/reject without full reload).

### Pages to build

1. **`GET /`** — Review queue: ranked table of `saas_idea_candidate` ordered by `overall_score DESC`.
   - Columns: idea name, label, score, industries, review state, created_at
   - Filters: label, review_state (`pending`/`approved`/`rejected`), score range
   - Each row links to detail page

2. **`GET /ideas/:id`** — Idea detail:
   - Full pitch, pain_point, why_it_might_work/fail, possible_mvp, first_10_customers
   - Score breakdown (radar/table from `ranking_score.components`)
   - Citations/evidence excerpts with source URL
   - Approve / Reject / Edit buttons (HTMX POSTs)

3. **`POST /ideas/:id/review`** — Update `review_status.state` + notes. Return updated row partial.

4. **`GET /export/csv`** — Stream CSV of all `approved` ideas.

### DB queries needed (add to `internal/store/`)

```go
// ListIdeas(ctx, db, filter) ([]IdeaRow, error)
// GetIdeaDetail(ctx, db, id) (IdeaDetail, error)
// UpdateReviewStatus(ctx, db, ideaID, state, notes) error
// ExportApproved(ctx, db) ([]IdeaRow, error)
```

### Key schema tables to query

```sql
-- Ranked queue
SELECT i.id, i.idea_name, i.one_sentence_pitch, i.label, i.sales_motion,
       i.industries, i.pain_point, rs.overall_score, rv.state
FROM saas_idea_candidate i
JOIN ranking_score rs ON rs.idea_id = i.id
JOIN review_status rv ON rv.idea_id = i.id
ORDER BY rs.overall_score DESC;

-- Detail + evidence
SELECT e.excerpt, e.source_url, e.citation_text
FROM evidence e WHERE e.idea_id = $1;

-- Score components (JSONB)
SELECT components FROM ranking_score WHERE idea_id = $1;
-- components: {"criteria":[14 ints], "generic_risk":int, "consulting_risk":int, "trust_weight":float}
```

### review_status.state enum values
```
pending → approved | rejected
```

### Suggested file layout for Phase 4
```
/web/
  templates/
    base.html       # layout with nav, Tailwind CDN, HTMX CDN
    queue.html      # review queue page
    detail.html     # idea detail page
/internal/api/
  server.go         # chi/net-http router, middleware
  handlers.go       # queue, detail, review action, CSV export
  store_queries.go  # ListIdeas, GetIdeaDetail, UpdateReviewStatus, ExportApproved
```

### Suggested entry point addition in main.go
```go
case "serve":
    _ = serveCmd.Parse(os.Args[2:])
    if err := runServe(*serveAddr); err != nil { ... }
```

---

## Notes / gotchas

- **UTF-8**: `pdftotext` emits invalid byte sequences — already sanitized with `strings.ToValidUTF8` in pipeline.
- **DB enum spellings**: `doc_state` uses American spelling (`analyzed`, not `analysed`). `idea_label` values: `enterprise_high_ticket`, `smb_saas`, `vertical_saas`, `developer_tool`, `compliance_regtech`, `ai_workflow_automation`, `marketplace`, `consumer_mass_market`, `not_suitable`. `sales_motion` values: `enterprise`, `SMB`, `mass-market`, `developer-led`, `marketplace`.
- **pgvector embeddings**: stored as `vector(1536)`, scanned as `string` in text protocol — use `parseVecAny()` in `store/analysis.go` to decode.
- **Score range**: `overall_score` is 1–100 (deterministic rubric in `internal/scoring/rubric.go`).
- **Migrations**: run with `make migrate` (custom Go migrator, reads `internal/store/migrations/*.up.sql` in order).
- **Module path**: `github.com/mateusgetulio/papertrail`
