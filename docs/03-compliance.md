# 03 — Compliance & Safety

The rules that keep Paper Trail legal and ethical. These are **hard constraints**, enforced in code by an "allowed-fetch" gate, not guidelines. When in doubt, **skip the document**.

> Engineering guidance, not legal advice. Have counsel review before production. Re-verify each source's `robots.txt` and Terms of Service at build time.

## 1. Core principles

1. **Public, non-gated only.** We fetch material that is freely and publicly accessible without authentication, payment, or form submission.
2. **Respect machine-readable rules.** `robots.txt` directives are obeyed for every fetch.
3. **Respect human-readable rules.** A source's Terms of Service override convenience. If ToS forbids automated access or reuse, we don't.
4. **Minimal collection.** Store the least we need: metadata, citations, short excerpts, and original summaries — not full proprietary documents.
5. **Be a polite citizen.** Identify ourselves, rate-limit, back off, cache.
6. **Transformative output.** We produce original analysis, never republished prose.

## 2. The allowed-fetch gate (enforced before every download)

A fetch proceeds **only if all** checks pass:

```
ALLOW fetch(url) IFF
  robots_allows(url, user_agent)            # robots.txt permits this path for our UA
  AND NOT is_gated(url)                      # no login/paywall/email-gate signals
  AND NOT is_anti_bot(url)                    # no CAPTCHA / bot-wall detected
  AND source_tos_permits_automated_access     # per-source policy flag (default: conservative)
  AND within_rate_limit(domain)               # per-domain budget not exceeded
  AND content_type_allowed(url)               # pdf/html only
```

Any failure → record `fetch_decision = skipped` with a reason; never retry as a bypass.

### Detecting gated / anti-bot content
- **Pre-fetch signals:** URL patterns (`/login`, `/gated`, `?download=`, `register`), known-gated domains list.
- **Post-fetch signals (then discard body):** HTTP 401/403; presence of login forms, paywall markers, "register to download", email-capture forms; CAPTCHA / JS-challenge markers (e.g., Cloudflare interstitials). If detected → discard, mark `gated`, do **not** attempt to solve or bypass.

## 3. robots.txt handling

- Fetch and cache each domain's `robots.txt`; respect `Disallow`, `Allow`, and `Crawl-delay`.
- Match against our **declared User-Agent**; honor `*` rules too.
- Re-fetch `robots.txt` on a TTL (e.g., 24h). On fetch error, **fail closed** (assume disallowed for ambiguous paths) for sensitive domains.
- Never fetch a path that robots disallows, even if reachable.

## 4. Terms of Service handling

- Maintain a per-source **policy registry** (`source.policy`) with fields like `allows_automated_access`, `allows_excerpt_storage`, `license` (e.g., `cc-by`, `all-rights-reserved`, `public-domain`), and `notes`.
- Default to **conservative** (`all-rights-reserved`, excerpt-only, no full-text retention) until a source is explicitly reviewed.
- Open-licensed sources (e.g., many World Bank/OECD/gov publications) can have relaxed retention — still cite and record the license.

## 5. Polite fetching policy

| Control | Default |
|---|---|
| User-Agent | Honest, identifiable string + contact URL (e.g., `PaperTrailResearchBot/0.1 (+https://internal/contact)`) |
| Per-domain concurrency | 1–2 |
| Per-domain rate | Respect `Crawl-delay`; else ~1 req / 2–5s |
| Global concurrency | Bounded worker pool |
| Backoff | Exponential on 429/5xx; honor `Retry-After` |
| Caching | Cache by URL + ETag/Last-Modified; avoid re-fetching |
| Timeouts | Sane connect/read timeouts; size cap on downloads |

## 6. Copyright & content-retention policy

What we store, by content category:

| Data | Store? | Rule |
|---|---|---|
| URL, title, author, publisher, date, source domain | **Yes** | Metadata is freely storable. |
| Full extracted text of a copyrighted report | **No (persist)** | Hold transiently in memory/temp only for processing; do not persist long-term. |
| Short excerpts / quotes | **Yes, limited** | Cap length (e.g., ≤ ~25 words per excerpt, few per doc); only when needed as evidence; always attributed. |
| Original LLM summaries (our analysis) | **Yes** | Transformative; describes implications, not reproduces prose. |
| Embeddings/derived vectors | **Yes** | Derived representations, not the text itself. |
| Open-licensed / public-domain full text | **Yes (allowed)** | Permitted when license allows; record the license. |

### Excerpt safety rules
- Excerpts must be **short, attributed, and necessary** as evidence for a claim/score.
- Never assemble multiple excerpts into a substantial reproduction of the original.
- Summaries must be genuinely transformative — paraphrase + analysis, not light edits of source sentences.

## 7. What must NOT be done (prohibited actions)

- Bypassing paywalls, login gates, email-gated downloads, CAPTCHAs, or anti-bot/JS challenges.
- Ignoring or circumventing `robots.txt`.
- Violating any source's Terms of Service.
- Mass-crawling or bulk-downloading a publisher's catalog.
- Persisting full copyrighted report text beyond transient processing.
- Republishing report content, or producing summaries that closely track source prose.
- Misrepresenting our crawler's identity, rotating IPs/UA to evade blocks, or scraping via headless browsers where the site disallows automation.

## 8. Operational safeguards

- **Audit log:** record every fetch decision (allowed/skipped + reason) and what was stored.
- **Retention job:** scheduled deletion of transient full-text after processing; enforce excerpt caps.
- **Domain allow/deny lists:** curated; gated domains marked once and skipped thereafter.
- **Kill switch:** ability to halt fetching for a domain on abuse signals (429s, complaints).
- **Review before adding sources:** new domains require a policy-registry entry before fetching.

## 9. Playwright / headless browsers

- Use **only** where a source's robots/ToS permit automated access and content is public.
- Never use headless automation to defeat bot-walls, paywalls, or gates.
- Prefer plain HTTP fetch; reserve Playwright for legitimately JS-rendered public pages on allowed domains.
