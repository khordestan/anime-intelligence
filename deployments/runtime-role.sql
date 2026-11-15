-- Run as the migration administrator after cmd/migrate. Provision the login
-- password out of band (for example, psql's interactive \password command).
-- Never put a real password or connection URL in this public repository.
CREATE ROLE anime_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
  NOINHERIT NOREPLICATION NOBYPASSRLS;
GRANT USAGE ON SCHEMA public TO anime_runtime;
GRANT SELECT ON anime TO anime_runtime;
GRANT SELECT, INSERT, UPDATE ON users, auth_sessions, user_anime,
  watch_progress, user_ratings TO anime_runtime;
