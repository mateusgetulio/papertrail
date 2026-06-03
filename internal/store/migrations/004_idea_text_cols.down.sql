ALTER TABLE saas_idea_candidate
  DROP COLUMN IF EXISTS pain_point,
  DROP COLUMN IF EXISTS industries,
  DROP COLUMN IF EXISTS countries_or_regions;
