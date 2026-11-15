-- Run as the migration administrator after cmd/migrate. Provision the login
-- password out of band (for example, psql's interactive \password command).
-- Never put a real password or connection URL in this public repository.
DO $role$ BEGIN
CREATE ROLE anime_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
  NOINHERIT NOREPLICATION NOBYPASSRLS;
EXCEPTION WHEN duplicate_object THEN NULL;
END $role$;
GRANT USAGE ON SCHEMA public TO anime_runtime;
GRANT SELECT ON anime TO anime_runtime;
GRANT SELECT, INSERT, UPDATE ON users, auth_sessions, user_anime,
  watch_progress, user_ratings TO anime_runtime;

-- Export reads the remaining personal-data resources. Account deletion is
-- scoped by forced RLS; dependent records are removed by foreign-key cascades.
GRANT SELECT ON user_preferences, recommendations, recommendation_feedback,
  user_follows, notifications TO anime_runtime;
GRANT DELETE ON users TO anime_runtime;

GRANT SELECT, INSERT, UPDATE ON auth_challenges TO anime_runtime;

GRANT SELECT, INSERT, UPDATE, DELETE ON identity_outbox TO anime_runtime;
