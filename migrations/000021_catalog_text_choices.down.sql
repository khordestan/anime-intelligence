-- Never silently erase selected or restored text/status review history.
DO $downgrade$
BEGIN
 IF EXISTS(SELECT 1 FROM catalog_field_choices WHERE field IN ('title','nativeTitle','synopsis','status'))
 OR EXISTS(SELECT 1 FROM catalog_field_choice_audits WHERE field IN ('title','nativeTitle','synopsis','status')) THEN
  RAISE EXCEPTION 'review retained text choices before downgrade';
 END IF;
ALTER TABLE catalog_field_choices DROP CONSTRAINT catalog_field_choices_field_check;
ALTER TABLE catalog_field_choices ADD CONSTRAINT catalog_field_choices_field_check
 CHECK(field IN ('genres','studios','synonyms','themes','demographics','tags','releaseYear','season','mediaType','episodeCount'));
ALTER TABLE catalog_field_choices DROP CONSTRAINT catalog_field_choices_check;
ALTER TABLE catalog_field_choices ADD CONSTRAINT catalog_field_choices_check CHECK(
 (mode='SELECT' AND source_id IS NOT NULL AND external_id IS NOT NULL AND value IS NOT NULL AND observed_at IS NOT NULL
 AND ((field IN ('genres','studios','synonyms','themes','demographics','tags') AND jsonb_typeof(value)='array')
 OR (field IN ('season','mediaType') AND jsonb_typeof(value)='string')
 OR (field='releaseYear' AND jsonb_typeof(value)='number')
 OR (field='episodeCount' AND jsonb_typeof(value) IN ('number','null'))))
 OR (mode='BASE' AND source_id IS NULL AND external_id IS NULL AND value IS NULL AND observed_at IS NULL)
);

END $downgrade$;
