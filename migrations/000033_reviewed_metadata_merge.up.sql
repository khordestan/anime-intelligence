ALTER TABLE anime_redirects ADD COLUMN metadata_digest text
  CHECK (metadata_digest ~ '^[0-9a-f]{64}$');

CREATE FUNCTION review_catalog_metadata_merge(
  retired uuid, survivor uuid, expected_mappings uuid[], expected_digest text, reference text, apply boolean
) RETURNS TABLE(mapping_ids uuid[], observation_digest text)
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, public, pg_temp
SET TimeZone = 'UTC'
SET lock_timeout = '3s'
SET app.catalog_metadata_projection = '1'
AS $review$
DECLARE
  redirect anime_redirects%ROWTYPE;
  source_observation catalog_metadata%ROWTYPE;
  source_display jsonb;
  target_display jsonb;
  provider_key text;
  mapping_facts jsonb;
BEGIN
  IF NOT has_column_privilege(session_user, 'public.anime_redirects', 'review_reference', 'SELECT') THEN
    RAISE EXCEPTION 'catalog merge reviewer required' USING ERRCODE='42501';
  END IF;
  IF retired IS NULL OR survivor IS NULL OR retired=survivor OR apply IS NULL
     OR expected_mappings IS NULL OR cardinality(expected_mappings)>100
     OR reference IS NULL OR length(btrim(reference)) NOT BETWEEN 1 AND 512
     OR (apply AND (expected_digest IS NULL OR expected_digest !~ '^[0-9a-f]{64}$')) THEN
    RAISE EXCEPTION 'invalid reviewed metadata merge';
  END IF;

  PERFORM pg_advisory_xact_lock(742019684);
  IF apply THEN
    LOCK TABLE anime IN ACCESS EXCLUSIVE MODE;
    PERFORM 1 FROM catalog_base_metadata WHERE anime_id IN (retired,survivor) FOR UPDATE;
    PERFORM 1 FROM catalog_metadata WHERE anime_id IN (retired,survivor) FOR UPDATE;
  END IF;

  SELECT r.* INTO redirect FROM anime_redirects r WHERE r.retired_id=retired;
  IF FOUND THEN
    IF redirect.metadata_digest IS NULL OR (apply AND redirect.metadata_digest<>expected_digest) THEN
      RAISE EXCEPTION 'merge differs from recorded metadata review';
    END IF;
    mapping_ids := review_catalog_placeholder_merge(retired, survivor, expected_mappings, reference, apply);
    observation_digest := redirect.metadata_digest;
    RETURN NEXT;
    RETURN;
  END IF;

  SELECT to_jsonb(a)-ARRAY['id','created_at','updated_at'] INTO source_display FROM anime a WHERE id=retired;
  SELECT to_jsonb(a)-ARRAY['id','created_at','updated_at'] INTO target_display FROM anime a WHERE id=survivor;
  IF source_display IS NULL OR source_display IS DISTINCT FROM target_display THEN
    RAISE EXCEPTION 'metadata merge requires matching public title fields';
  END IF;
  IF EXISTS (SELECT 1 FROM catalog_metadata WHERE anime_id=survivor)
     OR EXISTS (SELECT 1 FROM catalog_base_metadata WHERE anime_id=survivor)
     OR EXISTS (SELECT 1 FROM catalog_field_choices WHERE anime_id IN (retired,survivor))
     OR EXISTS (SELECT 1 FROM airing_schedule WHERE anime_id IN (retired,survivor)) THEN
    RAISE EXCEPTION 'metadata merge requires an unused target authority and no schedule or field decisions';
  END IF;

  SELECT m.* INTO source_observation FROM catalog_metadata m WHERE m.anime_id=retired;
  IF NOT FOUND OR NOT EXISTS (
    SELECT 1 FROM catalog_base_metadata b
    WHERE b.anime_id=retired AND (b.source_id,b.observed_at,b.payload)=
      (source_observation.source_id,source_observation.observed_at,source_observation.payload)
  ) OR COALESCE(source_observation.payload->'relations','[]'::jsonb)<>'[]'::jsonb
    OR COALESCE(source_observation.payload->'episodes','[]'::jsonb)<>'[]'::jsonb THEN
    RAISE EXCEPTION 'metadata merge requires one unprojected observation without relation or episode claims';
  END IF;
  SELECT s.provider_key INTO provider_key FROM data_sources s
  JOIN anime_external_ids i ON i.provider=s.provider_key AND i.anime_id=retired
  WHERE s.id=source_observation.source_id AND s.enabled AND btrim(COALESCE(s.policy_url,''))<>''
    AND i.external_id=source_observation.payload->>'externalId';
  IF NOT FOUND THEN
    RAISE EXCEPTION 'metadata observation has no approved exact provider mapping';
  END IF;

  IF EXISTS (SELECT 1 FROM anime_external_ids a JOIN anime_external_ids b USING(provider)
    WHERE a.anime_id=retired AND b.anime_id=survivor) THEN
    RAISE EXCEPTION 'provider identity slots overlap';
  END IF;
  SELECT COALESCE(array_agg(i.id ORDER BY i.id),ARRAY[]::uuid[]),
         jsonb_agg(jsonb_build_array(i.id,i.provider,i.external_id) ORDER BY i.id)
    INTO mapping_ids,mapping_facts
    FROM anime_external_ids i WHERE i.anime_id=retired;
  IF cardinality(mapping_ids)>100 THEN
    RAISE EXCEPTION 'too many provider mappings for metadata merge';
  END IF;
  observation_digest := encode(sha256(convert_to(jsonb_build_array(
    retired,survivor,source_display,to_jsonb(source_observation),provider_key,mapping_facts
  )::text,'UTF8')),'hex');
  IF apply THEN
    IF observation_digest<>expected_digest THEN
      RAISE EXCEPTION 'metadata observation changed since review';
    END IF;
    UPDATE catalog_base_metadata SET anime_id=survivor WHERE anime_id=retired;
    UPDATE catalog_metadata SET anime_id=survivor WHERE anime_id=retired;
    -- The original procedure checks all remaining dependencies and records the
    -- redirect. Any conflict rolls back the metadata transfer and the redirect.
    mapping_ids := review_catalog_placeholder_merge(retired, survivor, expected_mappings, reference, true);
    UPDATE anime_redirects SET metadata_digest=observation_digest WHERE retired_id=retired;
  END IF;
  RETURN NEXT;
END
$review$;

REVOKE ALL ON FUNCTION review_catalog_metadata_merge(uuid, uuid, uuid[], text, text, boolean) FROM PUBLIC;
