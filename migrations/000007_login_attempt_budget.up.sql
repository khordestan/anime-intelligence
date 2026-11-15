-- Shared login-failure observation window; no IP address or client identifier
-- is stored. Existing nonzero counters start a fresh window on their next failure.
ALTER TABLE users ADD COLUMN login_failure_started_at TIMESTAMPTZ;
COMMENT ON COLUMN users.login_failure_started_at IS
  'Start of the current persisted failed-login observation window.';
