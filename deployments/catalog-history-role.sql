-- Optional operator-only observation reader. Set credentials outside source control.
-- API runtime and identity mapper roles do not receive archive payload access.
DO $role$ BEGIN
 CREATE ROLE anime_catalog_history LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
 NOINHERIT NOREPLICATION NOBYPASSRLS;
EXCEPTION WHEN duplicate_object THEN NULL;
END $role$;
GRANT USAGE ON SCHEMA public TO anime_catalog_history;
GRANT SELECT(id,provider_key) ON data_sources TO anime_catalog_history;
GRANT SELECT(id,source_id,external_id,record_type,payload) ON source_records TO anime_catalog_history;
