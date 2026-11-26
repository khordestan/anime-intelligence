DO $role$
BEGIN
  CREATE ROLE anime_catalog_continuity_curator LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
    NOINHERIT NOREPLICATION NOBYPASSRLS;
EXCEPTION WHEN duplicate_object THEN NULL;
END
$role$;
GRANT USAGE ON SCHEMA public TO anime_catalog_continuity_curator;
GRANT SELECT(review_reference) ON catalog_continuity_retractions TO anime_catalog_continuity_curator;
GRANT EXECUTE ON FUNCTION review_continuity_retraction(uuid, uuid, uuid[], text, boolean)
  TO anime_catalog_continuity_curator;
GRANT SELECT(review_reference) ON catalog_continuity_additions TO anime_catalog_continuity_curator;
GRANT EXECUTE ON FUNCTION review_continuity_addition(uuid, uuid, text, text, boolean)
  TO anime_catalog_continuity_curator;
