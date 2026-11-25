CREATE TABLE release_email_jobs (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  event_id UUID REFERENCES domain_events(id) ON DELETE SET NULL,
  attempts SMALLINT NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
  lease UUID
);
DO $$
BEGIN
  PERFORM set_config('row_security','off',true);
  INSERT INTO release_email_jobs(user_id)
   SELECT id FROM users WHERE release_email_since IS NOT NULL;
  PERFORM set_config('row_security','on',true);
END
$$;
CREATE INDEX release_email_jobs_due ON release_email_jobs(available_at,user_id);
ALTER TABLE release_email_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE release_email_jobs FORCE ROW LEVEL SECURITY;
CREATE POLICY release_email_own_jobs ON release_email_jobs
 USING (user_id=app_current_user_id()) WITH CHECK (user_id=app_current_user_id());
-- The worker can discover due IDs, then returns to owner-scoped access.
CREATE POLICY release_email_discover_jobs ON release_email_jobs FOR SELECT
 USING (current_setting('app.release_email_worker',true)='on');
