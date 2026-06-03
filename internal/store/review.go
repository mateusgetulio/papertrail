package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReviewStates is the ordered set of valid review_state enum values.
var ReviewStates = []string{"pending", "approved", "rejected", "needs_more_evidence", "merged"}

// IdeaFilter narrows the review queue. Zero values mean "no constraint".
type IdeaFilter struct {
	Label    string // idea_label value, or "" for any
	State    string // review_state value, or "" for any
	MinScore int    // 0 = no minimum
	MaxScore int    // 0 = no maximum
}

// IdeaRow is a row in the review queue / CSV export.
type IdeaRow struct {
	ID           int64
	IdeaName     string
	Pitch        string
	Label        string
	SalesMotion  string
	Industries   []string
	PainPoint    string
	OverallScore int
	ReviewState  string
	CreatedAt    time.Time
}

// Evidence is one citation backing an idea.
type Evidence struct {
	Excerpt      string
	CitationText string
	SourceURL    string
}

// IdeaDetail is the full record for the idea detail page.
type IdeaDetail struct {
	IdeaRow
	DisruptionDriver    string
	TargetCustomer      string
	BuyerPersona        string
	Countries           []string
	WhyWork             string
	WhyFail             string
	PossibleMVP         string
	First10             string
	ValidationQuestions []string

	Criteria       []int
	GenericRisk    int
	ConsultingRisk int
	TrustWeight    float64

	Evidence   []Evidence
	Reviewer   string
	Notes      string
	MergedInto *int64
}

// ListIdeas returns review-queue rows ordered by overall_score descending.
func ListIdeas(ctx context.Context, db *pgxpool.Pool, f IdeaFilter) ([]IdeaRow, error) {
	const q = `
SELECT i.id, i.idea_name, COALESCE(i.one_sentence_pitch,''),
       COALESCE(i.label::text,''), COALESCE(i.sales_motion::text,''),
       i.industries, COALESCE(i.pain_point,''),
       COALESCE(rs.overall_score,0), COALESCE(rv.state::text,'pending'), i.created_at
FROM saas_idea_candidate i
LEFT JOIN ranking_score rs ON rs.idea_id = i.id
LEFT JOIN review_status rv ON rv.idea_id = i.id
WHERE ($1 = '' OR i.label::text = $1)
  AND ($2 = '' OR COALESCE(rv.state::text,'pending') = $2)
  AND ($3 = 0 OR COALESCE(rs.overall_score,0) >= $3)
  AND ($4 = 0 OR COALESCE(rs.overall_score,0) <= $4)
ORDER BY rs.overall_score DESC NULLS LAST, i.created_at DESC`
	rows, err := db.Query(ctx, q, f.Label, f.State, f.MinScore, f.MaxScore)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IdeaRow
	for rows.Next() {
		var r IdeaRow
		if err := rows.Scan(&r.ID, &r.IdeaName, &r.Pitch, &r.Label, &r.SalesMotion,
			&r.Industries, &r.PainPoint, &r.OverallScore, &r.ReviewState, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetIdeaDetail loads the full idea, score components, review status, and evidence.
// Returns (nil, nil) when no idea has the given id.
func GetIdeaDetail(ctx context.Context, db *pgxpool.Pool, id int64) (*IdeaDetail, error) {
	const q = `
SELECT i.id, i.idea_name, COALESCE(i.one_sentence_pitch,''),
       COALESCE(i.label::text,''), COALESCE(i.sales_motion::text,''),
       i.industries, COALESCE(i.pain_point,''),
       COALESCE(i.disruption_driver,''), COALESCE(i.target_customer,''), COALESCE(i.buyer_persona,''),
       i.countries_or_regions, COALESCE(i.why_it_might_work,''), COALESCE(i.why_it_might_fail,''),
       COALESCE(i.possible_mvp,''), COALESCE(i.first_10_customers,''), i.validation_questions,
       i.created_at,
       COALESCE(rs.overall_score,0), rs.components,
       COALESCE(rv.state::text,'pending'), COALESCE(rv.reviewer,''), COALESCE(rv.notes,''), rv.merged_into
FROM saas_idea_candidate i
LEFT JOIN ranking_score rs ON rs.idea_id = i.id
LEFT JOIN review_status rv ON rv.idea_id = i.id
WHERE i.id = $1`

	var d IdeaDetail
	var compRaw []byte
	err := db.QueryRow(ctx, q, id).Scan(
		&d.ID, &d.IdeaName, &d.Pitch, &d.Label, &d.SalesMotion,
		&d.Industries, &d.PainPoint,
		&d.DisruptionDriver, &d.TargetCustomer, &d.BuyerPersona,
		&d.Countries, &d.WhyWork, &d.WhyFail,
		&d.PossibleMVP, &d.First10, &d.ValidationQuestions,
		&d.CreatedAt,
		&d.OverallScore, &compRaw,
		&d.ReviewState, &d.Reviewer, &d.Notes, &d.MergedInto,
	)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}

	if len(compRaw) > 0 {
		var c struct {
			Criteria       []int   `json:"criteria"`
			GenericRisk    int     `json:"generic_risk"`
			ConsultingRisk int     `json:"consulting_risk"`
			TrustWeight    float64 `json:"trust_weight"`
		}
		if err := json.Unmarshal(compRaw, &c); err == nil {
			d.Criteria = c.Criteria
			d.GenericRisk = c.GenericRisk
			d.ConsultingRisk = c.ConsultingRisk
			d.TrustWeight = c.TrustWeight
		}
	}

	ev, err := db.Query(ctx, `
SELECT COALESCE(excerpt,''), COALESCE(citation_text,''), COALESCE(source_url,'')
FROM evidence WHERE idea_id = $1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer ev.Close()
	for ev.Next() {
		var e Evidence
		if err := ev.Scan(&e.Excerpt, &e.CitationText, &e.SourceURL); err != nil {
			return nil, err
		}
		d.Evidence = append(d.Evidence, e)
	}
	if err := ev.Err(); err != nil {
		return nil, err
	}
	return &d, nil
}

// UpdateReviewStatus upserts the review decision for an idea. mergedInto may be
// nil unless state == "merged".
func UpdateReviewStatus(ctx context.Context, db *pgxpool.Pool,
	ideaID int64, state, reviewer, notes string, mergedInto *int64) error {
	_, err := db.Exec(ctx, `
INSERT INTO review_status (idea_id, state, reviewer, notes, merged_into, updated_at)
VALUES ($1, $2::review_state, $3, $4, $5, now())
ON CONFLICT (idea_id) DO UPDATE
  SET state = $2::review_state, reviewer = $3, notes = $4, merged_into = $5, updated_at = now()`,
		ideaID, state, reviewer, notes, mergedInto)
	return err
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
