# 02 — Source Discovery

How Paper Trail finds candidate documents: which search providers to use, which publishers are good (and safe) candidates, and the query patterns that surface disruption-themed reports.

> **Pricing/feature note:** API tiers and ToS change frequently. Treat the numbers below as planning estimates and **verify each provider's current pricing and Terms of Service at build time.**

## 1. Search / discovery providers compared

| Provider | Model | `site:` / `filetype:` operators | Freshness | Cost posture | Best for | Caveats |
|---|---|---|---|---|---|---|
| **Google Programmable Search (PSE)** | Custom Search JSON API | Yes (operators work) | High | Free quota (~100 queries/day) then paid per 1k | Precise `site: filetype:pdf` discovery | Low free quota; results scoped to your engine config |
| **Bing Web Search API** | REST | Yes | High | Paid per 1k (free trial tiers historically) | General web + operators | Microsoft has signaled changes to this API; confirm availability |
| **Brave Search API** | REST (independent index) | Partial (good keyword, operator support limited) | High | Generous free tier + cheap paid | Cost-effective first mover; independent index | Operator fidelity weaker than Google |
| **SerpAPI** | Scrapes Google/Bing SERPs as a service | Yes (full Google operators) | High | Paid per search (no/low free tier) | Highest-fidelity Google operators without running scrapers | Cost scales with volume; you depend on their scraping legality |
| **Tavily** | LLM-native search API | Limited operators | High | Free tier + paid | Agent-friendly, clean ranked results, summaries | Less control over precise operator queries |
| **Exa (metaphor)** | Neural/semantic + keyword search | Limited classic operators; strong semantic | High | Free tier + paid | Semantic discovery ("reports about X disruption") | Different query mental model; verify content licensing |
| **Common Crawl** | Static web corpus (WARC/CDX index) | N/A — you query the index/corpus | Low/lagging (monthly crawls) | Free data; you pay compute/storage | Zero per-query cost bulk discovery, backfill | Heavy infra; stale; you still must respect robots/ToS for re-fetch |

### Recommended starting combination (no providers yet)

1. **Primary discovery:** **Brave Search API** (free tier, cheap, independent index) **or** **Google PSE** free quota for highest operator fidelity. Start with whichever has a workable free quota for your volume.
2. **Semantic augmentation:** **Exa** or **Tavily** free tier for "find reports *about* topic X" where exact operators fall short.
3. **Bulk/backfill (later):** **Common Crawl** once volume justifies the infra, for zero-marginal-cost historical discovery.
4. **Upgrade path:** add **SerpAPI** if/when you need high-fidelity Google operator queries at volume and the cost is justified.

Design discovery behind a `SearchProvider` interface so providers are swappable and can be combined/deduped.

## 2. Candidate publishers / sources

Grouped by type. All are accessed **only for public, non-gated pages**; gated or email-required downloads are skipped (see `docs/03-compliance.md`).

### Consulting firms (public insight pages + public PDFs)
- **PwC** (`pwc.com`) — public reports/insights, many open PDFs.
- **Capgemini** (`capgemini.com`) — Research Institute publishes open PDFs.
- **Deloitte** (`deloitte.com` / Deloitte Insights) — large public library.
- **McKinsey** (`mckinsey.com` / McKinsey Global Institute) — public articles; some reports gated.
- **BCG** (`bcg.com`) — public insights.
- **Bain** (`bain.com`) — public reports incl. annual studies.
- **Accenture** (`accenture.com`) — public research PDFs.

> Consulting sites are **higher ToS sensitivity**. Favor clearly-public PDFs and insight pages; honor `robots.txt`; never touch gated downloads. Confirm each firm's ToS at build time.

### Analyst firms (public pages only)
- **Gartner** (`gartner.com`) — mostly paywalled; use **public press releases / free public pages only**.
- **Forrester** (`forrester.com`) — similar; **public pages / press releases only**.

> Treat Gartner/Forrester as **low-yield, high-caution**. Most value is gated — do not attempt it. Use only their public predictions/press content.

### Multilateral & policy institutions (high trust, open-access friendly)
- **World Economic Forum** (`weforum.org`) — many open reports.
- **OECD** (`oecd.org`) — open data + reports.
- **World Bank** (`worldbank.org` / Open Knowledge Repository — explicitly open-licensed, often CC BY).
- **IMF** (`imf.org`) — public working papers / reports.

> These are the **best-yield, lowest-risk** sources. Many carry open licenses (e.g., World Bank OKR is frequently CC BY) that even permit broader reuse — still cite properly.

### Government & official statistics
- National/EU/UN agency reports, regulators, statistics offices (e.g., `.gov`, `europa.eu`, `un.org`). Generally public-domain or openly licensed; high trust.

### Industry associations & standards bodies
- Sector associations, trade bodies, standards orgs publishing public outlooks and white papers.

### Startup / VC reports
- Public "state of" reports from VCs and platforms (e.g., annual industry/state-of-software reports). Often open and marketing-driven (good signal on emerging categories). Verify each report's redistribution terms.

## 3. Source trust tiers (feeds trust scoring in `docs/04`)

| Tier | Examples | Default trust weight |
|---|---|---|
| **A — Multilateral / Gov / Open-license** | World Bank, OECD, IMF, WEF, gov stats | Highest |
| **B — Major consulting (public)** | PwC, Deloitte, McKinsey, BCG, Bain, Capgemini, Accenture | High |
| **C — Industry associations / standards** | Trade bodies, standards orgs | Medium-high |
| **D — VC / startup reports** | VC "state of" reports | Medium (marketing bias) |
| **E — Analyst public pages** | Gartner/Forrester press releases | Medium (sparse, gated core) |
| **F — Other reputable industry pubs** | Trade press, vendor research | Lower; corroborate before trusting |

## 4. Query patterns

Templates the discovery stage expands across sources, topics, and years. Operator support varies by provider (see table); for weak-operator providers, fall back to keyword + post-filter on URL/extension/domain.

### Site + filetype targeted
```
site:pwc.com filetype:pdf "disruption" "report"
site:capgemini.com filetype:pdf "AI" "industry"
site:deloitte.com filetype:pdf "outlook" "2026"
site:weforum.org filetype:pdf "future of" "jobs" OR "industry"
site:oecd.org filetype:pdf "digital transformation"
site:worldbank.org filetype:pdf "technology" "report"
```

### Cross-site thematic
```
"market disruption" "white paper" filetype:pdf
"digital transformation" "pain points" "report"
"industry outlook" "challenges" "2026"
"emerging technology" "adoption barriers" filetype:pdf
"regulatory" "compliance burden" "report" filetype:pdf
"state of" "industry" report 2026 filetype:pdf
```

### Pain-point / opportunity oriented
```
"manual process" OR "inefficiency" "report" filetype:pdf
"underserved" OR "unmet need" "market" filetype:pdf
"automation opportunity" "industry" filetype:pdf
"data silos" OR "fragmented" workflow report filetype:pdf
```

### Query expansion strategy
- **Dimensions:** {source domain} × {theme keyword} × {industry} × {year}.
- **Year sweeping:** rotate recent years (e.g., 2024–2027) to capture "outlook" reports.
- **Semantic fallback (Exa/Tavily):** natural-language prompts like *"recent consulting reports describing operational pain points in healthcare logistics"* where exact operators underperform.
- **Budgeting:** cap queries/day per provider to stay in free tiers; persist seen-query fingerprints to avoid re-spending quota.

## 5. Output of this stage

Each discovery hit becomes a candidate row with: `url`, `title` (from SERP), `source_domain`, `discovered_via` (provider + query), `content_type_guess` (pdf/html), `discovered_at`. These flow into **URL deduplication → trust scoring → allowed-fetch check** (see `docs/04-architecture.md`).
