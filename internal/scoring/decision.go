package scoring

// Decision-support helpers. All deterministic — derived from stored sub-scores,
// no model calls. Used by both the report generator and the dashboard to help a
// human triage a large candidate pool.

// Recommendation returns a triage tier from an idea's overall score, build
// effort (mvp complexity 1..10), and evidence strength (distinct sources).
func Recommendation(score, mvpComplexity, evidenceCount int) string {
	switch {
	case evidenceCount == 0 || score < 45:
		return "Park"
	case score >= 50 && mvpComplexity <= 5:
		return "Build now"
	case score >= 50:
		return "Validate" // strong, but heavy build — de-risk first
	default:
		return "Explore" // 45–49
	}
}

// QuickWinScore is reward ÷ effort (higher = more attractive quick win):
// reward = market urgency + high-ticket potential (2..20); effort = mvp
// complexity (1..10, clamped ≥1).
func QuickWinScore(marketUrgency, highTicket, mvpComplexity int) float64 {
	effort := mvpComplexity
	if effort < 1 {
		effort = 1
	}
	return float64(marketUrgency+highTicket) / float64(effort)
}

// Quadrant classifies an idea on reward (overall score) vs effort (mvp
// complexity): Quick win, Big bet, Filler, or Heavy/uncertain.
func Quadrant(score, mvpComplexity int) string {
	highReward := score >= 50
	lowEffort := mvpComplexity <= 5
	switch {
	case highReward && lowEffort:
		return "Quick win"
	case highReward && !lowEffort:
		return "Big bet"
	case !highReward && lowEffort:
		return "Filler"
	default:
		return "Heavy/uncertain"
	}
}

// Confidence labels evidence strength by the number of distinct source documents
// backing an idea.
func Confidence(distinctSources int) string {
	switch {
	case distinctSources >= 3:
		return "High"
	case distinctSources == 2:
		return "Medium"
	case distinctSources == 1:
		return "Low"
	default:
		return "None"
	}
}
