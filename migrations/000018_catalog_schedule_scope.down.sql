-- Review/archive scoped jobs and observations before downgrade. Refuse atomically
-- rather than silently turn schedule-only jobs into unrestricted metadata imports.
DO $downgrade$
BEGIN
 IF EXISTS(SELECT 1 FROM catalog_fetch_jobs WHERE import_scope='SCHEDULES')
 OR EXISTS(SELECT 1 FROM catalog_schedule_observations) THEN
  RAISE EXCEPTION 'review retained schedule-only work before downgrade';
 END IF;
 DROP TRIGGER catalog_fetch_scope_guard ON catalog_fetch_jobs;
 DROP FUNCTION catalog_fetch_scope_guard();
 ALTER TABLE catalog_fetch_jobs DROP COLUMN import_scope;
 DROP TABLE catalog_schedule_observations;
END $downgrade$;
