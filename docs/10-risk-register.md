# 10 — Risk Register

Legal/compliance and technical risks for Paper Trail, with likelihood (L), impact (I), and mitigations. Scale: L/M/H. Mitigations reference the relevant doc.

> Engineering risk assessment, not legal advice. Counsel should review the compliance posture (`docs/03`) before production.

## 1. Legal / compliance risks

| # | Risk | L | I | Mitigation |
|---|---|---|---|---|
| L1 | **Copyright infringement** by storing/republishing full report text | M | H | Store metadata + capped excerpts + original summaries only; transient full-text purged; transformative output (`docs/03 §6`, `docs/07 §3`). |
| L2 | **ToS violation** on consulting/analyst sites | M | H | Per-source policy registry; conservative defaults; skip sources whose ToS forbids automated access (`docs/03 §4`). |
| L3 | **Accessing gated/paywalled content** | L | H | Hard allowed-fetch gate detects login/paywall/email-gate and skips; no bypass code exists (`docs/03 §2`). |
| L4 | **Ignoring robots.txt** | L | M | robots parsed/obeyed per fetch; fail-closed on ambiguity (`docs/03 §3`). |
| L5 | **Anti-bot circumvention** (CAPTCHA/JS challenge) | L | H | Never attempt; detect and drop the source (`docs/03 §2,§9`). |
| L6 | **Derivative-work risk** from summaries too close to source | M | M | Enforce transformative paraphrase + analysis; excerpt caps; separation of "source says" vs "our analysis" (`docs/07 §3`). |
| L7 | **License misreading** of open-licensed sources | L | M | Record `license` per source; cite properly even when reuse is permitted (`docs/05 source`). |
| L8 | **PII / sensitive data** captured incidentally | L | M | Prefer public business reports; redact/avoid PII; no special-category data. |
| L9 | **Search provider ToS** (e.g., scraped-SERP services) | M | M | Use official APIs; review SerpAPI-type providers' terms before adopting (`docs/02 §1`). |

## 2. Technical risks

| # | Risk | L | I | Mitigation |
|---|---|---|---|---|
| T1 | **LLM hallucinated signals/ideas** (plausible but ungrounded) | H | H | Evidence-or-omit grounding; `chunk_refs` required; human review gate (`docs/07 §3,§9`). |
| T2 | **"Consulting-not-software" false positives** dominate output | H | M | Dedicated rejection filter + heavy scoring penalty (`docs/06 §4`, `docs/07 §4`). |
| T3 | **"Too generic" ideas** clutter the queue | H | M | Generic penalty multiplier + name/pitch lint (`docs/06 §4`, `docs/08 §2`). |
| T4 | **PDF extraction failures** (scanned/garbled) | M | M | OCR fallback (`tesseract`); mark `failed` rather than fabricate (`docs/04 §7`, `docs/07 §8`). |
| T5 | **LLM/search API cost overrun** | L | M | Single cheap model only; chunk retrieval not whole-doc; caching; per-stage budget alerts (`docs/07 §7`, `docs/09 Phase 5`). |
| T6 | **Search quota exhaustion** | M | M | Query fingerprint dedup; per-provider daily caps; multi-provider fallback (`docs/02 §4`). |
| T7 | **Duplicate ideas/signals inflate counts** | M | M | Embedding clustering; `problem_frequency` counts distinct source docs only (`docs/07 §5`). |
| T8 | **Domain bans from impolite fetching** | L | M | Rate limits, backoff, identifiable UA, kill switch (`docs/03 §5,§8`). |
| T9 | **Schema drift / invalid LLM output** | M | L | Structured-output mode + Go validation + repair retry + dead-letter (`docs/08 §2`). |
| T10 | **Stale data** (Common Crawl lag, old reports) | M | L | Year-sweeping queries; freshness sweeps; date metadata in scoring context (`docs/02 §4`, `docs/09 Phase 5`). |
| T11 | **Secret leakage** (API keys) | L | H | Env/secret manager only; never in DB/code/logs (`docs/04 §5`). |
| T12 | **Bias toward marketing-driven sources** (VC/vendor) | M | M | Trust tiers down-weight Tier D/F; corroborate across sources (`docs/02 §3`, `docs/06 §3`). |

## 3. Prohibited actions (hard "never" list)

These are non-negotiable and must have **no enabling code path**:

- Bypassing paywalls, login gates, email-gated downloads, CAPTCHAs, or anti-bot/JS challenges.
- Ignoring or circumventing `robots.txt`.
- Violating any source's Terms of Service.
- Mass-crawling or bulk-downloading a publisher's catalog.
- Persisting full copyrighted report text beyond transient processing.
- Republishing report content or producing near-verbatim summaries.
- Rotating IPs/User-Agents or spoofing identity to evade blocks.
- Hosting Paper Trail as a public service that serves ingested/proprietary report content to third parties. (The *source code* is open; the *content* it processes must not be redistributed.)
- Committing ingested content — source PDFs, extracted text, chunks, embeddings, or DB dumps — to the repository or any public artifact.

## 4. Residual risk statement

With the mitigations above, the dominant residual risks are **LLM output quality** (T1–T3, managed by grounding + human review) and **ToS/copyright interpretation** (L1–L2, managed by conservative defaults + counsel review). The architecture is intentionally **fail-closed**: when a source's status is uncertain, the system skips it rather than risk a violation. Keeping ingested content out of the repository — and not hosting a public instance that serves that content — keeps legal exposure low because nothing copyrighted is redistributed, even though the source code itself is open.
