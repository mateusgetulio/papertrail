package scoring

import (
	"math"
	"testing"

	"github.com/mateusgetulio/papertrail/internal/llm"
)

func uniform(v int) [14]int {
	var a [14]int
	for i := range a {
		a[i] = v
	}
	return a
}

func baseScores() llm.SubScores {
	return llm.SubScores{
		Criteria:             uniform(10),
		GenericRisk:          1,
		ConsultingRisk:       1,
		AvgSourceTrustWeight: 1,
		DistinctSourceDocs:   1,
	}
}

func TestWeightsSumToOne(t *testing.T) {
	sum := 0.0
	for _, w := range weights {
		sum += w
	}
	if math.Abs(sum-1.0) > 1e-9 {
		t.Errorf("weights sum = %v, want 1.0", sum)
	}
}

func TestCompute(t *testing.T) {
	cases := []struct {
		name   string
		modify func(*llm.SubScores)
		want   int
	}{
		{
			name:   "all tens with inverted criteria dragging down",
			modify: func(s *llm.SubScores) {},
			want:   81,
		},
		{
			name:   "all ones with inverted criteria lifting up",
			modify: func(s *llm.SubScores) { s.Criteria = uniform(1) },
			want:   29,
		},
		{
			name:   "zero trust applies 0.85 factor",
			modify: func(s *llm.SubScores) { s.AvgSourceTrustWeight = 0 },
			want:   69,
		},
		{
			name:   "two source docs add 2 percent bonus",
			modify: func(s *llm.SubScores) { s.DistinctSourceDocs = 2 },
			want:   83,
		},
		{
			name:   "six source docs reach the 10 percent cap",
			modify: func(s *llm.SubScores) { s.DistinctSourceDocs = 6 },
			want:   89,
		},
		{
			name:   "hundred source docs still capped at 10 percent",
			modify: func(s *llm.SubScores) { s.DistinctSourceDocs = 100 },
			want:   89,
		},
		{
			name:   "max generic risk multiplies by 0.55",
			modify: func(s *llm.SubScores) { s.GenericRisk = 10 },
			want:   45,
		},
		{
			name:   "max consulting risk multiplies by 0.46",
			modify: func(s *llm.SubScores) { s.ConsultingRisk = 10 },
			want:   37,
		},
		{
			name:   "criteria above 10 clamp to 10",
			modify: func(s *llm.SubScores) { s.Criteria = uniform(15) },
			want:   81,
		},
		{
			name:   "criteria below 1 clamp to 1",
			modify: func(s *llm.SubScores) { s.Criteria = uniform(-5) },
			want:   29,
		},
		{
			name:   "risk of 0 clamps to 1 leaving no penalty",
			modify: func(s *llm.SubScores) { s.GenericRisk = 0; s.ConsultingRisk = 0 },
			want:   81,
		},
		{
			name:   "trust above 1 clamps to 1",
			modify: func(s *llm.SubScores) { s.AvgSourceTrustWeight = 5 },
			want:   81,
		},
		{
			name:   "negative source docs treated as zero",
			modify: func(s *llm.SubScores) { s.DistinctSourceDocs = -3 },
			want:   81,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := baseScores()
			c.modify(&s)
			if got := Compute(s); got != c.want {
				t.Errorf("Compute() = %d, want %d", got, c.want)
			}
		})
	}
}

func TestComputeCeilingIs100(t *testing.T) {
	s := baseScores()
	s.Criteria[5], s.Criteria[7], s.Criteria[12] = 1, 1, 1
	s.DistinctSourceDocs = 6
	if got := Compute(s); got != 100 {
		t.Errorf("best possible idea should score 100, got %d", got)
	}
}

func TestComputeFloorIs10(t *testing.T) {
	s := llm.SubScores{
		Criteria:             uniform(1),
		GenericRisk:          10,
		ConsultingRisk:       10,
		AvgSourceTrustWeight: 0,
		DistinctSourceDocs:   0,
	}
	s.Criteria[5], s.Criteria[7], s.Criteria[12] = 10, 10, 10
	if got := Compute(s); got != 10 {
		t.Errorf("worst possible idea should floor at 10, got %d", got)
	}
}

func TestComputeInvertedCriterionLowersScore(t *testing.T) {
	low := baseScores()
	low.Criteria[5] = 1
	high := baseScores()
	high.Criteria[5] = 10
	if Compute(low) <= Compute(high) {
		t.Error("raising an inverted criterion (competition) must lower the score")
	}
}
