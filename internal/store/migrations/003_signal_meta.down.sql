ALTER TABLE disruption_signal
  DROP COLUMN IF EXISTS pain_point,
  DROP COLUMN IF EXISTS industries,
  DROP COLUMN IF EXISTS regions,
  DROP COLUMN IF EXISTS chunk_refs;
