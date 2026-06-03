# 01 — Technical & Legal Feasibility Report

**Verdict: Feasible and worth building — as a permanently private, compliance-first internal tool.** The hard part is not the engineering; it is staying inside legal/ToS boundaries while still gathering enough signal, and producing *original analysis* rather than copying report content.

## 1. Executive summary

- **Technically feasible:** Yes. Every stage (search → fetch → extract → LLM analysis → score → review) uses well-understood components. Go is a fine orchestration language; the only ecosystem gaps (PDF parsing, ML) are solved by shelling out to `pdftotext` and calling hosted LLM/embedding APIs.
- **Legally feasible:** Yes, *if* we restrict ourselves to public, non-gated material; respect `robots.txt` and ToS; rate-limit politely; and store metadata/citations/short excerpts/original summaries instead of full copyrighted text.
- **Biggest risks:** (a) copyright over-collection, (b) ToS violations on consulting sites, (c) anti-bot/gating that tempts bypass, (d) LLM-generated opportunities that are plausible but ungrounded ("hallucinated signal"). All are manageable with guardrails.
- **Recommended first version:** a private pipeline that discovers PDFs/HTML via a search API, fetches only allowed public content, extracts text, runs OpenAI analysis to emit scored SaaS-idea JSON with citations, and surfaces results in a human-review dashboard.

## 2. What is technically possible

| Capability | Possible? | Notes |
|---|---|---|
| Programmatic discovery of public PDFs/reports | Yes | Search APIs support `site:` and `filetype:pdf`; Common Crawl indexes huge swaths of the public web. |
| Fetch public PDFs/HTML | Yes | Standard HTTP with polite headers, after a robots/ToS allow-check. |
| Extract text from PDFs | Yes | Poppler `pdftotext` is robust; OCR (Tesseract) for scanned PDFs as a later add-on. |
| Detect duplicates / near-duplicates | Yes | URL canonicalization + content hash + embedding similarity. |
| Extract structured signals with an LLM | Yes | OpenAI structured-output mode emits schema-valid JSON. |
| Rank opportunities | Yes | Deterministic weighted rubric over LLM-produced sub-scores. |
| Human review + export | Yes | Web dashboard + CSV/Notion/Airtable export. |
| Cluster repeated pain points across sources | Yes | Embedding-based clustering in `pgvector`. |

## 3. What is legally risky (and how we neutralize it)

| Risk | Why risky | Mitigation |
|---|---|---|
| Storing full report text | Copyright infringement | Store metadata + short excerpts + original summaries; cap excerpt length; keep originals only transiently for processing. |
| Scraping gated/paywalled content | Circumvention + ToS breach + possible CFAA-style exposure | Hard block: never fetch behind login/paywall/email-gate; detect and skip. |
| Ignoring `robots.txt` | ToS/established-norm violation | Parse and obey `robots.txt` before every fetch. |
| Mass crawling | Server abuse, IP bans, ToS breach | Per-domain rate limits, concurrency caps, backoff, identifiable User-Agent. |
| Republishing summaries that are too close to source | Derivative-work risk | Enforce "transformative, original analysis"; summaries describe *implications*, not reproduce prose. |
| Anti-bot bypass | ToS breach, bad faith | Never bypass. If a source blocks bots, drop it. |

## 4. What should be avoided entirely

- Bypassing paywalls, logins, email gates, CAPTCHAs, or anti-bot systems.
- Ignoring `robots.txt` or a site's Terms of Service.
- Bulk-downloading a publisher's full catalog.
- Storing or redistributing full copyrighted reports.
- Presenting source excerpts as if they were our own content, or vice-versa.

## 5. Feasibility by component (build risk)

| Component | Build risk | Comment |
|---|---|---|
| Search/discovery | Low | API-driven; main work is query design + quota management. |
| Compliance gate | Medium | Correctness matters most; conservative defaults. |
| Fetch + extract | Low | Mature tooling. |
| LLM analysis | Medium | Prompt quality + grounding + cost control drive value. |
| Scoring | Low | Deterministic once rubric is fixed. |
| Dedup/clustering | Medium | Tuning similarity thresholds. |
| Dashboard | Low | Standard CRUD + review queue. |

## 6. Go-specific feasibility notes

- **Strengths:** excellent concurrency for fetch fan-out, simple deployment (single binary), strong HTTP/JSON, good Postgres support (`pgx`), mature job queues (`river`, `asynq`).
- **Gaps & workarounds:**
  - *PDF parsing* → shell out to `pdftotext` (Poppler); fall back to OCR via `tesseract` for scanned docs.
  - *LLM / embeddings* → call OpenAI HTTP APIs directly (no heavy native ML needed).
  - *HTML extraction* → `goquery` + a readability port for main-content extraction.
- **Net:** Go is well-suited; nothing in the design requires Python.

## 7. Recommendation

Build the **private MVP** described in `docs/09-mvp-backlog.md`. Prioritize the compliance gate and citation-grounding from day one — they are cheaper to build in than to retrofit, and they define whether the whole project is safe. Treat the LLM output as *leads for humans to validate*, not as ground truth.
