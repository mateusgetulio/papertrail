package scoring

import "testing"

func TestRecommendation(t *testing.T) {
	cases := []struct {
		score, mvp, evid int
		want             string
	}{
		{55, 4, 1, "Build now"}, // strong + easy + evidence
		{55, 8, 1, "Validate"},  // strong but heavy build
		{47, 4, 1, "Explore"},   // middling
		{40, 4, 1, "Park"},      // too weak
		{55, 4, 0, "Park"},      // no evidence → park regardless
	}
	for _, c := range cases {
		if got := Recommendation(c.score, c.mvp, c.evid); got != c.want {
			t.Errorf("Recommendation(%d,%d,%d)=%q want %q", c.score, c.mvp, c.evid, got, c.want)
		}
	}
}

func TestQuadrant(t *testing.T) {
	if Quadrant(60, 3) != "Quick win" {
		t.Error("high reward + low effort should be Quick win")
	}
	if Quadrant(60, 8) != "Big bet" {
		t.Error("high reward + high effort should be Big bet")
	}
	if Quadrant(30, 8) != "Heavy/uncertain" {
		t.Error("low reward + high effort should be Heavy/uncertain")
	}
}

func TestQuickWinAndConfidence(t *testing.T) {
	// reward 8+8=16, effort 4 → 4.0
	if got := QuickWinScore(8, 8, 4); got != 4.0 {
		t.Errorf("QuickWinScore=%v want 4.0", got)
	}
	if QuickWinScore(5, 5, 0) == 0 { // effort clamps to 1, no divide-by-zero
		t.Error("effort should clamp to >=1")
	}
	if Confidence(1) != "Low" || Confidence(3) != "High" || Confidence(0) != "None" {
		t.Error("confidence bands wrong")
	}
}
