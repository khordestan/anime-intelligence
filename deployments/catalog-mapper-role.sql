-- Reviewed exact-ID links only. Set the password outside source control.
DO $role$ BEGIN
CREATE ROLE anime_catalog_mapper LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
 NOINHERIT NOREPLICATION NOBYPASSRLS;
EXCEPTION WHEN duplicate_object THEN NULL;
END $role$;
GRANT USAGE ON SCHEMA public TO anime_catalog_mapper;
GRANT SELECT ON anime,anime_external_ids,data_sources TO anime_catalog_mapper;
GRANT SELECT(source_id,external_id,content_hash) ON source_records TO anime_catalog_mapper;
GRANT INSERT ON anime_external_ids,source_records TO anime_catalog_mapper;
-- Row locks need an UPDATE grant; these columns cannot remap identities/enable sources.
GRANT UPDATE(external_url) ON anime_external_ids TO anime_catalog_mapper;
GRANT UPDATE(display_name) ON data_sources TO anime_catalog_mapper;

-- Migration 000022: invoker-rights identity retirement fence.
GRANT SELECT ON catalog_identity_retractions TO anime_catalog_mapper;
