-- Downgrade fails atomically while TVmaze jobs remain. Review/archive retained
-- public jobs explicitly; never silently delete durable work on rollback.
DO $downgrade$
BEGIN
 ALTER TABLE catalog_fetch_jobs DROP CONSTRAINT catalog_fetch_jobs_external_id_check;
 ALTER TABLE catalog_fetch_jobs ADD CONSTRAINT catalog_fetch_jobs_external_id_check
  CHECK (external_id ~ '^Q[1-9][0-9]{0,11}$');
END $downgrade$;
