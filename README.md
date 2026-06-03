# Paper Trail

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

An **open-source** Go CLI that discovers "market disruption" signals from public business white papers, consulting insights, research reports, and industry publications — then extracts repeated pain points, underserved industries, and ranked SaaS opportunity candidates.

> **Status:** working pipeline (Phases 0–3). Discovery, ingestion, and LLM analysis run end-to-end; the review dashboard (Phase 4) is in progress. See [`docs/09-mvp-backlog.md`](./docs/09-mvp-backlog.md).

## What this is (and is not)

- **Is:** open-source **code** that you run yourself, with your own API keys, to produce *original analysis* — structured summaries, scores, and citations pointing back to source material.
- **Is not:** a content aggregator, a republisher of report text, or a hosted service that serves proprietary report content. The repository ships **code only** — never source PDFs, extracted text, or report content.

## Open code, private content

The **code** is open source under the [MIT License](./LICENSE). The **content** it processes is not redistributed — and that separation is what keeps the legal-risk posture intact:

- **The repo never contains copyrighted material.** Source PDFs, extracted text, chunk fixtures, embeddings, and database dumps stay out of version control (`*.pdf` and `.env` are gitignored). Removing report text from the repo removes the main copyright-infringement vector.
- **Operators run their own instance** with their own keys and their own data store. What an operator does with short excerpts for private analysis sits far closer to fair-use/fair-dealing territory than public redistribution.
- **Don't host it as a public service that serves proprietary report content.** Doing so reintroduces end-user ToS, DMCA exposure, and scaled-redistribution claims that the project is designed to avoid.
- Paper Trail stores and acts on **metadata + citations + short, legally-safe excerpts + original LLM-generated summaries** — never full proprietary documents when not necessary.

## Compliance stance (summary)

Hard rules — see `docs/03-compliance.md` for detail:

- Do **not** bypass paywalls, login gates, email-gated downloads, anti-bot systems, or `robots.txt`.
- Do **not** violate any source's Terms of Service.
- Do **not** mass-crawl, and do **not** copy large amounts of copyrighted content.
- **Do** use public metadata, allowed downloads, citations, short excerpts where legally safe, and original summaries.

## Tech direction (summary)

- **Language:** Go (orchestration, concurrency, job queue, HTTP clients).
- **PDF text:** shell out to Poppler `pdftotext` (Go's native PDF libs are thin).
- **LLM analysis/scoring:** a **single, cheapest OpenAI model with solid results** (e.g., GPT-4o-mini / GPT-4.1-mini class), structured-output mode, behind a provider-agnostic `LLMClient` interface. No premium tier — cost is held to the cheap model plus embeddings; the interface allows upgrading later only if quality demands it.
- **Storage:** Postgres + `pgvector` (via `pgx`) for embeddings + dedup/clustering.
- **Search discovery:** start free/cheap (Brave free tier or Google PSE quota + Common Crawl), upgrade path to Exa/Tavily/SerpAPI.

## Document index

| # | Doc | Covers |
|---|-----|--------|
| — | [`README.md`](./README.md) | This overview |
| 01 | [`docs/01-feasibility-report.md`](./docs/01-feasibility-report.md) | Technical + legal feasibility verdict |
| 02 | [`docs/02-source-discovery.md`](./docs/02-source-discovery.md) | Sources, search API comparison, query patterns |
| 03 | [`docs/03-compliance.md`](./docs/03-compliance.md) | robots.txt, ToS, copyright, retention policy |
| 04 | [`docs/04-architecture.md`](./docs/04-architecture.md) | Go pipeline, 13 stages, infra |
| 05 | [`docs/05-data-model.md`](./docs/05-data-model.md) | Postgres schema + DDL |
| 06 | [`docs/06-scoring-rubric.md`](./docs/06-scoring-rubric.md) | Opportunity scoring model |
| 07 | [`docs/07-agent-workflow.md`](./docs/07-agent-workflow.md) | LLM agent loop + labels |
| 08 | [`docs/08-output-format.md`](./docs/08-output-format.md) | Candidate JSON contract + examples |
| 09 | [`docs/09-mvp-backlog.md`](./docs/09-mvp-backlog.md) | Phased build backlog |
| 10 | [`docs/10-risk-register.md`](./docs/10-risk-register.md) | Risk register + prohibited actions |

## Quick start

Requirements: Go 1.26+, Docker (Postgres + pgvector), Poppler (`pdftotext`).

```bash
cp .env.example .env        # add your own OPENAI_API_KEY + BRAVE_API_KEY
docker compose up -d db     # Postgres + pgvector on :807
make migrate                # apply schema migrations
go build ./...

go run ./cmd/papertrail discover --limit 5   # find candidate sources
go run ./cmd/papertrail ingest   --limit 3   # fetch → extract → embed
go run ./cmd/papertrail analyse  --limit 10  # LLM workflow → scored ideas
go run ./cmd/papertrail serve                # review dashboard (Phase 4)
```

You bring your own API keys; none are shipped with the project.

## Contributing

Contributions are welcome — see [`CONTRIBUTING.md`](./CONTRIBUTING.md) and the
[Code of Conduct](./CODE_OF_CONDUCT.md). The one hard rule: never commit
secrets or ingested source content, and never add code that bypasses access
controls or `robots.txt`. Security issues: see [`SECURITY.md`](./SECURITY.md).

## License

[MIT](./LICENSE) © 2026 Mateus Vieira. You run your own instance with your own
keys and data; the license covers the code, not any third-party content you
choose to process with it.

## Disclaimer

These documents are engineering design guidance, **not legal advice**. Confirm each source's current `robots.txt` and Terms of Service at build time, and have counsel review the compliance posture before production use.
