CREATE TABLE anime_redirects (
  retired_id uuid PRIMARY KEY,
  anime_id uuid NOT NULL REFERENCES anime(id),
  transferred_mapping_ids uuid[] NOT NULL,
  review_reference text NOT NULL CHECK (length(btrim(review_reference)) BETWEEN 1 AND 512),
  reviewed_at timestamptz NOT NULL DEFAULT statement_timestamp(),
  CHECK (retired_id <> anime_id)
);

CREATE FUNCTION review_catalog_placeholder_merge(
  retired uuid, survivor uuid, expected_mappings uuid[], reference text, apply boolean
) RETURNS uuid[] LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, public, pg_temp
SET row_security = off
AS $review$
DECLARE
  existing anime_redirects%ROWTYPE;
  checked_fk record;
  dependency_exists boolean;
  mappings uuid[];
  expected uuid[];
BEGIN
  IF NOT has_column_privilege(session_user, 'public.anime_redirects', 'review_reference', 'SELECT') THEN
    RAISE EXCEPTION 'catalog merge reviewer required' USING ERRCODE='42501';
  END IF;
  IF retired IS NULL OR survivor IS NULL OR retired=survivor OR apply IS NULL
     OR expected_mappings IS NULL OR cardinality(expected_mappings)>100
     OR reference IS NULL OR length(btrim(reference)) NOT BETWEEN 1 AND 512 THEN
    RAISE EXCEPTION 'invalid catalog merge review';
  END IF;
  SELECT ARRAY(SELECT id FROM unnest(expected_mappings) AS id ORDER BY id) INTO expected;
  PERFORM pg_advisory_xact_lock(742019684);
  IF apply THEN
    -- Fence new foreign-key references to either title while checking and moving.
    LOCK TABLE anime IN ACCESS EXCLUSIVE MODE;
  END IF;

  SELECT * INTO existing FROM anime_redirects WHERE retired_id=retired;
  IF FOUND THEN
    IF existing.anime_id<>survivor OR existing.review_reference<>btrim(reference)
       OR (apply AND existing.transferred_mapping_ids IS DISTINCT FROM expected) THEN
      RAISE EXCEPTION 'merge differs from recorded review';
    END IF;
    RETURN existing.transferred_mapping_ids;
  END IF;

  IF (SELECT count(*) FROM anime WHERE id IN (retired,survivor))<>2 THEN
    RAISE EXCEPTION 'merge requires two active canonical titles';
  END IF;

  -- Refuse every inbound dependency except exact provider identity rows. Resolve
  -- foreign keys from the current schema so future tables also fail closed.
  FOR checked_fk IN
    SELECT n.nspname AS schema_name,c.relname AS table_name,a.attname AS column_name,
           p.conkey=ARRAY[a.attnum] AND p.confkey=ARRAY[id.attnum] AS single_id
    FROM pg_constraint p
    JOIN pg_class c ON c.oid=p.conrelid
    JOIN pg_namespace n ON n.oid=c.relnamespace
    JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum=p.conkey[1]
    JOIN pg_attribute id ON id.attrelid=p.confrelid AND id.attname='id'
    WHERE p.contype='f' AND p.confrelid='public.anime'::regclass AND p.conparentid=0
  LOOP
    IF NOT checked_fk.single_id THEN
      RAISE EXCEPTION 'unsupported canonical dependency';
    END IF;
    IF checked_fk.schema_name='public' AND checked_fk.table_name='anime_external_ids'
       AND checked_fk.column_name='anime_id' THEN
      CONTINUE;
    END IF;
    EXECUTE format('SELECT EXISTS (SELECT 1 FROM %I.%I WHERE %I=$1)',
      checked_fk.schema_name,checked_fk.table_name,checked_fk.column_name)
    INTO dependency_exists USING retired;
    IF dependency_exists THEN
      RAISE EXCEPTION 'retired title has dependent records';
    END IF;
  END LOOP;

  IF EXISTS (SELECT 1 FROM domain_events WHERE aggregate_type='ANIME' AND aggregate_id=retired)
     OR EXISTS (
       SELECT 1 FROM anime_external_ids i JOIN data_sources s ON s.provider_key=i.provider
       WHERE i.anime_id=retired AND (
         EXISTS (SELECT 1 FROM source_records r WHERE r.source_id=s.id AND r.external_id=i.external_id)
         OR EXISTS (SELECT 1 FROM catalog_fetch_jobs j WHERE j.source_id=s.id AND j.external_id=i.external_id)
         OR EXISTS (
           SELECT 1 FROM catalog_base_metadata b
           CROSS JOIN LATERAL jsonb_array_elements(COALESCE(b.payload->'relations','[]'::jsonb)) AS relation
           WHERE b.source_id=s.id AND relation->>'externalId'=i.external_id
         )
       )
     ) THEN
    RAISE EXCEPTION 'retired title has indirect catalog dependencies';
  END IF;

  IF EXISTS (SELECT 1 FROM anime_external_ids a JOIN anime_external_ids b USING(provider)
    WHERE a.anime_id=retired AND b.anime_id=survivor) THEN
    RAISE EXCEPTION 'provider identity slots overlap';
  END IF;
  SELECT COALESCE(array_agg(id ORDER BY id),ARRAY[]::uuid[]) INTO mappings
    FROM anime_external_ids WHERE anime_id=retired;
  IF cardinality(mappings)>100 THEN
    RAISE EXCEPTION 'too many provider mappings for placeholder merge';
  END IF;
  IF NOT apply THEN
    RETURN mappings;
  END IF;
  IF mappings IS DISTINCT FROM expected THEN
    RAISE EXCEPTION 'provider mappings changed since review';
  END IF;
  UPDATE anime_external_ids SET anime_id=survivor WHERE anime_id=retired;
  DELETE FROM anime WHERE id=retired;
  INSERT INTO anime_redirects(retired_id,anime_id,transferred_mapping_ids,review_reference)
    VALUES(retired,survivor,mappings,btrim(reference));
  RETURN mappings;
END
$review$;

CREATE FUNCTION guard_retired_anime_identity() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $guard$
BEGIN
  IF EXISTS (SELECT 1 FROM anime_redirects WHERE retired_id=NEW.id) THEN
    RAISE EXCEPTION 'canonical title ID is retired' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END
$guard$;

REVOKE ALL ON FUNCTION review_catalog_placeholder_merge(uuid, uuid, uuid[], text, boolean) FROM PUBLIC;
REVOKE ALL ON FUNCTION guard_retired_anime_identity() FROM PUBLIC;

CREATE TRIGGER guard_retired_anime_identity
  BEFORE INSERT OR UPDATE OF id ON anime FOR EACH ROW
  EXECUTE FUNCTION guard_retired_anime_identity();
