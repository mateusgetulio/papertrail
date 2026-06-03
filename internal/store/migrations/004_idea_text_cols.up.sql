ALTER TABLE saas_idea_candidate
  ADD COLUMN IF NOT EXISTS pain_point          TEXT,
  ADD COLUMN IF NOT EXISTS industries          TEXT[],
  ADD COLUMN IF NOT EXISTS countries_or_regions TEXT[];
