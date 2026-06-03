package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Count is a named tally used in the insights view.
type Count struct {
	Name string
	N    int
}

// ScoreBucket is one bar of the score-distribution histogram.
type ScoreBucket struct {
	Label string
	Count int
}

// Insights is the aggregate decision-support view over all candidate ideas.
type Insights struct {
	Total         int
	AvgScore      float64
	ScoreBuckets  []ScoreBucket
	TopIndustries []Count
	Labels        []Count
}

// GetInsights computes corpus-wide aggregates for the insights landing page.
func GetInsights(ctx context.Context, db *pgxpool.Pool) (*Insights, error) {
	ins := &Insights{}

	if err := db.QueryRow(ctx, `
SELECT count(*), COALESCE(round(avg(rs.overall_score)::numeric, 1), 0)
FROM saas_idea_candidate i
LEFT JOIN ranking_score rs ON rs.idea_id = i.id`).Scan(&ins.Total, &ins.AvgScore); err != nil {
		return nil, err
	}

	// Score distribution in five 20-point bands.
	var b [5]int
	if err := db.QueryRow(ctx, `
WITH s AS (
  SELECT COALESCE(rs.overall_score, 0) AS score
  FROM saas_idea_candidate i
  LEFT JOIN ranking_score rs ON rs.idea_id = i.id
)
SELECT
  count(*) FILTER (WHERE score BETWEEN  1 AND 20),
  count(*) FILTER (WHERE score BETWEEN 21 AND 40),
  count(*) FILTER (WHERE score BETWEEN 41 AND 60),
  count(*) FILTER (WHERE score BETWEEN 61 AND 80),
  count(*) FILTER (WHERE score BETWEEN 81 AND 100)
FROM s`).Scan(&b[0], &b[1], &b[2], &b[3], &b[4]); err != nil {
		return nil, err
	}
	labels := []string{"1–20", "21–40", "41–60", "61–80", "81–100"}
	for i, l := range labels {
		ins.ScoreBuckets = append(ins.ScoreBuckets, ScoreBucket{Label: l, Count: b[i]})
	}

	var err error
	if ins.TopIndustries, err = countQuery(ctx, db, `
SELECT ind, count(*) FROM (
  SELECT unnest(industries) AS ind FROM saas_idea_candidate
) t
WHERE ind <> '' GROUP BY ind ORDER BY count(*) DESC, ind LIMIT 12`); err != nil {
		return nil, err
	}

	if ins.Labels, err = countQuery(ctx, db, `
SELECT COALESCE(label::text, '(unlabeled)'), count(*)
FROM saas_idea_candidate GROUP BY 1 ORDER BY count(*) DESC, 1`); err != nil {
		return nil, err
	}

	return ins, nil
}

func countQuery(ctx context.Context, db *pgxpool.Pool, q string) ([]Count, error) {
	rows, err := db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Count
	for rows.Next() {
		var c Count
		if err := rows.Scan(&c.Name, &c.N); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
