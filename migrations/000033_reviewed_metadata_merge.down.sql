DO $rollback$
BEGIN
  LOCK TABLE anime IN ACCESS EXCLUSIVE MODE;
  LOCK TABLE anime_redirects IN ACCESS EXCLUSIVE MODE;
  IF EXISTS (SELECT 1 FROM anime_redirects WHERE metadata_digest IS NOT NULL) THEN
    RAISE EXCEPTION 'refusing to drop reviewed metadata merge evidence';
  END IF;
  DROP FUNCTION review_catalog_metadata_merge(uuid, uuid, uuid[], text, text, boolean);
  ALTER TABLE anime_redirects DROP COLUMN metadata_digest;
END
$rollback$;
