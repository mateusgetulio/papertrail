package search

import (
	"reflect"
	"strings"
	"testing"
)

func TestExpand(t *testing.T) {
	cases := []struct {
		name     string
		template QueryTemplate
		want     []string
	}{
		{
			name:     "no dimensions returns pattern verbatim",
			template: QueryTemplate{Pattern: `site:x.org "fixed query"`},
			want:     []string{`site:x.org "fixed query"`},
		},
		{
			name: "single dimension",
			template: QueryTemplate{
				Pattern:    `report "%s"`,
				Dimensions: [][]string{{"health", "energy"}},
			},
			want: []string{`report "health"`, `report "energy"`},
		},
		{
			name: "two dimensions produce ordered cartesian product",
			template: QueryTemplate{
				Pattern:    `"%s" "%s"`,
				Dimensions: [][]string{{"a", "b"}, {"1", "2"}},
			},
			want: []string{`"a" "1"`, `"a" "2"`, `"b" "1"`, `"b" "2"`},
		},
		{
			name: "empty dimension yields no queries",
			template: QueryTemplate{
				Pattern:    `"%s"`,
				Dimensions: [][]string{{}},
			},
			want: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.template.Expand()
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Expand() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDefaultTemplatesExpandCleanly(t *testing.T) {
	templates := DefaultTemplates()
	if len(templates) == 0 {
		t.Fatal("expected seed templates")
	}
	for ti, tpl := range templates {
		wantCount := 1
		for _, dim := range tpl.Dimensions {
			wantCount *= len(dim)
		}
		queries := tpl.Expand()
		if len(queries) != wantCount {
			t.Errorf("template %d expanded to %d queries, want %d", ti, len(queries), wantCount)
		}
		placeholders := strings.Count(tpl.Pattern, "%s")
		if placeholders != len(tpl.Dimensions) {
			t.Errorf("template %d has %d placeholders but %d dimensions", ti, placeholders, len(tpl.Dimensions))
		}
		for _, q := range queries {
			if strings.Contains(q, "%!") || strings.Contains(q, "MISSING") {
				t.Errorf("template %d produced malformed query %q", ti, q)
			}
			if strings.TrimSpace(q) == "" {
				t.Errorf("template %d produced empty query", ti)
			}
		}
	}
}

func TestDefaultTemplatesQueriesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, tpl := range DefaultTemplates() {
		for _, q := range tpl.Expand() {
			if seen[q] {
				t.Errorf("duplicate query across templates: %q", q)
			}
			seen[q] = true
		}
	}
}
