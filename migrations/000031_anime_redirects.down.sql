DO $rollback$
BEGIN
  LOCK TABLE anime IN ACCESS EXCLUSIVE MODE;
  LOCK TABLE anime_redirects IN ACCESS EXCLUSIVE MODE;
  IF EXISTS (SELECT 1 FROM anime_redirects) THEN
    RAISE EXCEPTION 'refusing to drop retained canonical redirects';
  END IF;
  DROP TRIGGER guard_retired_anime_identity ON anime;
  DROP FUNCTION guard_retired_anime_identity();
  DROP FUNCTION review_catalog_placeholder_merge(uuid, uuid, uuid[], text, boolean);
  DROP TABLE anime_redirects;
END
$rollback$;
