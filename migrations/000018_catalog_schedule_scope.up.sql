-- Separate reviewed schedule observations from canonical metadata authority.
CREATE TABLE catalog_schedule_observations (
 source_id UUID NOT NULL REFERENCES data_sources(id) ON DELETE CASCADE,
 anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
 observed_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(source_id,anime_id)
);
ALTER TABLE catalog_fetch_jobs ADD COLUMN import_scope TEXT NOT NULL DEFAULT 'FULL'
 CHECK(import_scope IN ('FULL','SCHEDULES'));

-- Compatibility fence: older queue binaries must fail before fetching or
-- acknowledging schedule-only work as a full metadata import. This is not an
-- authorization boundary; the catalog writer remains a trusted public writer.
CREATE FUNCTION catalog_fetch_scope_guard() RETURNS trigger LANGUAGE plpgsql AS $guard$
BEGIN
 IF NEW.import_scope='SCHEDULES'
 AND current_setting('app.catalog_import_scope',true) IS DISTINCT FROM 'SCHEDULES' THEN
  RAISE EXCEPTION 'explicit schedule import context required';
 END IF;
 RETURN NEW;
END $guard$;
CREATE TRIGGER catalog_fetch_scope_guard BEFORE INSERT OR UPDATE ON catalog_fetch_jobs
 FOR EACH ROW EXECUTE FUNCTION catalog_fetch_scope_guard();
