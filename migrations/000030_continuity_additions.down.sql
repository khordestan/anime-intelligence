DO $rollback$
BEGIN
  LOCK TABLE catalog_continuity_additions IN ACCESS EXCLUSIVE MODE;
  IF EXISTS (SELECT 1 FROM catalog_continuity_additions) THEN
    RAISE EXCEPTION 'refusing to drop retained continuity reviews';
  END IF;
  DROP FUNCTION review_continuity_addition(uuid, uuid, text, text, boolean);
  DROP TABLE catalog_continuity_additions;
END
$rollback$;
