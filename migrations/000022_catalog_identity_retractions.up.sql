-- Permanent reviewed retirement of an unused provider identity, never a merge.
CREATE TABLE catalog_identity_retractions (
 provider TEXT NOT NULL REFERENCES data_sources(provider_key),
 external_id TEXT NOT NULL CHECK(length(external_id) BETWEEN 1 AND 128 AND external_id ~ '^[A-Za-z0-9_.-]+$'),
 anime_id UUID NOT NULL REFERENCES anime(id),
 mapping_id UUID NOT NULL,
 reviewed_at TIMESTAMPTZ NOT NULL,
 reference TEXT NOT NULL CHECK(length(reference) BETWEEN 1 AND 2048),
 decision_hash TEXT NOT NULL CHECK(decision_hash ~ '^[0-9a-f]{64}$'),
 PRIMARY KEY(provider,external_id)
);
-- All supported identity insertions, including older import/map binaries, must
-- observe retirements. Invoker rights require explicit tombstone read grants.
CREATE FUNCTION catalog_identity_retirement_guard() RETURNS trigger LANGUAGE plpgsql AS $guard$
BEGIN
 PERFORM pg_advisory_xact_lock(742019684);
 IF EXISTS(SELECT 1 FROM catalog_identity_retractions r WHERE r.provider=NEW.provider AND r.external_id=NEW.external_id) THEN
  RAISE EXCEPTION 'catalog identity is retired' USING ERRCODE='23514',CONSTRAINT='catalog_identity_retired';
 END IF;
 RETURN NEW;
END $guard$;
CREATE TRIGGER catalog_identity_retirement_guard BEFORE INSERT OR UPDATE OF provider,external_id,anime_id ON anime_external_ids
 FOR EACH ROW EXECUTE FUNCTION catalog_identity_retirement_guard();
