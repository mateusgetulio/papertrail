# Compliance (pipeline summary)

This is the short, agent-focused summary. The full policy is in
[`03-compliance.md`](03-compliance.md); prohibited actions are in
[`10-risk-register.md`](10-risk-register.md).

## Hard rules (Agent 1 enforces these via `internal/compliance`)
- Do **not** bypass paywalls, login gates, email-gated downloads, CAPTCHAs,
  anti-bot/JS challenges, `robots.txt`, or any source's Terms of Service.
- Do **not** scrape aggressively — discovery is rate-limited and prefers search
  APIs, public PDFs/HTML, official report pages, and approved source lists.
- The fetch gate is **fail-closed**: if a source's status is uncertain, it is
  skipped, not fetched.

## Safe to store ✅
- Document metadata (title, publisher, domain, publication date, URL, type).
- Source URLs and citation references for **every** extracted signal/idea.
- Short, legally-safe excerpts used as evidence.
- Original, LLM-generated summaries and analysis.
- Embeddings/derived vectors for dedup and clustering.

## Must NOT store / commit ❌
- Full copyrighted report text beyond transient processing.
- Near-verbatim summaries that reproduce source prose.
- Raw downloaded PDFs/documents, cache files, vector stores, local databases,
  logs, or generated reports **in version control** — all are gitignored
  (`generated_reports/`, `raw_documents/`, `vector_store/`, `*.pdf`, `*.db`, …).
- Secrets — use `.env` (gitignored); `.env.example` holds placeholders only.

## Why this matters to the pipeline
The Chain Quality Gate (Agent 4) treats citation loss and disallowed/gated
fetching as **blockers**: ungrounded or non-compliant output stops the chain
rather than being published. Every surfaced idea must trace back to a source URL.
