CREATE TABLE catalog_continuity_retractions (
  prequel_id uuid NOT NULL REFERENCES anime(id),
  sequel_id uuid NOT NULL REFERENCES anime(id),
  removed_relation_ids uuid[] NOT NULL CHECK (cardinality(removed_relation_ids) BETWEEN 1 AND 2),
  review_reference text NOT NULL CHECK (length(btrim(review_reference)) BETWEEN 1 AND 512),
  reviewed_at timestamptz NOT NULL DEFAULT statement_timestamp(),
  PRIMARY KEY (prequel_id, sequel_id),
  CHECK (prequel_id <> sequel_id)
);

-- The curator may use only this bounded operation. No direct delete privilege is needed.
CREATE FUNCTION review_continuity_retraction(
  first_title uuid, second_title uuid, expected uuid[], reference text, apply boolean
) RETURNS uuid[] LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, public, pg_temp
AS $review$
DECLARE
  found_ids uuid[];
  saved_ids uuid[];
  saved_reference text;
BEGIN
  -- Logical restore without ACLs recreates default function permissions.
  -- Require the curator's explicit column grant even in that interval.
  IF NOT has_column_privilege(session_user, 'public.catalog_continuity_retractions',
      'review_reference', 'SELECT') THEN
    RAISE EXCEPTION 'continuity curator required' USING ERRCODE='42501';
  END IF;
  IF first_title IS NULL OR second_title IS NULL OR first_title = second_title
     OR apply IS NULL OR expected IS NULL OR cardinality(expected) > 2
     OR reference IS NULL OR length(btrim(reference)) NOT BETWEEN 1 AND 512 THEN
    RAISE EXCEPTION 'invalid continuity review';
  END IF;
  PERFORM pg_advisory_xact_lock(742019684);

  SELECT r.removed_relation_ids, r.review_reference INTO saved_ids, saved_reference
    FROM catalog_continuity_retractions r
    WHERE r.prequel_id = first_title AND r.sequel_id = second_title;
  IF FOUND THEN
    IF apply AND (saved_reference <> btrim(reference) OR
        saved_ids <> ARRAY(SELECT id FROM unnest(expected) AS id ORDER BY id)) THEN
      RAISE EXCEPTION 'continuity review differs from recorded decision';
    END IF;
    RETURN saved_ids;
  END IF;

  SELECT coalesce(array_agg(r.id ORDER BY r.id), ARRAY[]::uuid[]) INTO found_ids
    FROM anime_relations r
    WHERE (r.source_anime_id=first_title AND r.target_anime_id=second_title AND r.relation_type='SEQUEL')
       OR (r.source_anime_id=second_title AND r.target_anime_id=first_title AND r.relation_type='PREQUEL');
  IF NOT apply THEN
    RETURN found_ids;
  END IF;
  IF cardinality(found_ids) = 0 OR found_ids <> ARRAY(SELECT id FROM unnest(expected) AS id ORDER BY id) THEN
    RAISE EXCEPTION 'continuity links changed since review';
  END IF;
  INSERT INTO catalog_continuity_retractions(prequel_id,sequel_id,removed_relation_ids,review_reference)
    VALUES(first_title,second_title,found_ids,btrim(reference));
  DELETE FROM anime_relations WHERE id = ANY(found_ids);
  RETURN found_ids;
END
$review$;

REVOKE ALL ON FUNCTION review_continuity_retraction(uuid, uuid, uuid[], text, boolean) FROM PUBLIC;

CREATE FUNCTION guard_continuity_retraction() RETURNS trigger LANGUAGE plpgsql
SET search_path = pg_catalog, public, pg_temp AS $guard$
BEGIN
  IF NEW.relation_type IN ('PREQUEL', 'SEQUEL') THEN
    PERFORM pg_advisory_xact_lock(742019684);
    IF EXISTS (
      SELECT 1 FROM catalog_continuity_retractions r
      WHERE r.prequel_id = CASE NEW.relation_type WHEN 'SEQUEL' THEN NEW.source_anime_id ELSE NEW.target_anime_id END
        AND r.sequel_id = CASE NEW.relation_type WHEN 'SEQUEL' THEN NEW.target_anime_id ELSE NEW.source_anime_id END
    ) THEN
      RAISE EXCEPTION 'continuity link is retired' USING ERRCODE='23514', CONSTRAINT='catalog_continuity_retired';
    END IF;
  END IF;
  RETURN NEW;
END
$guard$;

REVOKE ALL ON FUNCTION guard_continuity_retraction() FROM PUBLIC;

CREATE TRIGGER catalog_continuity_retirement_guard
  BEFORE INSERT OR UPDATE OF source_anime_id, target_anime_id, relation_type ON anime_relations
  FOR EACH ROW EXECUTE FUNCTION guard_continuity_retraction();
