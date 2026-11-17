-- Explicit public catalog work only. No account lists, payloads or credentials.
CREATE TABLE catalog_fetch_jobs (
  source_id UUID NOT NULL REFERENCES data_sources(id) ON DELETE CASCADE,
  external_id TEXT NOT NULL CHECK (external_id ~ '^Q[1-9][0-9]{0,11}$'),
  interval_seconds INT NOT NULL CHECK (interval_seconds BETWEEN 3600 AND 604800),
  next_attempt_at TIMESTAMPTZ NOT NULL,
  state TEXT NOT NULL DEFAULT 'PENDING'
    CHECK (state IN ('PENDING','LEASED','RETRY','SUCCESS','FAILED')),
  attempts INT NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
  lease_digest BYTEA,
  lease_until TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (source_id,external_id),
  CHECK ((state='LEASED' AND octet_length(lease_digest)=32 AND lease_digest IS NOT NULL AND lease_until IS NOT NULL)
      OR (state<>'LEASED' AND lease_digest IS NULL AND lease_until IS NULL))
);
CREATE INDEX catalog_fetch_jobs_due_idx ON catalog_fetch_jobs(source_id,next_attempt_at,external_id)
  WHERE state<>'FAILED';
