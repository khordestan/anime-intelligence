-- Manual reviewed-feature import login. Provision its password out of band.
-- It cannot update/delete observations, alter taxonomy or access private tables.
DO $role$ BEGIN
CREATE ROLE anime_curator LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
  NOINHERIT NOREPLICATION NOBYPASSRLS;
EXCEPTION WHEN duplicate_object THEN NULL;
END $role$;
GRANT USAGE ON SCHEMA public TO anime_curator;
GRANT SELECT ON anime,anime_features,anime_feature_observations TO anime_curator;
GRANT INSERT ON anime_feature_observations TO anime_curator;
