package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DocRecord is a minimal view of a document row for analysis.
type DocRecord struct {
	ID           int64
	CanonicalURL string
	Title        string
	ContentType  string
	State        string
	Domain       string
	TrustTier    string
}

// ChunkRecord is a single chunk row.
type ChunkRecord struct {
	ID         int64
	ChunkIndex int
	Excerpt    string
	Embedding  []float32
}

// GetExtractedDocuments returns documents in state 'extracted' (ready to analyse).
func GetExtractedDocuments(ctx context.Context, db *pgxpool.Pool, limit int) ([]DocRecord, error) {
	const q = `
SELECT d.id, d.canonical_url, COALESCE(d.title,''), d.content_type::text, d.state::text,
       s.domain, s.trust_tier::text
FROM document d
JOIN source s ON s.id = d.source_id
WHERE d.state = 'extracted'
ORDER BY d.created_at ASC
LIMIT $1`
	rows, err := db.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []DocRecord
	for rows.Next() {
		var d DocRecord
		if err := rows.Scan(&d.ID, &d.CanonicalURL, &d.Title, &d.ContentType, &d.State, &d.Domain, &d.TrustTier); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// GetChunksForDocument returns all chunks for a document ordered by index.
func GetChunksForDocument(ctx context.Context, db *pgxpool.Pool, docID int64) ([]ChunkRecord, error) {
	const q = `
SELECT id, chunk_index, COALESCE(excerpt,''), embedding
FROM document_chunk
WHERE document_id = $1
ORDER BY chunk_index ASC`
	rows, err := db.Query(ctx, q, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chunks []ChunkRecord
	for rows.Next() {
		var c ChunkRecord
		var embRaw any
		if err := rows.Scan(&c.ID, &c.ChunkIndex, &c.Excerpt, &embRaw); err != nil {
			return nil, err
		}
		c.Embedding = parseVecAny(embRaw)
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

// InsertDisruptionSignal persists one signal row, returning its ID.
func InsertDisruptionSignal(ctx context.Context, db *pgxpool.Pool,
	docID int64, summary, signalType, painPoint string,
	industries, regions []string, chunkRefs []int,
	confidence float64, embedding []float32,
) (int64, error) {
	const q = `
INSERT INTO disruption_signal
  (document_id, summary, signal_type, industries, regions, chunk_refs, confidence, embedding)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
RETURNING id`
	var id int64
	err := db.QueryRow(ctx, q,
		docID, summary, signalType, industries, regions, chunkRefs,
		confidence, fmtVec(embedding),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert signal: %w", err)
	}
	return id, nil
}

// InsertSaaSIdeaCandidate persists a candidate idea, returning its ID.
func InsertSaaSIdeaCandidate(ctx context.Context, db *pgxpool.Pool,
	ideaName, pitch, driver, painPoint, targetCustomer, buyerPersona string,
	salesMotion, label string,
	scores map[string]int,
	whyWork, whyFail, possibleMVP, first10 string,
	validationQs []string,
	industries, countries []string,
	embedding []float32,
) (int64, error) {
	const q = `
INSERT INTO saas_idea_candidate
  (idea_name, one_sentence_pitch, disruption_driver, pain_point, target_customer, buyer_persona,
   sales_motion, label, high_ticket_potential, mass_market_potential, technical_feasibility,
   market_urgency, competition_risk, data_availability, mvp_complexity,
   why_it_might_work, why_it_might_fail, possible_mvp, first_10_customers,
   validation_questions, industries, countries_or_regions, embedding)
VALUES
  ($1,$2,$3,$4,$5,$6,
   $7::sales_motion,$8::idea_label,$9,$10,$11,
   $12,$13,$14,$15,
   $16,$17,$18,$19,
   $20,$21,$22,$23)
RETURNING id`
	var id int64
	err := db.QueryRow(ctx, q,
		ideaName, pitch, driver, painPoint, targetCustomer, buyerPersona,
		salesMotion, label,
		scores["high_ticket_potential"], scores["mass_market_potential"], scores["technical_feasibility"],
		scores["market_urgency"], scores["competition_risk"], scores["data_availability"], scores["mvp_complexity"],
		whyWork, whyFail, possibleMVP, first10,
		validationQs, industries, countries, fmtVec(embedding),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert idea: %w", err)
	}
	return id, nil
}

// InsertIdeaSourceDoc links an idea to its source document.
func InsertIdeaSourceDoc(ctx context.Context, db *pgxpool.Pool, ideaID, docID int64) error {
	_, err := db.Exec(ctx,
		`INSERT INTO idea_source_document (idea_id, document_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
		ideaID, docID)
	return err
}

// InsertEvidence persists a citation linking an idea to a chunk.
func InsertEvidence(ctx context.Context, db *pgxpool.Pool,
	ideaID, docID int64, chunkID *int64, excerpt, citationText, sourceURL string,
) error {
	_, err := db.Exec(ctx, `
INSERT INTO evidence (idea_id, document_id, chunk_id, excerpt, citation_text, source_url)
VALUES ($1,$2,$3,$4,$5,$6)`,
		ideaID, docID, chunkID, excerpt, citationText, sourceURL)
	return err
}

// InsertRankingScore upserts the scoring components for an idea.
func InsertRankingScore(ctx context.Context, db *pgxpool.Pool,
	ideaID int64, overallScore int, components map[string]any,
	problemFrequency int, trustWeighted float64,
) error {
	compJSON, _ := json.Marshal(components)
	_, err := db.Exec(ctx, `
INSERT INTO ranking_score (idea_id, overall_score, components, problem_frequency, trust_weighted)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (idea_id) DO UPDATE
  SET overall_score=$2, components=$3, problem_frequency=$4, trust_weighted=$5, scored_at=now()`,
		ideaID, overallScore, compJSON, problemFrequency, trustWeighted)
	return err
}

// InsertReviewStatus creates an initial 'pending' review status row for an idea.
func InsertReviewStatus(ctx context.Context, db *pgxpool.Pool, ideaID int64) error {
	_, err := db.Exec(ctx, `
INSERT INTO review_status (idea_id, state) VALUES ($1,'pending')
ON CONFLICT (idea_id) DO NOTHING`,
		ideaID)
	return err
}

// GetExistingIdeaEmbeddings returns (id, embedding) for recent candidates, for dedup.
func GetExistingIdeaEmbeddings(ctx context.Context, db *pgxpool.Pool, limit int) ([]struct {
	ID        int64
	Embedding []float32
}, error) {
	rows, err := db.Query(ctx, `
SELECT id, embedding FROM saas_idea_candidate
WHERE embedding IS NOT NULL
ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		ID        int64
		Embedding []float32
	}
	for rows.Next() {
		var item struct {
			ID        int64
			Embedding []float32
		}
		var embRaw any
		if err := rows.Scan(&item.ID, &embRaw); err != nil {
			return nil, err
		}
		item.Embedding = parseVecAny(embRaw)
		out = append(out, item)
	}
	return out, rows.Err()
}

// UpdateDocumentStateAnalysed transitions a document to 'analyzed'.
func UpdateDocumentStateAnalysed(ctx context.Context, db *pgxpool.Pool, docID int64, state string) error {
	_, err := db.Exec(ctx,
		`UPDATE document SET state=$1::doc_state, updated_at=now() WHERE id=$2`,
		state, docID)
	return err
}

// parseVecAny converts a pgvector []float32 from the raw scan interface.
func parseVecAny(v any) []float32 {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []float32:
		return t
	case string:
		// pgvector returns as string "[0.1,0.2,...]" in text protocol
		var f []float32
		_ = json.Unmarshal([]byte(t), &f)
		return f
	}
	return nil
}
