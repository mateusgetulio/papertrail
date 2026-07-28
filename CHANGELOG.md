# Changelog

All notable changes to this project are documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.1.0] - 2026-07-28

First tagged release: the full pipeline runs end to end.

### Added

- 13-stage pipeline: search discovery, URL dedup, source trust scoring, PDF/HTML detection, compliance gate, fetch, text extraction, metadata, chunking + embeddings, LLM agent workflow, structured opportunity extraction, deterministic scoring, review dashboard.
- 4-agent LLM analysis workflow (document operator, opportunity analyst, human report, chain quality gate) with structured output and a provider-agnostic `LLMClient` interface.
- Deterministic 14-criterion scoring rubric with penalty multipliers and stored component breakdowns.
- Decision support: Build/Validate/Park recommendation, quick-win score, decision matrix, themes, confidence tiers.
- Review dashboard (`papertrail serve`): queue with search/sort/pagination, candidate detail with score breakdown, compare view, insights, inline edit, CSV export of approved ideas.
- Compliance layer enforced in code: robots.txt parsing and caching, gating detection, polite rate limits; no fetch happens without passing the gate.
- Phase 0 methodology doc: pre-MVP audience and demand validation ladder with numeric pass bars (`docs/00-audience-validation.md`).
- Unit tests for the pure-logic packages; CI (build, vet, race tests, golangci-lint) on every push and pull request.

### Changed

- Dashboard assets (compiled Tailwind stylesheet, htmx) are embedded in the binary via `go:embed`; no CDN or runtime asset step.

[0.1.0]: https://github.com/mateusgetulio/papertrail/releases/tag/v0.1.0
