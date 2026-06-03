ALTER TABLE disruption_signal
  ADD COLUMN IF NOT EXISTS pain_point TEXT,
  ADD COLUMN IF NOT EXISTS industries TEXT[],
  ADD COLUMN IF NOT EXISTS regions    TEXT[],
  ADD COLUMN IF NOT EXISTS chunk_refs INT[];
