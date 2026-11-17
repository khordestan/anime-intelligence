-- Reviewed label and bounded scalar decisions only. Provision password out of band.
DO $role$ BEGIN
 CREATE ROLE anime_catalog_field_curator LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
 NOINHERIT NOREPLICATION NOBYPASSRLS;
EXCEPTION WHEN duplicate_object THEN NULL;
END $role$;
GRANT USAGE ON SCHEMA public TO anime_catalog_field_curator;
GRANT SELECT ON anime,anime_external_ids,data_sources,catalog_metadata,catalog_base_metadata,
 catalog_field_choices,catalog_field_choice_audits TO anime_catalog_field_curator;
-- Source locks require an update privilege; source approval remains immutable.
GRANT UPDATE(display_name) ON data_sources TO anime_catalog_field_curator;
GRANT UPDATE(payload) ON catalog_metadata TO anime_catalog_field_curator;
GRANT INSERT,UPDATE ON catalog_field_choices TO anime_catalog_field_curator;
GRANT INSERT ON catalog_field_choice_audits TO anime_catalog_field_curator;

-- Migration 000020: public scalar projection and retained-episode validation.
GRANT SELECT ON airing_schedule TO anime_catalog_field_curator;
GRANT UPDATE(release_year,season_name,media_type,episode_count,updated_at) ON anime TO anime_catalog_field_curator;
