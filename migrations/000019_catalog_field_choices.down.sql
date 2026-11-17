-- Explicitly review/archive decisions before downgrade; never discard choices
-- or audit history or make an old primary writer silently replace them.
DO $downgrade$
BEGIN
 IF EXISTS(SELECT 1 FROM catalog_field_choices) OR EXISTS(SELECT 1 FROM catalog_field_choice_audits) THEN
  RAISE EXCEPTION 'review retained catalog field choices before downgrade';
 END IF;
 DROP TRIGGER catalog_metadata_projection_guard ON catalog_metadata;
 DROP FUNCTION catalog_metadata_projection_guard();
 DROP TABLE catalog_field_choice_audits;
 DROP TABLE catalog_field_choices;
 DROP TABLE catalog_base_metadata;
END $downgrade$;
