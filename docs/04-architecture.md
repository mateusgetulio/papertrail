# 04 — Architecture

The Go-based ingestion pipeline, end to end. Thirteen stages, each idempotent and resumable, coordinated by a job queue. The compliance gate (stage 5) is the safety choke point: nothing is downloaded without passing it.

## 1. High-level flow

```
                       ┌──────────────────────────────────────────────────────────┐
                       │                    Job Queue (river/asynq)                 │
                       └──────────────────────────────────────────────────────────┘
                                              ▲   ▲   ▲
  (1) Search/Discovery ─► (2) URL Dedup ─► (3) Trust Score ─► (4) PDF/HTML Detect
        ─► (5) Allowed-Fetch Gate ─► (6) Download/Extract Page ─► (7) Text Extraction
        ─► (8) Metadata Extraction ─► (9) Chunking ─► (10) LLM Analysis
        ─► (11) Structured Opportunity Extraction ─► (12) Ranking ─► (13) Review Dashboard
                                              │
                              Postgres + pgvector (state, vectors, audit)
                              Object/temp store (transient full-text)
```

Each stage reads/writes Postgres and enqueues the next stage. Failures move to a dead-letter with reason; reruns are safe (idempotent on content hash / URL canonical form).

## 2. Stage-by-stage

### (1) Search / Discovery
- Behind a `SearchProvider` interface (Brave / Google PSE / Exa / Tavily / Common Crawl).
- Expands query templates (see `docs/02`) across {domain × theme × industry × year}.
- Enforces per-provider daily quota; persists query fingerprints to avoid re-spending.
- Emits candidate `(url, title, source_domain, discovered_via)` rows.

### (2) URL Deduplication
- **Canonicalize:** lowercase host, strip tracking params (`utm_*`, `fbclid`), normalize trailing slashes, resolve redirects later.
- **Dedup keys:** canonical URL hash; later, content hash (post-download) for cross-URL dup detection.
- Skip URLs already in `document` (any non-failed state).

### (3) Source Trust Scoring
- Map `source_domain` → trust tier (A–F, see `docs/02 §3`) via the `source` registry.
- Compute `trust_weight` used later by ranking. Unknown domains get a low default and are flagged for review before fetch.

### (4) PDF / HTML Detection
- Guess content type from extension, then confirm via `HEAD` (Content-Type) where allowed.
- Route: `application/pdf` → PDF path; `text/html` → HTML path; others → skip.

### (5) Allowed-Fetch Gate  ⛔ safety choke point
- Implements `docs/03 §2`: robots.txt check, gating/anti-bot detection, ToS policy flag, rate-limit budget, content-type allow.
- Records `fetch_decision` (allowed/skipped + reason) to the audit log. Skips are terminal (no bypass).

### (6) Download / Page Extraction
- **PDF:** stream download with size cap; store to **transient** temp store (not long-term).
- **HTML:** fetch; for legitimately JS-rendered allowed pages, optional Playwright worker; else `goquery` + readability for main content.
- Compute content hash → cross-check dedup (stage 2).

### (7) Text Extraction
- **PDF → text:** shell out to Poppler `pdftotext -layout`; fallback OCR via `tesseract` for scanned/empty-text PDFs.
- **HTML → text:** readability-extracted main content; strip nav/boilerplate.
- Output normalized UTF-8 plain text (transient).

### (8) Metadata Extraction
- From PDF info dict / HTML `<meta>` / OpenGraph / SERP data: title, author(s), publisher, publish date, language, page count.
- Heuristics + optional small LLM call to normalize messy fields (e.g., publisher name).
- Persist metadata permanently (metadata is storable).

### (9) Chunking
- Split normalized text into overlapping chunks (e.g., ~800–1200 tokens, ~15% overlap), respecting section boundaries where detectable.
- Generate OpenAI embeddings per chunk → store vectors in `pgvector` (chunks themselves held transiently; embeddings + offsets persisted, not full proprietary text).
- Chunks feed both LLM analysis and cross-document pain-point clustering.

### (10) LLM Analysis
- OpenAI (default) behind `LLMClient` interface. **Single cheap model** (cheapest with solid results, e.g., GPT-4o-mini / GPT-4.1-mini class) used for all steps — no premium tier:
  - *Triage:* is this document actually about market disruption / pain points? Discard if not.
  - *Analysis:* extract disruption signals, pain points, industries, drivers, segments — each tied to chunk evidence offsets.
- Cost control comes from prompt design + chunk retrieval + caching, not from a more expensive model.
- Strict grounding: every extracted claim must reference supporting chunk(s); see `docs/07`.

### (11) Structured Opportunity Extraction
- Convert signals/pain points → candidate SaaS ideas using the agent workflow (`docs/07`).
- Reject weak ideas; group duplicates via embedding similarity; assign category labels.
- Emit schema-valid candidate JSON (`docs/08`) with citations.

### (12) Ranking
- Apply the deterministic weighted rubric (`docs/06`) over LLM sub-scores + `trust_weight` + cross-source `problem_frequency`.
- Produce `overall_score` (1–100) and store `ranking_score` with component breakdown for explainability.

### (13) Human Review Dashboard
- Web UI lists ranked candidates with citations, evidence excerpts, scores, and labels.
- Reviewer actions: approve / reject / edit / merge duplicates / request more evidence → updates `review_status`.
- Export approved ideas to CSV / Notion / Airtable.

## 3. Concurrency model (Go)

- **Worker pools per stage**, each consuming from the job queue; bounded concurrency overall.
- **Per-domain limiter** (token bucket keyed by host) enforces politeness across all fetch workers.
- Fetch fan-out uses goroutines + `errgroup`; CPU-bound extraction offloaded to a separate pool to avoid starving I/O.
- Backpressure via queue depth; autoscale workers within limits.

## 4. Job queue & state

- **Queue:** `riverqueue` (Postgres-backed, transactional with app data — preferred) or `asynq` (Redis-backed).
- River's Postgres-native transactions let us enqueue the next stage in the same tx that records stage output → exactly-once-ish, no lost work.
- **State machine** per document: `discovered → deduped → scored → gated_check → fetched → extracted → analyzed → ranked → in_review → {approved|rejected}` (+ `skipped`/`failed` terminals with reasons).

## 5. Storage layout

| Store | Holds | Retention |
|---|---|---|
| **Postgres** | All entities (`docs/05`), audit log, job state | Persistent |
| **pgvector** | Chunk embeddings, idea embeddings | Persistent (derived data) |
| **Temp/object store** | Downloaded PDFs, extracted full text | **Transient** — deleted after processing per retention job |
| **Secrets** | API keys (OpenAI, search providers) | Env / secret manager, never in DB/code |

## 6. Component map (Go packages)

```
/cmd/papertrail        # entrypoints: api, worker, cli
/internal/search       # SearchProvider impls (brave, google, exa, tavily, commoncrawl)
/internal/compliance   # robots parser, gate, rate limiter, policy registry
/internal/fetch        # http client, playwright bridge, size caps, caching
/internal/extract      # pdftotext/tesseract bridge, html readability
/internal/chunk        # chunking + embeddings client
/internal/llm          # LLMClient interface + openai impl, prompt templates
/internal/opportunity  # idea extraction, dedup/clustering, labels
/internal/scoring      # deterministic rubric
/internal/store        # pgx repositories, migrations
/internal/jobs         # river/asynq workers + state machine
/internal/api          # dashboard API + exports (csv/notion/airtable)
/web                   # review dashboard (server-rendered or SPA)
```

## 7. Observability & ops

- Structured logs (slog) with `document_id` correlation; per-stage metrics (counts, latency, skip reasons).
- Audit trail of fetch decisions and stored artifacts (compliance evidence).
- Cost tracking for LLM/search API spend per stage; alert on quota/budget thresholds.
- Retention cron enforces transient-text deletion and excerpt caps.

## 8. External dependencies

- **Binaries:** `pdftotext` (Poppler), `tesseract` (optional OCR), optional Playwright runtime.
- **APIs:** OpenAI (analysis + embeddings); chosen search provider(s).
- **Infra:** Postgres 15+ with `pgvector`; optional Redis (only if using `asynq`).
