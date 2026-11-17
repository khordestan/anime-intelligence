-- Refuse to erase scalar decisions or their BASE/revision audit history.
DO $downgrade$
BEGIN
 IF EXISTS(SELECT 1 FROM catalog_field_choices WHERE field IN ('releaseYear','season','mediaType','episodeCount'))
 OR EXISTS(SELECT 1 FROM catalog_field_choice_audits WHERE field IN ('releaseYear','season','mediaType','episodeCount')) THEN
  RAISE EXCEPTION 'review retained scalar choices before downgrade';
 END IF;
 ALTER TABLE catalog_field_choices DROP CONSTRAINT catalog_field_choices_field_check;
 ALTER TABLE catalog_field_choices ADD CONSTRAINT catalog_field_choices_field_check
  CHECK(field IN ('genres','studios','synonyms','themes','demographics','tags'));
 ALTER TABLE catalog_field_choices DROP CONSTRAINT catalog_field_choices_check;
 ALTER TABLE catalog_field_choices ADD CONSTRAINT catalog_field_choices_check CHECK(
 (mode='SELECT' AND source_id IS NOT NULL AND external_id IS NOT NULL AND value IS NOT NULL
 AND jsonb_typeof(value)='array' AND observed_at IS NOT NULL)
 OR (mode='BASE' AND source_id IS NULL AND external_id IS NULL AND value IS NULL AND observed_at IS NULL));
END $downgrade$;
