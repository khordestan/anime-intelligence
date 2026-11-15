-- Only purpose-bound token digests are persisted. Raw credentials exist solely
-- at the trusted delivery boundary and in the user's explicit POST request.
CREATE TABLE auth_challenges (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 purpose TEXT NOT NULL CHECK (purpose IN ('PASSWORD_RESET','EMAIL_VERIFICATION')),
 email TEXT NOT NULL CHECK (length(email) BETWEEN 1 AND 254),
 token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 expires_at TIMESTAMPTZ NOT NULL,
 consumed_at TIMESTAMPTZ,
 revoked_at TIMESTAMPTZ,
 CHECK (expires_at>created_at)
);
CREATE INDEX auth_challenges_user_purpose_idx ON auth_challenges(user_id,purpose,created_at DESC);
CREATE UNIQUE INDEX auth_challenges_one_pending_idx ON auth_challenges(user_id,purpose)
 WHERE consumed_at IS NULL AND revoked_at IS NULL;
ALTER TABLE auth_challenges ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth_challenges FORCE ROW LEVEL SECURITY;
CREATE POLICY auth_challenges_self_access ON auth_challenges
 USING (user_id=app_current_user_id()) WITH CHECK (user_id=app_current_user_id());
CREATE POLICY auth_challenges_digest_lookup ON auth_challenges FOR SELECT
 USING (
  purpose=NULLIF(current_setting('app.challenge_purpose',true),'') AND
  token_hash=decode(NULLIF(current_setting('app.challenge_token_hash',true),''),'hex')
 );
COMMENT ON TABLE auth_challenges IS 'Single-use identity challenges; never persist raw tokens.';
