# 08 — Output Format

The structured contract every SaaS-idea candidate must satisfy. Produced by the agent (`docs/07`) via OpenAI structured-output mode, validated in Go, and persisted per `docs/05`.

## 1. Candidate JSON schema

```json
{
  "idea_name": "",
  "one_sentence_pitch": "",
  "source_documents": [],
  "industries": [],
  "countries_or_regions": [],
  "disruption_driver": "",
  "pain_point": "",
  "target_customer": "",
  "buyer_persona": "",
  "sales_motion": "enterprise | SMB | mass-market | developer-led | marketplace",
  "high_ticket_potential": 1,
  "mass_market_potential": 1,
  "technical_feasibility": 1,
  "market_urgency": 1,
  "competition_risk": 1,
  "data_availability": 1,
  "mvp_complexity": 1,
  "overall_score": 1,
  "why_it_might_work": "",
  "why_it_might_fail": "",
  "possible_mvp": "",
  "first_10_customers": "",
  "validation_questions": [],
  "citations": []
}
```

## 2. Field definitions & validation

| Field | Type | Rule |
|---|---|---|
| `idea_name` | string | 3–80 chars; specific, not generic |
| `one_sentence_pitch` | string | ≤ 200 chars; single sentence |
| `source_documents` | string[] | ≥ 1; URLs/IDs of supporting docs (must exist in `document`) |
| `industries` | string[] | from controlled vocab where possible |
| `countries_or_regions` | string[] | may be empty if global/unspecified |
| `disruption_driver` | string | the shift creating the opportunity |
| `pain_point` | string | crisp, buyer-centric problem statement |
| `target_customer` | string | who uses it |
| `buyer_persona` | string | who pays / owns budget |
| `sales_motion` | enum | one of `enterprise`, `SMB`, `mass-market`, `developer-led`, `marketplace` |
| `high_ticket_potential` | int 1–10 | per `docs/06` |
| `mass_market_potential` | int 1–10 | per `docs/06` |
| `technical_feasibility` | int 1–10 | higher = easier to build |
| `market_urgency` | int 1–10 | |
| `competition_risk` | int 1–10 | higher = more/stronger competition (risk) |
| `data_availability` | int 1–10 | |
| `mvp_complexity` | int 1–10 | higher = more complex MVP |
| `overall_score` | int 1–100 | computed deterministically (`docs/06`), not free-form by LLM |
| `why_it_might_work` | string | grounded reasoning |
| `why_it_might_fail` | string | honest risks incl. generic/consulting traps |
| `possible_mvp` | string | concrete first build |
| `first_10_customers` | string | where the first 10 come from |
| `validation_questions` | string[] | 3–7 questions to test the idea |
| `citations` | object[] | ≥ 1; see citation shape below |

### Citation object shape (richer than a bare string)
```json
{
  "document_id": "",
  "source_url": "",
  "title": "",
  "publisher": "",
  "published_at": "",
  "excerpt": "",            // SHORT, capped, attributed (docs/03 §6)
  "chunk_index": 0
}
```

### Validation gates (Go, before persist)
- `sales_motion` ∈ enum; all 1–10 fields integers in range; `overall_score` matches the recomputed rubric value (±0 — recompute, don't trust the model).
- `citations` non-empty and every `source_url` resolves to a stored `document`.
- `excerpt` length ≤ cap; reject if any citation lacks attribution.
- `idea_name`/`pitch` fail a "too generic" lint (e.g., bans bare "AI assistant for everyone").

## 3. Worked example A — strong, enterprise/regtech

```json
{
  "idea_name": "AuditTrail for EU AI Act",
  "one_sentence_pitch": "Automated evidence collection and conformity documentation for mid-market AI vendors subject to the EU AI Act.",
  "source_documents": ["doc:1042", "doc:1187", "doc:1203"],
  "industries": ["Software", "Financial Services", "Healthcare"],
  "countries_or_regions": ["European Union"],
  "disruption_driver": "Phased entry-into-force of the EU AI Act creating mandatory conformity obligations for high-risk AI systems.",
  "pain_point": "Mid-market AI vendors lack the legal/ops capacity to continuously gather and maintain Act-required technical documentation and risk evidence.",
  "target_customer": "Mid-market companies shipping AI features into regulated EU markets",
  "buyer_persona": "Head of Compliance / VP Engineering",
  "sales_motion": "enterprise",
  "high_ticket_potential": 7,
  "mass_market_potential": 3,
  "technical_feasibility": 6,
  "market_urgency": 9,
  "competition_risk": 6,
  "data_availability": 6,
  "mvp_complexity": 6,
  "overall_score": 63,
  "why_it_might_work": "A hard regulatory deadline forces adoption, the budget owner is clear, and the documentation burden is recurring and software-shaped.",
  "why_it_might_fail": "Big GRC incumbents may add Act modules; scope could drift toward consulting if customers want bespoke legal interpretation.",
  "possible_mvp": "Connectors that pull model/data lineage from common ML stacks and auto-generate the Act's technical documentation templates with gap flags.",
  "first_10_customers": "Series A/B EU AI startups in fintech/healthtech already asking customers about Act readiness.",
  "validation_questions": [
    "Will compliance leads pay for automation vs. using consultants?",
    "Which Act artifacts are most painful to produce today?",
    "Can we pull lineage from the top 3 ML platforms reliably?",
    "What is the willingness-to-pay range at mid-market?"
  ],
  "citations": [
    {
      "document_id": "doc:1042",
      "source_url": "https://example.org/eu-ai-act-readiness-report.pdf",
      "title": "AI Act Readiness Outlook",
      "publisher": "Example Institute",
      "published_at": "2026-01-15",
      "excerpt": "...mid-market vendors report documentation as the top compliance burden...",
      "chunk_index": 12
    }
  ]
}
```

## 4. Worked example B — correctly rejected (consulting problem)

```json
{
  "idea_name": "Org Redesign Advisor",
  "one_sentence_pitch": "A platform that tells enterprises how to restructure teams after digital transformation.",
  "source_documents": ["doc:990"],
  "industries": ["Cross-industry"],
  "countries_or_regions": [],
  "disruption_driver": "Reports note org structures lag behind digital transformation.",
  "pain_point": "Companies struggle to reorganize roles and reporting lines after adopting new technology.",
  "target_customer": "Large enterprises undergoing transformation",
  "buyer_persona": "Chief Transformation Officer",
  "sales_motion": "enterprise",
  "high_ticket_potential": 6,
  "mass_market_potential": 2,
  "technical_feasibility": 4,
  "market_urgency": 4,
  "competition_risk": 7,
  "data_availability": 3,
  "mvp_complexity": 7,
  "overall_score": 21,
  "why_it_might_work": "Transformation budgets are large and the pain is real and widely cited.",
  "why_it_might_fail": "The remedy is fundamentally advisory/change-management judgment, not a recurring software workflow — a classic 'consulting problem, not software problem'. No clean data to power it; output is one-off.",
  "possible_mvp": "N/A — flagged not suitable for SaaS in current form.",
  "first_10_customers": "Unclear; buyers would likely engage consultancies instead.",
  "validation_questions": [
    "Is there a recurring, data-driven workflow software could own here?",
    "Would buyers pay for a tool vs. a consulting engagement?"
  ],
  "citations": [
    {
      "document_id": "doc:990",
      "source_url": "https://example.com/transformation-outlook.pdf",
      "title": "Transformation Outlook 2026",
      "publisher": "Example Advisory",
      "published_at": "2026-02-01",
      "excerpt": "...organizational structures often lag technology adoption...",
      "chunk_index": 5
    }
  ]
}
```
> Label: `not_suitable`. Surfaced only in audit views, not the active queue (score < 30, see `docs/06 §7`).

## 5. Notes

- The LLM may *propose* `overall_score`, but Go **recomputes** it from sub-scores via `docs/06` and overwrites — the model's number is never trusted as final.
- Output is validated against a JSON Schema; invalid → one repair attempt → dead-letter (`docs/07 §8`).
- The persisted form spreads across `saas_idea_candidate`, `evidence`, `ranking_score`, and join tables (`docs/05`); this JSON is the export/interchange shape (also used for CSV/Notion/Airtable export).
