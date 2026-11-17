-- Optional dedicated catalog worker/import login. Set its password out of band.
-- It cannot enable sources, alter policy URLs or access private account tables.
DO $role$ BEGIN
CREATE ROLE anime_catalog_writer LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
  NOINHERIT NOREPLICATION NOBYPASSRLS;
EXCEPTION WHEN duplicate_object THEN NULL;
END $role$;
GRANT USAGE ON SCHEMA public TO anime_catalog_writer;
GRANT SELECT ON data_sources TO anime_catalog_writer;
-- PostgreSQL row locks require UPDATE privilege on at least one column.
-- Only the display label is writable; approval flags/policies/keys remain protected.
GRANT UPDATE(display_name) ON data_sources TO anime_catalog_writer;
GRANT SELECT,INSERT ON anime,anime_external_ids,catalog_metadata,catalog_sync_state,
  airing_schedule,anime_relations,domain_events TO anime_catalog_writer;
GRANT UPDATE(canonical_title,native_title,synopsis,media_type,status,release_year,
  season_name,episode_count,updated_at) ON anime TO anime_catalog_writer;
GRANT UPDATE(external_url) ON anime_external_ids TO anime_catalog_writer;
GRANT UPDATE(observed_at,payload) ON catalog_metadata TO anime_catalog_writer;
GRANT UPDATE(observed_at) ON catalog_sync_state TO anime_catalog_writer;
GRANT UPDATE(last_verified_at) ON airing_schedule TO anime_catalog_writer;
GRANT UPDATE(confidence) ON anime_relations TO anime_catalog_writer;

GRANT SELECT,INSERT,UPDATE ON catalog_fetch_state TO anime_catalog_writer;

-- Migration 000016: durable explicit-title jobs; no API/runtime grants.
GRANT SELECT,INSERT,UPDATE ON catalog_fetch_jobs TO anime_catalog_writer;

-- Migration 000018: public schedule observation watermarks only.
GRANT SELECT,INSERT ON catalog_schedule_observations TO anime_catalog_writer;
GRANT UPDATE(observed_at) ON catalog_schedule_observations TO anime_catalog_writer;
