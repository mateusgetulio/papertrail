# Contributing to Paper Trail

Thanks for your interest in contributing! Paper Trail is an open-source Go CLI
that discovers, ingests, and analyses **public** research material to surface
ranked SaaS opportunity candidates.

## Ground rules (read first)

Paper Trail talks to third-party sources and AI APIs. Contributions **must**
preserve the project's compliance posture:

- **Never** add code that bypasses paywalls, login gates, email-gated
  downloads, anti-bot systems, or `robots.txt`.
- **Never** commit ingested content — source PDFs, extracted text, chunk
  fixtures, embeddings, or database dumps. Only original code and docs belong
  in this repo. (`.pdf` and `.env` are already gitignored; keep it that way.)
- **Never** commit API keys or secrets. Use `.env` (gitignored); update
  `.env.example` with placeholders only.
- Respect each source's Terms of Service. See [`docs/03-compliance.md`](./docs/03-compliance.md).

## Development setup

Requirements: Go 1.26+, Docker (for Postgres + pgvector), Poppler (`pdftotext`).

```bash
cp .env.example .env        # fill in your own keys
docker compose up -d db     # starts Postgres + pgvector on :807
make migrate                # apply migrations
go build ./...
```

Run the pipeline:

```bash
go run ./cmd/papertrail discover --limit 5
go run ./cmd/papertrail ingest   --limit 3
go run ./cmd/papertrail analyse  --limit 10
go run ./cmd/papertrail serve                  # review dashboard
```

## Pull requests

1. Fork and branch from `main` (`feat/...`, `fix/...`, `docs/...`).
2. Keep changes focused; one logical change per PR.
3. `go build ./...` and `go vet ./...` must pass; run `go test ./...` if you
   touch tested packages.
4. Format with `gofmt`/`goimports`.
5. Describe what changed and why. Link any related issue.

## Reporting bugs / requesting features

Open an issue using the templates in `.github/`. For security issues, follow
[`SECURITY.md`](./SECURITY.md) instead of filing a public issue.

## License

By contributing, you agree your contributions are licensed under the project's
[MIT License](./LICENSE).
