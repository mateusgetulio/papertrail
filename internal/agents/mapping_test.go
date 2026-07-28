package agents

import (
	"reflect"
	"testing"

	"github.com/mateusgetulio/papertrail/internal/store"
)

func TestCitations(t *testing.T) {
	cases := []struct {
		name     string
		evidence []store.Evidence
		want     []string
	}{
		{
			name:     "no evidence yields no citations",
			evidence: nil,
			want:     nil,
		},
		{
			name: "blank and whitespace-only URLs skipped",
			evidence: []store.Evidence{
				{SourceURL: ""},
				{SourceURL: "   "},
				{SourceURL: "https://example.org/a"},
			},
			want: []string{"https://example.org/a"},
		},
		{
			name: "surrounding whitespace trimmed",
			evidence: []store.Evidence{
				{SourceURL: "  https://example.org/a  "},
			},
			want: []string{"https://example.org/a"},
		},
		{
			name: "first occurrence order preserved",
			evidence: []store.Evidence{
				{SourceURL: "https://example.org/b"},
				{SourceURL: "https://example.org/a"},
				{SourceURL: "https://example.org/b"},
			},
			want: []string{"https://example.org/b", "https://example.org/a"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := citations(&store.IdeaDetail{Evidence: c.evidence})
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("citations() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestCandidateFromDetailFieldMapping(t *testing.T) {
	d := &store.IdeaDetail{
		IdeaRow: store.IdeaRow{
			ID: 42, IdeaName: "Gamma", Pitch: "pitch", Label: "vertical_saas",
			Industries: []string{"Finance", "Insurance"}, PainPoint: "pain",
			OverallScore: 66,
		},
		DisruptionDriver: "demand_shift",
		TargetCustomer:   "mid-market insurers",
		Countries:        []string{"BR", "US"},
		Evidence:         []store.Evidence{{SourceURL: "https://example.org/x"}},
	}
	c := candidateFromDetail(d)

	if c.CandidateID != "42" {
		t.Errorf("CandidateID = %q, want the int64 ID formatted as %q", c.CandidateID, "42")
	}
	if c.IdeaName != "Gamma" || c.OneSentencePitch != "pitch" || c.Category != "vertical_saas" {
		t.Errorf("identity fields mismapped: %+v", c)
	}
	if !reflect.DeepEqual(c.Industries, d.Industries) {
		t.Errorf("Industries = %v, want %v", c.Industries, d.Industries)
	}
	if !reflect.DeepEqual(c.CountriesOrRegions, d.Countries) {
		t.Errorf("CountriesOrRegions = %v, want %v", c.CountriesOrRegions, d.Countries)
	}
	if c.TargetCustomer != "mid-market insurers" || c.DisruptionDriver != "demand_shift" {
		t.Errorf("narrative fields mismapped: %+v", c)
	}
	if !reflect.DeepEqual(c.SourceDocuments, c.Citations) {
		t.Errorf("SourceDocuments %v and Citations %v must stay in sync", c.SourceDocuments, c.Citations)
	}
}
