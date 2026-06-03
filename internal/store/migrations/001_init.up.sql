CREATE EXTENSION IF NOT EXISTS vector;

-- enums
CREATE TYPE content_type   AS ENUM ('pdf', 'html', 'other');
CREATE TYPE fetch_decision  AS ENUM ('allowed', 'skipped', 'failed');
CREATE TYPE doc_state       AS ENUM (
  'discovered','deduped','scored','gated_check','fetched',
  'extracted','analyzed','ranked','in_review','approved','rejected','skipped','failed');
CREATE TYPE trust_tier      AS ENUM ('A','B','C','D','E','F');
CREATE TYPE license_type    AS ENUM ('public_domain','cc_by','cc_by_sa','open_other','all_rights_reserved','unknown');
CREATE TYPE sales_motion    AS ENUM ('enterprise','SMB','mass-market','developer-led','marketplace');
CREATE TYPE idea_label      AS ENUM (
  'enterprise_high_ticket','smb_saas','vertical_saas','developer_tool',
  'compliance_regtech','ai_workflow_automation','marketplace','consumer_mass_market','not_suitable');
CREATE TYPE review_state    AS ENUM ('pending','approved','rejected','needs_more_evidence','merged');

-- source registry
CREATE TABLE source (
  id                      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  domain                  TEXT NOT NULL UNIQUE,
  name                    TEXT NOT NULL,
  trust_tier              trust_tier NOT NULL DEFAULT 'F',
  trust_weight            NUMERIC(3,2) NOT NULL DEFAULT 0.30,
  allows_automated_access BOOLEAN NOT NULL DEFAULT FALSE,
  allows_excerpt_storage  BOOLEAN NOT NULL DEFAULT TRUE,
  allows_fulltext_storage BOOLEAN NOT NULL DEFAULT FALSE,
  license                 license_type NOT NULL DEFAULT 'unknown',
  robots_checked_at       TIMESTAMPTZ,
  policy_notes            TEXT,
  created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- documents
CREATE TABLE document (
  id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  source_id        BIGINT NOT NULL REFERENCES source(id),
  canonical_url    TEXT NOT NULL,
  url_hash         BYTEA NOT NULL UNIQUE,
  content_hash     BYTEA,
  content_type     content_type NOT NULL DEFAULT 'other',
  title            TEXT,
  authors          TEXT[],
  publisher        TEXT,
  published_at     DATE,
  language         TEXT,
  page_count       INT,
  discovered_via   TEXT,
  discovered_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  fetch_decision   fetch_decision,
  fetch_reason     TEXT,
  state            doc_state NOT NULL DEFAULT 'discovered',
  fulltext_retained BOOLEAN NOT NULL DEFAULT FALSE,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON document (source_id);
CREATE INDEX ON document (state);
CREATE INDEX ON document (content_hash);

-- chunks (embeddings persist; full text does not)
CREATE TABLE document_chunk (
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  document_id  BIGINT NOT NULL REFERENCES document(id) ON DELETE CASCADE,
  chunk_index  INT NOT NULL,
  char_start   INT,
  char_end     INT,
  token_count  INT,
  excerpt      TEXT,
  embedding    vector(1536),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (document_id, chunk_index)
);
CREATE INDEX ON document_chunk USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- disruption signals
CREATE TABLE disruption_signal (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  document_id BIGINT NOT NULL REFERENCES document(id) ON DELETE CASCADE,
  summary     TEXT NOT NULL,
  signal_type TEXT,
  confidence  NUMERIC(3,2),
  embedding   vector(1536),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON disruption_signal USING ivfflat (embedding vector_cosine_ops);

-- pain points
CREATE TABLE pain_point (
  id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  canonical_text TEXT NOT NULL,
  cluster_id     BIGINT,
  frequency      INT NOT NULL DEFAULT 1,
  embedding      vector(1536),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- controlled vocab
CREATE TABLE industry (
  id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name      TEXT NOT NULL UNIQUE,
  naics_code TEXT
);
CREATE TABLE region (
  id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name      TEXT NOT NULL UNIQUE,
  iso_code  TEXT
);
CREATE TABLE technology_driver (
  id   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE regulation_driver (
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name         TEXT NOT NULL UNIQUE,
  jurisdiction TEXT
);
CREATE TABLE customer_segment (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name        TEXT NOT NULL UNIQUE,
  sales_motion sales_motion
);

-- saas idea candidates
CREATE TABLE saas_idea_candidate (
  id                    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  idea_name             TEXT NOT NULL,
  one_sentence_pitch    TEXT NOT NULL,
  pain_point_id         BIGINT REFERENCES pain_point(id),
  disruption_driver     TEXT,
  target_customer       TEXT,
  buyer_persona         TEXT,
  sales_motion          sales_motion,
  label                 idea_label,
  high_ticket_potential  SMALLINT,
  mass_market_potential  SMALLINT,
  technical_feasibility  SMALLINT,
  market_urgency         SMALLINT,
  competition_risk       SMALLINT,
  data_availability      SMALLINT,
  mvp_complexity         SMALLINT,
  why_it_might_work     TEXT,
  why_it_might_fail     TEXT,
  possible_mvp          TEXT,
  first_10_customers    TEXT,
  validation_questions  TEXT[],
  cluster_id            BIGINT,
  embedding             vector(1536),
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON saas_idea_candidate USING ivfflat (embedding vector_cosine_ops);

CREATE TABLE idea_source_document (
  idea_id     BIGINT REFERENCES saas_idea_candidate(id) ON DELETE CASCADE,
  document_id BIGINT REFERENCES document(id),
  PRIMARY KEY (idea_id, document_id)
);

-- evidence / citations
CREATE TABLE evidence (
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  signal_id    BIGINT REFERENCES disruption_signal(id) ON DELETE CASCADE,
  idea_id      BIGINT REFERENCES saas_idea_candidate(id) ON DELETE CASCADE,
  document_id  BIGINT NOT NULL REFERENCES document(id),
  chunk_id     BIGINT REFERENCES document_chunk(id),
  excerpt      TEXT,
  citation_text TEXT NOT NULL,
  source_url   TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (signal_id IS NOT NULL OR idea_id IS NOT NULL)
);
CREATE INDEX ON evidence (idea_id);
CREATE INDEX ON evidence (signal_id);

-- ranking scores
CREATE TABLE ranking_score (
  idea_id          BIGINT PRIMARY KEY REFERENCES saas_idea_candidate(id) ON DELETE CASCADE,
  overall_score    SMALLINT NOT NULL,
  components       JSONB NOT NULL,
  problem_frequency INT,
  trust_weighted   NUMERIC(5,2),
  scored_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON ranking_score (overall_score DESC);

-- review status
CREATE TABLE review_status (
  idea_id     BIGINT PRIMARY KEY REFERENCES saas_idea_candidate(id) ON DELETE CASCADE,
  state       review_state NOT NULL DEFAULT 'pending',
  reviewer    TEXT,
  notes       TEXT,
  merged_into BIGINT REFERENCES saas_idea_candidate(id),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- audit
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
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  provider     TEXT NOT NULL,
  query_text   TEXT NOT NULL,
  query_hash   BYTEA NOT NULL UNIQUE,
  result_count INT,
  ran_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
