# 09 — MVP Backlog

A phased, build-ready backlog for the **private** Paper Trail tool in **Go**. Each phase is shippable and de-risks the next. Effort is rough (S/M/L) for a small team; sequence matters more than estimates.

## Recommended first MVP architecture (concrete)

| Concern | Choice (MVP) | Why |
|---|---|---|
| Language | **Go** | Single binary, great concurrency, simple ops |
| DB | **Postgres 15+** | Relational model + transactions |
| Vectors | **pgvector** | One datastore; good enough at MVP scale |
| Jobs | **riverqueue** (Postgres-backed) | Transactional enqueue with app data; no extra infra |
| Search | **Brave API** (free tier) + manual seed URLs | Cheapest viable start |
| PDF text | **Poppler `pdftotext`** (shell out) | Robust, no native lib gaps |
| HTML | **goquery** + readability port | Main-content extraction |
| LLM | **OpenAI** single cheapest-solid model (GPT-4o-mini / GPT-4.1-mini class), structured outputs | Internal-tool cost cap: one cheap model for everything |
| Embeddings | **OpenAI embeddings** | Powers dedup/clustering |
| Dashboard | **Go server-rendered (templ/html-template) + HTMX**, or small React SPA | Fast to build; internal-only |
| Export | CSV first; Notion/Airtable via their APIs | Get value out early |
| Browser automation | **Playwright** worker (optional, allowed sources only) | Only for legit JS-rendered public pages |

> Pragmatic stance: lean custom pipeline (not LangChain/LlamaIndex) — in Go the orchestration is straightforward and a custom pipeline keeps the compliance gate and grounding fully under our control.

## Phase 0 — Spike & foundations (validate the risky bits)
- **[S] Repo + tooling:** Go module, lint, migrations tool, `.env`/secrets handling, Makefile.
- **[S] Postgres + pgvector** up via docker-compose; run base migrations (`docs/05`).
- **[M] LLM spike:** `LLMClient` interface + OpenAI impl; structured-output call returning schema-valid candidate JSON (`docs/08`) from a pasted sample text.
- **[S] PDF spike:** `pdftotext` bridge extracting text from 5 sample public PDFs.
- **Exit criteria:** one PDF → text → LLM → valid JSON, end to end, by hand.

## Phase 1 — Discovery + compliance (the safety core)
- **[M] `SearchProvider` interface** + Brave impl; query-template expansion (`docs/02 §4`); `search_query_log` dedup.
- **[M] Compliance gate (`docs/03 §2`):** robots.txt parser/cache, gating/anti-bot detection, per-domain rate limiter, policy registry seeding (`source` table tiers).
- **[S] URL canonicalization + dedup** (stage 2).
- **[S] Source trust registry** seeded with Tier A–F domains (`docs/02 §3`).
- **[S] Fetch audit logging** (`fetch_audit`).
- **Exit criteria:** given seed queries, system produces a vetted, deduped, *allowed-only* URL list with full audit trail. **No bypass paths exist.**

## Phase 2 — Ingestion + extraction
- **[M] Fetcher** with size caps, caching, backoff; transient temp store; content hashing.
- **[S] PDF/HTML detection + routing.**
- **[M] Text extraction:** `pdftotext` (+ optional `tesseract` OCR fallback); HTML readability.
- **[S] Metadata extraction** (PDF info / meta tags / OpenGraph), persisted.
- **[M] Chunking + embeddings** into `pgvector`; enforce excerpt caps + transient-text retention job (`docs/03 §8`).
- **Exit criteria:** allowed URL → stored metadata + chunks/embeddings; full text auto-purged after processing.

## Phase 3 — Analysis, scoring, dedup
- **[L] Agent workflow (`docs/07`):** triage → signal extraction → idea generation → reject → score → label, all grounded with `chunk_refs`.
- **[M] Deterministic scoring (`docs/06`)** in Go, including penalty multipliers; store `ranking_score.components`.
- **[M] Idea/pain-point clustering** via embeddings; `problem_frequency` rollups.
- **[S] JSON Schema validation + repair retry + dead-letter** (`docs/08 §2`, `docs/07 §8`).
- **Exit criteria:** documents produce scored, deduped, cited candidate ideas matching the output contract.

## Phase 4 — Review dashboard + export
- **[M] Review queue UI:** ranked list, filters (label/industry/score), idea detail with citations/excerpts + score breakdown.
- **[S] Reviewer actions:** approve / reject / edit / merge duplicates / request evidence → `review_status`.
- **[S] CSV export;** **[M] Notion + Airtable export** via their APIs.
- **[S] Rubric calibration loop:** capture reviewer decisions for weight tuning (`docs/06 §8`).
- **Exit criteria:** a reviewer can go from queue → approved ideas → exported, and decisions feed calibration.

## Phase 5 — Hardening & scale (post-MVP)
- Common Crawl backfill provider; add Exa/Tavily semantic discovery; optional SerpAPI.
- Cost dashboards + budget alerts (LLM/search spend per stage).
- Observability (metrics, tracing), retry/dead-letter tooling, kill switches.
- Playwright worker for allowed JS-rendered sources.
- Scheduled re-runs / freshness sweeps for new "outlook" reports.

## Cross-cutting (every phase)
- **Compliance first:** never merge a fetch path that can bypass the gate.
- **Idempotency:** keyed on `url_hash`/`content_hash`/`query_hash`.
- **Secrets:** env/secret manager only.
- **Tests:** unit tests for compliance gate + scoring math; golden-file tests for the output JSON contract.

## Suggested first sprint (2 weeks)
1. Phase 0 fully.
2. Phase 1: `SearchProvider`+Brave, robots/gate, dedup, trust registry.
3. Demo: seed queries → audited, allowed URL list → one doc hand-run through to a scored JSON candidate in the DB.
