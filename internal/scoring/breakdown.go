package scoring

// CriterionRow is one labeled, weighted criterion for dashboard explainability.
// It is the read-side counterpart to Compute/Components: it maps the 14 raw
// scores stored in ranking_score.components.criteria back to human labels using
// the same weights and inversion rules defined in rubric.go (single source of
// truth — do not duplicate the weights elsewhere).
type CriterionRow struct {
	Key          string
	Label        string
	Raw          int     // 1..10 as scored by the LLM
	Adj          float64 // inverted criteria flipped (11-raw)
	Weight       float64 // 0..1
	Contribution float64 // Weight * Adj
	Inverted     bool
}

var criterionKeys = [14]string{
	"problem_frequency", "urgency", "willingness_to_pay", "budget_owner_clarity",
	"regulatory_pressure", "competition", "market_size", "impl_complexity",
	"data_availability", "defensibility", "sales_motion_clarity",
	"high_ticket_potential", "time_to_mvp", "founder_fit",
}

var criterionLabels = [14]string{
	"Problem frequency", "Problem urgency", "Willingness to pay", "Budget owner clarity",
	"Regulatory pressure", "Existing competition", "Market size", "Implementation complexity",
	"Data availability", "Defensibility / moat", "Sales-motion clarity",
	"High-ticket potential", "Time-to-MVP", "Founder / build fit",
}

// Labels returns the 14 criterion labels in rubric order.
func Labels() []string { return criterionLabels[:] }

// Breakdown maps the stored raw criteria scores to labeled, weighted rows in
// rubric order. Extra or missing entries are tolerated (it ranges over the
// shorter of len(criteria) and 14).
func Breakdown(criteria []int) []CriterionRow {
	rows := make([]CriterionRow, 0, len(criterionKeys))
	for i := 0; i < len(criterionKeys) && i < len(criteria); i++ {
		adj := float64(clampScore(criteria[i]))
		if inverted[i] {
			adj = 11.0 - adj
		}
		rows = append(rows, CriterionRow{
			Key:          criterionKeys[i],
			Label:        criterionLabels[i],
			Raw:          criteria[i],
			Adj:          adj,
			Weight:       weights[i],
			Contribution: weights[i] * adj,
			Inverted:     inverted[i],
		})
	}
	return rows
}
