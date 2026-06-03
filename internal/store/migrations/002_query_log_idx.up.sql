-- Ensure search_query_log has an index on provider for reporting.
CREATE INDEX IF NOT EXISTS search_query_log_provider_idx ON search_query_log (provider);
CREATE INDEX IF NOT EXISTS search_query_log_ran_at_idx   ON search_query_log (ran_at DESC);
