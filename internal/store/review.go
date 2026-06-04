package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReviewStates is the ordered set of valid review_state enum values.
var ReviewStates = []string{"pending", "approved", "rejected", "needs_more_evidence", "merged"}

// SortOptions maps the queue sort keys to human labels (and bounds the
// whitelist used to build ORDER BY — never interpolate user input directly).
var SortOptions = []struct{ Key, Label string }{
	{"score", "Highest score"},
	{"quickwin", "Quick win (reward/effort)"},
	{"recent", "Newest"},
	{"name", "Name (A–Z)"},
}

// IdeaFilter narrows the review queue. Zero values mean "no constraint".
type IdeaFilter struct {
	Label    string // idea_label value, or "" for any
	State    string // review_state value, or "" for any
	MinScore int    // 0 = no minimum
	MaxScore int    // 0 = no maximum
	Query    string // free-text search over name/pitch/pain point/industries
	Sort     string // one of SortOptions keys; defaults to "score"
	Limit    int    // <= 0 means no LIMIT (used by CSV export)
	Offset   int
}

// IdeaRow is a row in the review queue / CSV export.
type IdeaRow struct {
	ID            int64
	IdeaName      string
	Pitch         string
	Label         string
	SalesMotion   string
	Industries    []string
	PainPoint     string
	OverallScore  int
	ReviewState   string
	EvidenceCount int // distinct source documents backing the idea
	CreatedAt     time.Time

	// Per-axis LLM sub-scores stored as columns (1..10). Used for the
	// recommendation tier, quick-win ranking, and quadrant.
	HighTicketPotential  int
	MassMarketPotential  int
	TechnicalFeasibility int
	MarketUrgency        int
	CompetitionRisk      int
	DataAvailability     int
	MVPComplexity        int
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

	MergedIntoName string    // name of the idea this one was merged into (if any)
	MergedFrom     []IdeaRef // ideas that were merged into this one
}

// IdeaRef is a minimal (id, name) reference used for merge targets and links.
type IdeaRef struct {
	ID       int64
	IdeaName string
}

// IdeaEdit holds the editable narrative fields of an idea.
type IdeaEdit struct {
	IdeaName       string
	Pitch          string
	PainPoint      string
	TargetCustomer string
	BuyerPersona   string
	WhyWork        string
	WhyFail        string
	PossibleMVP    string
	First10        string
	Industries     []string
	Countries      []string
}

// buildWhere assembles the shared WHERE clause and its positional args for the
// queue listing and count queries. All user input is bound as parameters.
func buildWhere(f IdeaFilter) (string, []any) {
	var conds []string
	var args []any
	add := func(tmpl string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(tmpl, len(args)))
	}
	if f.Label != "" {
		add("i.label::text = $%d", f.Label)
	}
	if f.State != "" {
		add("COALESCE(rv.state::text,'pending') = $%d", f.State)
	}
	if f.MinScore > 0 {
		add("COALESCE(rs.overall_score,0) >= $%d", f.MinScore)
	}
	if f.MaxScore > 0 {
		add("COALESCE(rs.overall_score,0) <= $%d", f.MaxScore)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, "%"+q+"%")
		n := len(args)
		conds = append(conds, fmt.Sprintf(
			"(i.idea_name ILIKE $%d OR i.one_sentence_pitch ILIKE $%d OR COALESCE(i.pain_point,'') ILIKE $%d OR array_to_string(i.industries,' ') ILIKE $%d)",
			n, n, n, n))
	}
	if len(conds) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

// orderClause maps a sort key to a safe ORDER BY (whitelist only).
func orderClause(sort string) string {
	switch sort {
	case "recent":
		return "ORDER BY i.created_at DESC"
	case "name":
		return "ORDER BY i.idea_name ASC"
	case "quickwin":
		return "ORDER BY (COALESCE(i.market_urgency,0)+COALESCE(i.high_ticket_potential,0))::float / GREATEST(COALESCE(i.mvp_complexity,1),1) DESC, rs.overall_score DESC NULLS LAST"
	default: // "score"
		return "ORDER BY rs.overall_score DESC NULLS LAST, i.created_at DESC"
	}
}

// CountIdeas returns the total number of ideas matching the filter (ignoring
// Limit/Offset), for pagination.
func CountIdeas(ctx context.Context, db *pgxpool.Pool, f IdeaFilter) (int, error) {
	where, args := buildWhere(f)
	q := `
SELECT count(*)
FROM saas_idea_candidate i
LEFT JOIN ranking_score rs ON rs.idea_id = i.id
LEFT JOIN review_status rv ON rv.idea_id = i.id
` + where
	var n int
	if err := db.QueryRow(ctx, q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// ListIdeas returns review-queue rows for the given filter, sort, and page.
func ListIdeas(ctx context.Context, db *pgxpool.Pool, f IdeaFilter) ([]IdeaRow, error) {
	where, args := buildWhere(f)
	q := `
SELECT i.id, i.idea_name, COALESCE(i.one_sentence_pitch,''),
       COALESCE(i.label::text,''), COALESCE(i.sales_motion::text,''),
       i.industries, COALESCE(i.pain_point,''),
       COALESCE(rs.overall_score,0), COALESCE(rv.state::text,'pending'),
       (SELECT count(DISTINCT source_url) FROM evidence e WHERE e.idea_id = i.id), i.created_at,
       COALESCE(i.high_ticket_potential,0), COALESCE(i.mass_market_potential,0),
       COALESCE(i.technical_feasibility,0), COALESCE(i.market_urgency,0),
       COALESCE(i.competition_risk,0), COALESCE(i.data_availability,0), COALESCE(i.mvp_complexity,0)
FROM saas_idea_candidate i
LEFT JOIN ranking_score rs ON rs.idea_id = i.id
LEFT JOIN review_status rv ON rv.idea_id = i.id
` + where + "\n" + orderClause(f.Sort)

	if f.Limit > 0 {
		args = append(args, f.Limit)
		q += fmt.Sprintf("\nLIMIT $%d", len(args))
		args = append(args, f.Offset)
		q += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IdeaRow
	for rows.Next() {
		var r IdeaRow
		if err := rows.Scan(&r.ID, &r.IdeaName, &r.Pitch, &r.Label, &r.SalesMotion,
			&r.Industries, &r.PainPoint, &r.OverallScore, &r.ReviewState,
			&r.EvidenceCount, &r.CreatedAt,
			&r.HighTicketPotential, &r.MassMarketPotential, &r.TechnicalFeasibility,
			&r.MarketUrgency, &r.CompetitionRisk, &r.DataAvailability, &r.MVPComplexity); err != nil {
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
       COALESCE(i.high_ticket_potential,0), COALESCE(i.mass_market_potential,0),
       COALESCE(i.technical_feasibility,0), COALESCE(i.market_urgency,0),
       COALESCE(i.competition_risk,0), COALESCE(i.data_availability,0), COALESCE(i.mvp_complexity,0),
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
		&d.HighTicketPotential, &d.MassMarketPotential,
		&d.TechnicalFeasibility, &d.MarketUrgency,
		&d.CompetitionRisk, &d.DataAvailability, &d.MVPComplexity,
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
	// Distinct source documents → confidence + recommendation tier.
	srcSeen := map[string]bool{}
	for _, e := range d.Evidence {
		if e.SourceURL != "" {
			srcSeen[e.SourceURL] = true
		}
	}
	d.EvidenceCount = len(srcSeen)

	// Merge relationships: the target this idea was merged into, and any ideas
	// that were merged into this one.
	if d.MergedInto != nil {
		_ = db.QueryRow(ctx, `SELECT idea_name FROM saas_idea_candidate WHERE id = $1`, *d.MergedInto).Scan(&d.MergedIntoName)
	}
	mf, err := db.Query(ctx, `
SELECT i.id, i.idea_name
FROM review_status rv
JOIN saas_idea_candidate i ON i.id = rv.idea_id
WHERE rv.merged_into = $1
ORDER BY i.idea_name`, id)
	if err != nil {
		return nil, err
	}
	defer mf.Close()
	for mf.Next() {
		var ref IdeaRef
		if err := mf.Scan(&ref.ID, &ref.IdeaName); err != nil {
			return nil, err
		}
		d.MergedFrom = append(d.MergedFrom, ref)
	}
	if err := mf.Err(); err != nil {
		return nil, err
	}
	return &d, nil
}

// ListIdeaRefs returns minimal (id, name) refs for all ideas except exclude
// (pass 0 to include all), ordered by name — used for the merge-target picker.
func ListIdeaRefs(ctx context.Context, db *pgxpool.Pool, exclude int64) ([]IdeaRef, error) {
	rows, err := db.Query(ctx, `
SELECT id, idea_name FROM saas_idea_candidate
WHERE id <> $1 ORDER BY idea_name`, exclude)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IdeaRef
	for rows.Next() {
		var ref IdeaRef
		if err := rows.Scan(&ref.ID, &ref.IdeaName); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// UpdateIdeaFields persists edits to an idea's narrative fields.
func UpdateIdeaFields(ctx context.Context, db *pgxpool.Pool, id int64, e IdeaEdit) error {
	_, err := db.Exec(ctx, `
UPDATE saas_idea_candidate SET
  idea_name = $2, one_sentence_pitch = $3, pain_point = $4,
  target_customer = $5, buyer_persona = $6,
  why_it_might_work = $7, why_it_might_fail = $8,
  possible_mvp = $9, first_10_customers = $10,
  industries = $11, countries_or_regions = $12
WHERE id = $1`,
		id, e.IdeaName, e.Pitch, e.PainPoint, e.TargetCustomer, e.BuyerPersona,
		e.WhyWork, e.WhyFail, e.PossibleMVP, e.First10, e.Industries, e.Countries)
	return err
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
