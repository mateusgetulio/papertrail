# 05 — Data Model

Postgres schema (with `pgvector`) for Paper Trail. DDL sketches below are illustrative — refine types/constraints during migration authoring. Design reflects the compliance posture: **metadata and derived data are persistent; full proprietary text is not.**

## 1. Entity overview

```
source 1───* document 1───* document_chunk
  │                │
  │                ├──* disruption_signal *──── pain_point
  │                │         │                     │
  │                │         ├──* (signal_industry) industry
  │                │         ├──* (signal_region)   region
  │                │         ├──* technology_driver
  │                │         └──* regulation_driver
  │                │
  │                └──* evidence  (links signals/ideas → chunk + short excerpt)
  │
saas_idea_candidate 1───1 ranking_score
        │  *──* customer_segment
        │  *──* industry / region (via join tables)
        │  *──* source_document (provenance)
        └──1 review_status
```

## 2. Reference / enum types

```sql
CREATE TYPE content_type      AS ENUM ('pdf', 'html', 'other');
CREATE TYPE fetch_decision    AS ENUM ('allowed', 'skipped', 'failed');
CREATE TYPE doc_state         AS ENUM (
  'discovered','deduped','scored','gated_check','fetched',
  'extracted','analyzed','ranked','in_review','approved','rejected','skipped','failed');
CREATE TYPE trust_tier        AS ENUM ('A','B','C','D','E','F');
CREATE TYPE license_type      AS ENUM ('public_domain','cc_by','cc_by_sa','open_other','all_rights_reserved','unknown');
CREATE TYPE sales_motion      AS ENUM ('enterprise','SMB','mass-market','developer-led','marketplace');
CREATE TYPE idea_label        AS ENUM (
  'enterprise_high_ticket','smb_saas','vertical_saas','developer_tool',
  'compliance_regtech','ai_workflow_automation','marketplace','consumer_mass_market','not_suitable');
CREATE TYPE review_state      AS ENUM ('pending','approved','rejected','needs_more_evidence','merged');
```

## 3. Core tables

### source
```sql
CREATE TABLE source (
  id              BIGGENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  domain          TEXT NOT NULL UNIQUE,            -- e.g., 'pwc.com'
  name            TEXT NOT NULL,                   -- 'PwC'
  trust_tier      trust_tier NOT NULL DEFAULT 'F',
  trust_weight    NUMERIC(3,2) NOT NULL DEFAULT 0.30,  -- 0..1, feeds ranking
  -- compliance policy registry (see docs/03 §4)
  allows_automated_access BOOLEAN NOT NULL DEFAULT FALSE,
  allows_excerpt_storage  BOOLEAN NOT NULL DEFAULT TRUE,
  allows_fulltext_storage BOOLEAN NOT NULL DEFAULT FALSE,
  license         license_type NOT NULL DEFAULT 'unknown',
  robots_checked_at TIMESTAMPTZ,
  policy_notes    TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```
> Note: `BIGGENERATED` above is shorthand — use `BIGINT GENERATED ALWAYS AS IDENTITY` in real DDL.

### document
```sql
CREATE TABLE document (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  source_id       BIGINT NOT NULL REFERENCES source(id),
  canonical_url   TEXT NOT NULL,
  url_hash        BYTEA NOT NULL UNIQUE,           -- dedup key
  content_hash    BYTEA,                           -- post-download dedup
  content_type    content_type NOT NULL DEFAULT 'other',
  title           TEXT,
  authors         TEXT[],
  publisher       TEXT,
  published_at    DATE,
  language        TEXT,
  page_count      INT,
  -- discovery + compliance audit
  discovered_via  TEXT,                            -- provider + query
  discovered_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  fetch_decision  fetch_decision,
  fetch_reason    TEXT,                            -- why skipped/failed
  state           doc_state NOT NULL DEFAULT 'discovered',
  -- retention: full text is transient; we keep only a flag + where it lived
  fulltext_retained BOOLEAN NOT NULL DEFAULT FALSE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON document (source_id);
CREATE INDEX ON document (state);
CREATE INDEX ON document (content_hash);
```

### document_chunk
```sql
CREATE TABLE document_chunk (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  document_id   BIGINT NOT NULL REFERENCES document(id) ON DELETE CASCADE,
  chunk_index   INT NOT NULL,
  char_start    INT,                               -- offsets for citation, not the text itself
  char_end      INT,
  token_count   INT,
  excerpt       TEXT,                              -- SHORT, capped excerpt for evidence only
  embedding     vector(1536),                      -- OpenAI embedding dim (adjust to model)
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (document_id, chunk_index)
);
CREATE INDEX ON document_chunk USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
```
> `excerpt` is length-capped at the application layer (see `docs/03 §6`). Full chunk text is not persisted.

## 4. Signal & dimension tables

### disruption_signal
```sql
CREATE TABLE disruption_signal (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  document_id   BIGINT NOT NULL REFERENCES document(id) ON DELETE CASCADE,
  summary       TEXT NOT NULL,                     -- original, transformative summary
  signal_type   TEXT,                              -- e.g., 'technology_shift','regulatory','demand_shift'
  confidence    NUMERIC(3,2),                      -- 0..1 from LLM
  embedding     vector(1536),                      -- for cross-doc clustering
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON disruption_signal USING ivfflat (embedding vector_cosine_ops);
```

### pain_point
```sql
CREATE TABLE pain_point (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  canonical_text TEXT NOT NULL,                    -- normalized statement
  cluster_id    BIGINT,                            -- groups duplicates across docs
  frequency     INT NOT NULL DEFAULT 1,            -- # of distinct source docs mentioning it
  embedding     vector(1536),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE signal_pain_point (
  signal_id     BIGINT NOT NULL REFERENCES disruption_signal(id) ON DELETE CASCADE,
  pain_point_id BIGINT NOT NULL REFERENCES pain_point(id) ON DELETE CASCADE,
  PRIMARY KEY (signal_id, pain_point_id)
);
```

### industry / region (controlled vocabularies)
```sql
CREATE TABLE industry (
  id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name  TEXT NOT NULL UNIQUE,                       -- e.g., 'Healthcare', 'Logistics'
  naics_code TEXT
);
CREATE TABLE region (
  id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name  TEXT NOT NULL UNIQUE,                        -- country or region
  iso_code TEXT
);
```

### technology_driver / regulation_driver
```sql
CREATE TABLE technology_driver (
  id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name  TEXT NOT NULL UNIQUE                          -- 'Generative AI','Edge compute'
);
CREATE TABLE regulation_driver (
  id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name  TEXT NOT NULL UNIQUE,                         -- 'EU AI Act','GDPR','Basel IV'
  jurisdiction TEXT
);
```

### customer_segment
```sql
CREATE TABLE customer_segment (
  id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name  TEXT NOT NULL UNIQUE,                         -- 'Enterprise IT','SMB ops','Developers'
  sales_motion sales_motion
);
```

### Signal ↔ dimension join tables
```sql
CREATE TABLE signal_industry          (signal_id BIGINT REFERENCES disruption_signal(id) ON DELETE CASCADE, industry_id BIGINT REFERENCES industry(id), PRIMARY KEY(signal_id,industry_id));
CREATE TABLE signal_region            (signal_id BIGINT REFERENCES disruption_signal(id) ON DELETE CASCADE, region_id   BIGINT REFERENCES region(id),   PRIMARY KEY(signal_id,region_id));
CREATE TABLE signal_technology_driver (signal_id BIGINT REFERENCES disruption_signal(id) ON DELETE CASCADE, tech_id     BIGINT REFERENCES technology_driver(id), PRIMARY KEY(signal_id,tech_id));
CREATE TABLE signal_regulation_driver (signal_id BIGINT REFERENCES disruption_signal(id) ON DELETE CASCADE, reg_id      BIGINT REFERENCES regulation_driver(id), PRIMARY KEY(signal_id,reg_id));
```

## 5. Opportunity tables

### saas_idea_candidate
```sql
CREATE TABLE saas_idea_candidate (
  id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  idea_name         TEXT NOT NULL,
  one_sentence_pitch TEXT NOT NULL,
  pain_point_id     BIGINT REFERENCES pain_point(id),
  disruption_driver TEXT,
  target_customer   TEXT,
  buyer_persona     TEXT,
  sales_motion      sales_motion,
  label             idea_label,
  -- sub-scores (1..10), mirrored into output JSON (docs/08)
  high_ticket_potential   SMALLINT,
  mass_market_potential   SMALLINT,
  technical_feasibility   SMALLINT,
  market_urgency          SMALLINT,
  competition_risk        SMALLINT,
  data_availability       SMALLINT,
  mvp_complexity          SMALLINT,
  why_it_might_work TEXT,
  why_it_might_fail TEXT,
  possible_mvp      TEXT,
  first_10_customers TEXT,
  validation_questions TEXT[],
  cluster_id        BIGINT,                          -- dedup grouping for similar ideas
  embedding         vector(1536),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON saas_idea_candidate USING ivfflat (embedding vector_cosine_ops);

-- provenance: which documents support this idea
CREATE TABLE idea_source_document (
  idea_id     BIGINT REFERENCES saas_idea_candidate(id) ON DELETE CASCADE,
  document_id BIGINT REFERENCES document(id),
  PRIMARY KEY (idea_id, document_id)
);
CREATE TABLE idea_industry (idea_id BIGINT REFERENCES saas_idea_candidate(id) ON DELETE CASCADE, industry_id BIGINT REFERENCES industry(id), PRIMARY KEY(idea_id,industry_id));
CREATE TABLE idea_region   (idea_id BIGINT REFERENCES saas_idea_candidate(id) ON DELETE CASCADE, region_id   BIGINT REFERENCES region(id),   PRIMARY KEY(idea_id,region_id));
CREATE TABLE idea_segment  (idea_id BIGINT REFERENCES saas_idea_candidate(id) ON DELETE CASCADE, segment_id  BIGINT REFERENCES customer_segment(id), PRIMARY KEY(idea_id,segment_id));
```

### evidence / citation
```sql
CREATE TABLE evidence (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  -- evidence can attach to a signal and/or an idea
  signal_id     BIGINT REFERENCES disruption_signal(id) ON DELETE CASCADE,
  idea_id       BIGINT REFERENCES saas_idea_candidate(id) ON DELETE CASCADE,
  document_id   BIGINT NOT NULL REFERENCES document(id),
  chunk_id      BIGINT REFERENCES document_chunk(id),
  excerpt       TEXT,                                -- short, capped, attributed
  citation_text TEXT NOT NULL,                       -- human-readable citation
  source_url    TEXT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (signal_id IS NOT NULL OR idea_id IS NOT NULL)
);
CREATE INDEX ON evidence (idea_id);
CREATE INDEX ON evidence (signal_id);
```

### ranking_score
```sql
CREATE TABLE ranking_score (
  idea_id        BIGINT PRIMARY KEY REFERENCES saas_idea_candidate(id) ON DELETE CASCADE,
  overall_score  SMALLINT NOT NULL,                  -- 1..100
  components     JSONB NOT NULL,                      -- per-criterion weighted breakdown (explainability)
  problem_frequency INT,                              -- cross-source count used in scoring
  trust_weighted    NUMERIC(5,2),
  scored_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON ranking_score (overall_score DESC);
```

### review_status
```sql
CREATE TABLE review_status (
  idea_id      BIGINT PRIMARY KEY REFERENCES saas_idea_candidate(id) ON DELETE CASCADE,
  state        review_state NOT NULL DEFAULT 'pending',
  reviewer     TEXT,
  notes        TEXT,
  merged_into  BIGINT REFERENCES saas_idea_candidate(id),  -- when deduped by a human
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## 6. Audit & ops tables

```sql
CREATE TABLE fetch_audit (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  document_id BIGINT REFERENCES document(id),
  url         TEXT NOT NULL,
  decision    fetch_decision NOT NULL,
  reason      TEXT,
  robots_ok   BOOLEAN,
  gated       BOOLEAN,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE search_query_log (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  provider      TEXT NOT NULL,
  query_text    TEXT NOT NULL,
  query_hash    BYTEA NOT NULL UNIQUE,               -- avoid re-spending quota
  result_count  INT,
  ran_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## 7. Notes & rationale

- **Vector dim** (`vector(1536)`) matches a common OpenAI embedding size; set to your chosen model's dimension.
- **Pain-point clustering** (`pain_point.cluster_id` + `frequency`) is what powers "repeated pain points across sources" in scoring.
- **No full-text column** anywhere persistent — only capped `excerpt` fields and offsets, per `docs/03`.
- **Explainability** lives in `ranking_score.components` (JSONB) so the dashboard can show why a score landed where it did.
- **Idempotency** comes from `document.url_hash`/`content_hash` and `search_query_log.query_hash`.
