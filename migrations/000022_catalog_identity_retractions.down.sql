DO $downgrade$
BEGIN
 IF EXISTS(SELECT 1 FROM catalog_identity_retractions) THEN
  RAISE EXCEPTION 'review retained identity retractions before downgrade';
 END IF;
 DROP TRIGGER catalog_identity_retirement_guard ON anime_external_ids;
 DROP FUNCTION catalog_identity_retirement_guard();
 DROP TABLE catalog_identity_retractions;
END $downgrade$;
