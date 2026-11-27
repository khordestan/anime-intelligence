DO $role$
BEGIN
  CREATE ROLE anime_catalog_merge_reviewer LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
    NOINHERIT NOREPLICATION NOBYPASSRLS;
EXCEPTION WHEN duplicate_object THEN NULL;
END
$role$;
GRANT USAGE ON SCHEMA public TO anime_catalog_merge_reviewer;
GRANT SELECT(review_reference) ON anime_redirects TO anime_catalog_merge_reviewer;
GRANT EXECUTE ON FUNCTION review_catalog_placeholder_merge(uuid, uuid, uuid[], text, boolean)
  TO anime_catalog_merge_reviewer;
GRANT EXECUTE ON FUNCTION review_catalog_metadata_merge(uuid, uuid, uuid[], text, text, boolean)
  TO anime_catalog_merge_reviewer;
