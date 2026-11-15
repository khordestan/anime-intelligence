-- Outcome codes are bounded operational metadata, never provider responses.
-- Legacy completions cannot be reliably inferred from timestamps/attempts.
ALTER TABLE identity_requests ADD COLUMN outcome TEXT,
 ADD CONSTRAINT identity_requests_outcome_check CHECK(
  outcome IS NULL OR (completed_at IS NOT NULL AND outcome IN ('ISSUED','NO_ACTION','EXPIRED','EXHAUSTED','LEGACY_UNKNOWN'))
 );
ALTER TABLE identity_outbox ADD COLUMN outcome TEXT,
 ADD CONSTRAINT identity_outbox_outcome_check CHECK(
  outcome IS NULL OR (completed_at IS NOT NULL AND outcome IN ('ACCEPTED','INVALIDATED','EXPIRED','EXHAUSTED','LEGACY_UNKNOWN'))
 );
SELECT set_config('app.identity_request_worker','on',true),set_config('app.delivery_worker','on',true);
UPDATE identity_requests SET outcome='LEGACY_UNKNOWN' WHERE completed_at IS NOT NULL;
UPDATE identity_outbox SET outcome='LEGACY_UNKNOWN' WHERE completed_at IS NOT NULL;
SELECT set_config('app.identity_request_worker','',true),set_config('app.delivery_worker','',true);
-- NULL on a completed row remains permitted for compatible old writers during
-- a rolling upgrade; diagnostics explicitly classify it as legacy unknown.
