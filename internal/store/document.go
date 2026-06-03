package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// InsertDocumentParams holds all values needed to create a document row.
type InsertDocumentParams struct {
	SourceID      int64
	CanonicalURL  string
	URLHash       []byte
	ContentHash   []byte
	ContentType   string // "pdf" | "html" | "other"
	Title         string
	Authors       []string
	Publisher     string
	PublishedAt   *time.Time
	Language      string
	PageCount     int
	DiscoveredVia string
	State         string // "fetched" after ingestion
}

// InsertDocument upserts a document row, returning its ID.
// On url_hash conflict it updates content_hash + state.
func InsertDocument(ctx context.Context, db *pgxpool.Pool, p InsertDocumentParams) (int64, error) {
	const q = `
INSERT INTO document
  (source_id, canonical_url, url_hash, content_hash, content_type,
   title, authors, publisher, published_at, language, page_count,
   discovered_via, state, updated_at)
VALUES
  ($1,$2,$3,$4,$5::content_type,$6,$7,$8,$9,$10,$11,$12,$13::doc_state,now())
ON CONFLICT (url_hash) DO UPDATE
  SET content_hash = EXCLUDED.content_hash,
      state        = EXCLUDED.state,
      updated_at   = now()
RETURNING id`

	var id int64
	err := db.QueryRow(ctx, q,
		p.SourceID, p.CanonicalURL, p.URLHash, p.ContentHash,
		p.ContentType, nullStr(p.Title), p.Authors, nullStr(p.Publisher),
		p.PublishedAt, nullStr(p.Language), p.PageCount,
		p.DiscoveredVia, p.State,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert document: %w", err)
	}
	return id, nil
}

// InsertChunk persists a single document chunk (excerpt + embedding).
func InsertChunk(ctx context.Context, db *pgxpool.Pool,
	docID int64, idx, charStart, charEnd, tokenCount int,
	excerpt string, embedding []float32,
) error {
	const q = `
INSERT INTO document_chunk
  (document_id, chunk_index, char_start, char_end, token_count, excerpt, embedding)
VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (document_id, chunk_index) DO UPDATE
  SET excerpt   = EXCLUDED.excerpt,
      embedding = EXCLUDED.embedding`

	_, err := db.Exec(ctx, q, docID, idx, charStart, charEnd, tokenCount, excerpt, fmtVec(embedding))
	if err != nil {
		return fmt.Errorf("insert chunk %d: %w", idx, err)
	}
	return nil
}

// UpdateDocumentState transitions a document to a new state.
func UpdateDocumentState(ctx context.Context, db *pgxpool.Pool, docID int64, state string) error {
	_, err := db.Exec(ctx,
		`UPDATE document SET state=$1::doc_state, updated_at=now() WHERE id=$2`,
		state, docID)
	return err
}

// InsertFetchAudit records every fetch decision (allowed/skipped) per docs/03 §8.
func InsertFetchAudit(ctx context.Context, db *pgxpool.Pool,
	docID *int64, rawURL, decision, reason string, robotsOK, gated bool,
) error {
	const q = `
INSERT INTO fetch_audit (document_id, url, decision, reason, robots_ok, gated)
VALUES ($1,$2,$3::fetch_decision,$4,$5,$6)`
	_, err := db.Exec(ctx, q, docID, rawURL, decision, reason, robotsOK, gated)
	return err
}

// EnsureSource upserts a source row and returns its ID.
func EnsureSource(ctx context.Context, db *pgxpool.Pool, domain, name, tier string) (int64, error) {
	const q = `
INSERT INTO source (domain, name, trust_tier, allows_automated_access)
VALUES ($1,$2,$3::trust_tier,true)
ON CONFLICT (domain) DO UPDATE SET name=EXCLUDED.name
RETURNING id`
	var id int64
	err := db.QueryRow(ctx, q, domain, name, tier).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("ensure source %s: %w", domain, err)
	}
	return id, nil
}

// LogSearchQuery records a search query fingerprint to prevent re-spending quota.
func LogSearchQuery(ctx context.Context, db *pgxpool.Pool, provider, query string, queryHash []byte, resultCount int) error {
	const q = `
INSERT INTO search_query_log (provider, query_text, query_hash, result_count)
VALUES ($1,$2,$3,$4)
ON CONFLICT (query_hash) DO NOTHING`
	_, err := db.Exec(ctx, q, provider, query, queryHash, resultCount)
	return err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// fmtVec formats a float32 slice as a pgvector literal string.
func fmtVec(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	b := make([]byte, 0, len(v)*8+2)
	b = append(b, '[')
	for i, f := range v {
		if i > 0 {
			b = append(b, ',')
		}
		b = fmt.Appendf(b, "%g", f)
	}
	b = append(b, ']')
	return string(b)
}
