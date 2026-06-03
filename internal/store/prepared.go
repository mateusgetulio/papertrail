package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PreparedDocument is a document that has been fetched, extracted, and is
// available for analysis — the inventory surfaced by the Document Operator agent.
type PreparedDocument struct {
	ID           int64
	Title        string
	SourceName   string
	SourceDomain string
	CanonicalURL string
	ContentType  string
	State        string
	PublishedAt  *time.Time
	DiscoveredAt time.Time
	TrustTier    string
	TrustWeight  float64
	ChunkCount   int
}

// DocumentExistsByURLHash reports whether a document with this canonical-URL
// hash is already stored, so the operator can skip re-fetching and re-analysing
// URLs it has already ingested.
func DocumentExistsByURLHash(ctx context.Context, db *pgxpool.Pool, urlHash []byte) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM document WHERE url_hash = $1)`, urlHash).Scan(&exists)
	return exists, err
}

// GetPreparedDocuments returns documents that reached at least the 'extracted'
// state (i.e. have usable text), newest first. Pass limit <= 0 for all.
func GetPreparedDocuments(ctx context.Context, db *pgxpool.Pool, limit int) ([]PreparedDocument, error) {
	q := `
SELECT d.id, COALESCE(d.title,''), s.name, s.domain, d.canonical_url,
       d.content_type::text, d.state::text, d.published_at, d.discovered_at,
       s.trust_tier::text, s.trust_weight,
       (SELECT count(*) FROM document_chunk c WHERE c.document_id = d.id)
FROM document d
JOIN source s ON s.id = d.source_id
WHERE d.state IN ('extracted', 'analyzed')
ORDER BY d.discovered_at DESC`
	if limit > 0 {
		q += "\nLIMIT $1"
	}

	var args []any
	if limit > 0 {
		args = append(args, limit)
	}
	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PreparedDocument
	for rows.Next() {
		var d PreparedDocument
		if err := rows.Scan(&d.ID, &d.Title, &d.SourceName, &d.SourceDomain, &d.CanonicalURL,
			&d.ContentType, &d.State, &d.PublishedAt, &d.DiscoveredAt,
			&d.TrustTier, &d.TrustWeight, &d.ChunkCount); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
