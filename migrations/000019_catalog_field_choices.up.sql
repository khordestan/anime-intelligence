-- Latest primary observation, not a raw provider response/history archive.
CREATE TABLE catalog_base_metadata (
 anime_id UUID PRIMARY KEY REFERENCES anime(id) ON DELETE CASCADE,
 source_id UUID NOT NULL REFERENCES data_sources(id),
 observed_at TIMESTAMPTZ NOT NULL,
 payload JSONB NOT NULL CHECK(jsonb_typeof(payload)='object')
);
INSERT INTO catalog_base_metadata SELECT anime_id,source_id,observed_at,payload FROM catalog_metadata;
CREATE TABLE catalog_field_choices (
 anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
 field TEXT NOT NULL CHECK(field IN ('genres','studios','synonyms','themes','demographics','tags')),
 revision INT NOT NULL CHECK(revision BETWEEN 1 AND 1000000000),
 mode TEXT NOT NULL CHECK(mode IN ('SELECT','BASE')),
 source_id UUID REFERENCES data_sources(id),
 external_id TEXT,
 value JSONB,
 observed_at TIMESTAMPTZ,
 reviewed_at TIMESTAMPTZ NOT NULL,
 reference TEXT NOT NULL,
 decision_hash TEXT NOT NULL CHECK(decision_hash ~ '^[0-9a-f]{64}$'),
 PRIMARY KEY(anime_id,field),
 CHECK((mode='SELECT' AND source_id IS NOT NULL AND external_id IS NOT NULL AND value IS NOT NULL
   AND jsonb_typeof(value)='array' AND observed_at IS NOT NULL)
 OR (mode='BASE' AND source_id IS NULL AND external_id IS NULL AND value IS NULL AND observed_at IS NULL))
);
CREATE TABLE catalog_field_choice_audits (
 anime_id UUID NOT NULL,
 field TEXT NOT NULL,
 revision INT NOT NULL,
 payload JSONB NOT NULL CHECK(jsonb_typeof(payload)='object'),
 PRIMARY KEY(anime_id,field,revision),
 FOREIGN KEY(anime_id,field) REFERENCES catalog_field_choices(anime_id,field) ON DELETE CASCADE
);
-- A compatibility marker prevents old worker binaries from overwriting resolved
-- decisions or leaving the retained primary observation stale. Trusted owners may
-- repair fixtures/schema; this marker is not an authorization boundary.
CREATE FUNCTION catalog_metadata_projection_guard() RETURNS trigger LANGUAGE plpgsql AS $guard$
BEGIN
 IF current_setting('app.catalog_metadata_projection',true) IS DISTINCT FROM '1'
 AND NOT EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolname=current_user AND r.rolsuper)
 AND NOT EXISTS(SELECT 1 FROM pg_class c WHERE c.oid=TG_RELID AND pg_has_role(current_user,c.relowner,'USAGE')) THEN
  RAISE EXCEPTION 'catalog projection context required';
 END IF;
 RETURN NEW;
END $guard$;
CREATE TRIGGER catalog_metadata_projection_guard BEFORE INSERT OR UPDATE ON catalog_metadata
 FOR EACH ROW EXECUTE FUNCTION catalog_metadata_projection_guard();
