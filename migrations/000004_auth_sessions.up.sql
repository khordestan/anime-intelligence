-- Authentication persistence primitives. Password hashing and session issuance
-- remain application responsibilities; raw session tokens must never be stored.
ALTER TABLE users
  ALTER COLUMN email SET NOT NULL,
  ADD COLUMN password_hash TEXT,
  ADD COLUMN email_verified_at TIMESTAMPTZ,
  ADD COLUMN disabled_at TIMESTAMPTZ,
  ADD COLUMN failed_login_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_login_count >= 0),
  ADD COLUMN locked_until TIMESTAMPTZ,
  ADD COLUMN last_login_at TIMESTAMPTZ,
  ADD CONSTRAINT users_password_hash_length_check
    CHECK (password_hash IS NULL OR length(password_hash) BETWEEN 20 AND 512);

CREATE UNIQUE INDEX users_email_lower_unique ON users (lower(email));

CREATE TABLE auth_sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  last_used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at TIMESTAMPTZ,
  CHECK (expires_at > created_at)
);
CREATE INDEX auth_sessions_user_active_idx
  ON auth_sessions(user_id, expires_at)
  WHERE revoked_at IS NULL;
CREATE INDEX auth_sessions_expiry_idx ON auth_sessions(expires_at);

COMMENT ON COLUMN users.password_hash IS
  'Encoded password hash only (e.g. Argon2id PHC string); never store plaintext passwords.';
COMMENT ON TABLE auth_sessions IS
  'Opaque session credentials stored as SHA-256 token hashes; raw bearer tokens are returned once and never persisted.';
COMMENT ON COLUMN auth_sessions.token_hash IS
  'SHA-256 digest of a cryptographically random session token; never the raw token.';
