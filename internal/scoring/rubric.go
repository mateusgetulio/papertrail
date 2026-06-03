package scoring

import (
	"math"

	"github.com/mateusgetulio/papertrail/internal/llm"
)

// weights for the 14 criteria (docs/06 §2). Must sum to 1.0.
var weights = [14]float64{
	0.12, // 0  problem_frequency
	0.10, // 1  urgency
	0.10, // 2  willingness_to_pay
	0.06, // 3  budget_owner_clarity
	0.06, // 4  regulatory_pressure
	0.08, // 5  competition          ↓ inverted
	0.10, // 6  market_size
	0.08, // 7  impl_complexity      ↓ inverted
	0.06, // 8  data_availability
	0.07, // 9  defensibility
	0.04, // 10 sales_motion_clarity
	0.05, // 11 high_ticket_potential
	0.05, // 12 time_to_mvp          ↓ inverted
	0.03, // 13 founder_fit
}

// inverted[i] = true: higher raw score is a risk (we flip it before weighting).
// indices:              0      1      2      3      4      5     6      7     8      9     10     11    12    13
var inverted = [14]bool{false, false, false, false, false, true, false, true, false, false, false, false, true, false}

// Compute deterministically calculates overall_score (1–100) from the LLM sub-scores.
// This is the single source of truth for ranking — the model's own number is never used.
func Compute(s llm.SubScores) int {
	base := 0.0
	for i, raw := range s.Criteria {
		adj := float64(clampScore(raw))
		if inverted[i] {
			adj = 11.0 - adj
		}
		base += weights[i] * adj
	}

	// Trust & frequency adjustments (docs/06 §3)
	trustFactor := 0.85 + 0.15*clamp01(s.AvgSourceTrustWeight)
	freqBonus := math.Min(0.10, 0.02*float64(max0(s.DistinctSourceDocs-1)))
	adjusted := base * trustFactor * (1.0 + freqBonus)

	// Penalty multipliers (docs/06 §4)
	genericPenalty := 1.0 - 0.05*float64(clampScore(s.GenericRisk)-1)
	consultingPenalty := 1.0 - 0.06*float64(clampScore(s.ConsultingRisk)-1)
	final := adjusted * genericPenalty * consultingPenalty

	// Map 1–10 → 1–100
	score := int(math.Round(clamp(final, 1, 10) * 10))
	return score
}

func clampScore(v int) int { return clampInt(v, 1, 10) }
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
func clamp01(v float64) float64 { return clamp(v, 0, 1) }
func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}
