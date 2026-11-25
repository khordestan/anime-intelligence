DO $rollback$
BEGIN
  IF EXISTS (SELECT 1 FROM catalog_continuity_retractions) THEN
    RAISE EXCEPTION 'continuity retractions require explicit review before rollback';
  END IF;
END
$rollback$;
DROP TRIGGER catalog_continuity_retirement_guard ON anime_relations;
DROP FUNCTION guard_continuity_retraction();
DROP FUNCTION review_continuity_retraction(uuid, uuid, uuid[], text, boolean);
DROP TABLE catalog_continuity_retractions;
