-- Defense-in-depth tenant isolation. The API must set app.user_id only from a
-- verified server-side session, inside each transaction. Never accept it from
-- request input. The application role must not own these tables or BYPASSRLS.

CREATE FUNCTION app_current_user_id() RETURNS uuid
LANGUAGE sql STABLE
SET search_path = pg_catalog, public
AS $$
  SELECT NULLIF(current_setting('app.user_id', true), '')::uuid
$$;

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;
CREATE POLICY users_self_access ON users
  USING (id = app_current_user_id())
  WITH CHECK (id = app_current_user_id());
-- Allow the login flow to find exactly the account named by a trusted
-- server-populated setting before its user ID is known.
CREATE POLICY users_login_lookup ON users FOR SELECT
  USING (lower(email) = lower(NULLIF(current_setting('app.login_email', true), '')));

ALTER TABLE auth_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY auth_sessions_self_access ON auth_sessions
  USING (user_id = app_current_user_id())
  WITH CHECK (user_id = app_current_user_id());
-- Resolve an opaque session before the server knows its owning user ID.
-- The setting must contain the SHA-256 digest of the presented random token.
CREATE POLICY auth_sessions_token_lookup ON auth_sessions FOR SELECT
  USING (token_hash = decode(NULLIF(current_setting('app.session_token_hash', true), ''), 'hex'));

ALTER TABLE user_anime ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_anime FORCE ROW LEVEL SECURITY;
CREATE POLICY user_anime_self_access ON user_anime
  USING (user_id = app_current_user_id())
  WITH CHECK (user_id = app_current_user_id());

ALTER TABLE user_ratings ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_ratings FORCE ROW LEVEL SECURITY;
CREATE POLICY user_ratings_self_access ON user_ratings
  USING (user_id = app_current_user_id())
  WITH CHECK (user_id = app_current_user_id());

ALTER TABLE watch_progress ENABLE ROW LEVEL SECURITY;
ALTER TABLE watch_progress FORCE ROW LEVEL SECURITY;
CREATE POLICY watch_progress_self_access ON watch_progress
  USING (user_id = app_current_user_id())
  WITH CHECK (user_id = app_current_user_id());

ALTER TABLE user_preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_preferences FORCE ROW LEVEL SECURITY;
CREATE POLICY user_preferences_self_access ON user_preferences
  USING (user_id = app_current_user_id())
  WITH CHECK (user_id = app_current_user_id());

ALTER TABLE recommendations ENABLE ROW LEVEL SECURITY;
ALTER TABLE recommendations FORCE ROW LEVEL SECURITY;
CREATE POLICY recommendations_self_access ON recommendations
  USING (user_id = app_current_user_id())
  WITH CHECK (user_id = app_current_user_id());

ALTER TABLE recommendation_feedback ENABLE ROW LEVEL SECURITY;
ALTER TABLE recommendation_feedback FORCE ROW LEVEL SECURITY;
CREATE POLICY recommendation_feedback_self_access ON recommendation_feedback
  USING (
    user_id = app_current_user_id()
    AND EXISTS (
      SELECT 1 FROM recommendations r
      WHERE r.id = recommendation_id AND r.user_id = app_current_user_id()
    )
  )
  WITH CHECK (
    user_id = app_current_user_id()
    AND EXISTS (
      SELECT 1 FROM recommendations r
      WHERE r.id = recommendation_id AND r.user_id = app_current_user_id()
    )
  );

ALTER TABLE user_follows ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_follows FORCE ROW LEVEL SECURITY;
CREATE POLICY user_follows_self_access ON user_follows
  USING (user_id = app_current_user_id())
  WITH CHECK (user_id = app_current_user_id());

ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE notifications FORCE ROW LEVEL SECURITY;
CREATE POLICY notifications_self_access ON notifications
  USING (user_id = app_current_user_id())
  WITH CHECK (user_id = app_current_user_id());

COMMENT ON FUNCTION app_current_user_id() IS
  'Returns transaction-scoped authenticated user context. Set app.user_id only in trusted server code after session verification.';
