-- Optional diagnostics-only login. Provision its password outside source control.
-- No private-table privileges and no writes, source activation or migrations.
DO $role$ BEGIN
CREATE ROLE anime_catalog_reader LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
  NOINHERIT NOREPLICATION NOBYPASSRLS;
EXCEPTION WHEN duplicate_object THEN NULL;
END $role$;
GRANT USAGE ON SCHEMA public TO anime_catalog_reader;
GRANT SELECT ON data_sources,catalog_sync_state,catalog_metadata,anime,airing_schedule
  TO anime_catalog_reader;

GRANT SELECT ON catalog_fetch_state TO anime_catalog_reader;
