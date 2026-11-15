-- Public source operations only: no request bodies, credentials or item lists.
CREATE TABLE catalog_fetch_state (
  source_id UUID PRIMARY KEY REFERENCES data_sources(id) ON DELETE CASCADE,
  last_attempt_at TIMESTAMPTZ NOT NULL,
  next_attempt_at TIMESTAMPTZ NOT NULL,
  outcome TEXT NOT NULL CHECK(outcome IN ('STARTED','SUCCESS','FETCH_FAILED','RATE_LIMITED','IMPORT_FAILED')),
  attempts INT NOT NULL DEFAULT 1 CHECK(attempts BETWEEN 1 AND 1000000000),
  CHECK(next_attempt_at>=last_attempt_at)
);
