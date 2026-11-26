CREATE TABLE catalog_continuity_additions (
  prequel_id uuid NOT NULL REFERENCES anime(id),
  sequel_id uuid NOT NULL REFERENCES anime(id),
  relation_id uuid NOT NULL UNIQUE,
  source_id uuid NOT NULL REFERENCES data_sources(id),
  review_reference text NOT NULL CHECK (length(btrim(review_reference)) BETWEEN 1 AND 512),
  reviewed_at timestamptz NOT NULL DEFAULT statement_timestamp(),
  PRIMARY KEY (prequel_id, sequel_id),
  CHECK (prequel_id <> sequel_id)
);

CREATE FUNCTION review_continuity_addition(
  first_title uuid, second_title uuid, provider text, reference text, apply boolean
) RETURNS uuid LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, public, pg_temp
AS $review$
DECLARE
  reviewed catalog_continuity_additions%ROWTYPE;
  approved_source uuid;
  created_relation uuid;
BEGIN
  IF NOT has_column_privilege(session_user, 'public.catalog_continuity_additions',
      'review_reference', 'SELECT') THEN
    RAISE EXCEPTION 'continuity curator required' USING ERRCODE='42501';
  END IF;
  IF first_title IS NULL OR second_title IS NULL OR first_title = second_title
     OR apply IS NULL OR provider IS NULL OR length(provider) NOT BETWEEN 1 AND 128
     OR reference IS NULL OR length(btrim(reference)) NOT BETWEEN 1 AND 512 THEN
    RAISE EXCEPTION 'invalid continuity review';
  END IF;
  PERFORM pg_advisory_xact_lock(742019684);

  SELECT id INTO approved_source FROM data_sources
    WHERE provider_key=provider AND enabled AND COALESCE(policy_url,'')<>''
    FOR SHARE;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'review source is unavailable';
  END IF;
  IF (SELECT count(*) FROM anime WHERE id IN (first_title,second_title)) <> 2 THEN
    RAISE EXCEPTION 'review requires two canonical titles';
  END IF;
  IF EXISTS (SELECT 1 FROM catalog_continuity_retractions r
    WHERE r.prequel_id=first_title AND r.sequel_id=second_title) THEN
    RAISE EXCEPTION 'continuity link is retired';
  END IF;

  SELECT * INTO reviewed FROM catalog_continuity_additions
    WHERE prequel_id=first_title AND sequel_id=second_title;
  IF FOUND THEN
    IF reviewed.source_id<>approved_source OR reviewed.review_reference<>btrim(reference) THEN
      RAISE EXCEPTION 'continuity review differs from recorded decision';
    END IF;
    RETURN reviewed.relation_id;
  END IF;
  IF EXISTS (SELECT 1 FROM anime_relations r
    WHERE (r.source_anime_id=first_title AND r.target_anime_id=second_title AND r.relation_type='SEQUEL')
       OR (r.source_anime_id=second_title AND r.target_anime_id=first_title AND r.relation_type='PREQUEL')) THEN
    RAISE EXCEPTION 'continuity link already exists';
  END IF;
  IF EXISTS (
    WITH RECURSIVE successors(id) AS (
      SELECT second_title
      UNION
      SELECT CASE r.relation_type WHEN 'SEQUEL' THEN r.target_anime_id ELSE r.source_anime_id END
      FROM successors s JOIN anime_relations r
        ON (r.relation_type='SEQUEL' AND r.source_anime_id=s.id)
        OR (r.relation_type='PREQUEL' AND r.target_anime_id=s.id)
    )
    SELECT 1 FROM successors WHERE id=first_title
  ) THEN
    RAISE EXCEPTION 'continuity link would form a cycle';
  END IF;

  IF NOT apply THEN
    RETURN NULL;
  END IF;
  INSERT INTO anime_relations(source_anime_id,target_anime_id,relation_type,source_id)
    VALUES(first_title,second_title,'SEQUEL',approved_source) RETURNING id INTO created_relation;
  INSERT INTO catalog_continuity_additions(prequel_id,sequel_id,relation_id,source_id,review_reference)
    VALUES(first_title,second_title,created_relation,approved_source,btrim(reference));
  RETURN created_relation;
END
$review$;

REVOKE ALL ON FUNCTION review_continuity_addition(uuid, uuid, text, text, boolean) FROM PUBLIC;
