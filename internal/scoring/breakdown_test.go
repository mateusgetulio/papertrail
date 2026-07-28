package scoring

import (
	"math"
	"testing"
)

func TestBreakdownFullCriteria(t *testing.T) {
	criteria := []int{10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 10, 9, 8, 7}
	rows := Breakdown(criteria)
	if len(rows) != 14 {
		t.Fatalf("len(rows) = %d, want 14", len(rows))
	}
	for i, row := range rows {
		if row.Key != criterionKeys[i] {
			t.Errorf("row %d key = %q, want %q", i, row.Key, criterionKeys[i])
		}
		if row.Label != criterionLabels[i] {
			t.Errorf("row %d label = %q, want %q", i, row.Label, criterionLabels[i])
		}
		if row.Raw != criteria[i] {
			t.Errorf("row %d raw = %d, want %d", i, row.Raw, criteria[i])
		}
		if row.Weight != weights[i] {
			t.Errorf("row %d weight = %v, want %v", i, row.Weight, weights[i])
		}
		if row.Inverted != inverted[i] {
			t.Errorf("row %d inverted = %v, want %v", i, row.Inverted, inverted[i])
		}
		wantAdj := float64(criteria[i])
		if inverted[i] {
			wantAdj = 11.0 - wantAdj
		}
		if row.Adj != wantAdj {
			t.Errorf("row %d adj = %v, want %v", i, row.Adj, wantAdj)
		}
		if math.Abs(row.Contribution-row.Weight*row.Adj) > 1e-12 {
			t.Errorf("row %d contribution = %v, want weight*adj = %v", i, row.Contribution, row.Weight*row.Adj)
		}
	}
}

func TestBreakdownInvertedFlags(t *testing.T) {
	rows := Breakdown(make([]int, 14))
	invertedKeys := map[string]bool{}
	for _, row := range rows {
		if row.Inverted {
			invertedKeys[row.Key] = true
		}
	}
	want := []string{"competition", "impl_complexity", "time_to_mvp"}
	if len(invertedKeys) != len(want) {
		t.Fatalf("inverted keys = %v, want exactly %v", invertedKeys, want)
	}
	for _, k := range want {
		if !invertedKeys[k] {
			t.Errorf("%s should be inverted", k)
		}
	}
}

func TestBreakdownClampsAdjButKeepsRaw(t *testing.T) {
	rows := Breakdown([]int{15})
	if rows[0].Raw != 15 {
		t.Errorf("raw = %d, want the stored 15 preserved", rows[0].Raw)
	}
	if rows[0].Adj != 10 {
		t.Errorf("adj = %v, want clamped to 10", rows[0].Adj)
	}
}

func TestBreakdownLengthTolerance(t *testing.T) {
	cases := []struct {
		name    string
		nInput  int
		wantLen int
	}{
		{"empty input", 0, 0},
		{"short input truncates rows", 5, 5},
		{"exact input", 14, 14},
		{"extra entries ignored", 20, 14},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := len(Breakdown(make([]int, c.nInput))); got != c.wantLen {
				t.Errorf("len(Breakdown(%d ints)) = %d, want %d", c.nInput, got, c.wantLen)
			}
		})
	}
}

func TestLabels(t *testing.T) {
	labels := Labels()
	if len(labels) != 14 {
		t.Fatalf("len(Labels()) = %d, want 14", len(labels))
	}
	for i, l := range labels {
		if l == "" {
			t.Errorf("label %d is empty", i)
		}
	}
}
