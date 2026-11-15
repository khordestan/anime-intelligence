-- Encrypted delivery envelopes; the encryption key must never be stored here.
ALTER TABLE auth_challenges ADD CONSTRAINT auth_challenges_id_user_key UNIQUE(id,user_id);
CREATE TABLE identity_outbox (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 challenge_id UUID NOT NULL UNIQUE,
 payload BYTEA,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 expires_at TIMESTAMPTZ NOT NULL,
 available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),
 lease_hash BYTEA CHECK(lease_hash IS NULL OR octet_length(lease_hash)=32),
 lease_until TIMESTAMPTZ,
 completed_at TIMESTAMPTZ,
 FOREIGN KEY(challenge_id,user_id) REFERENCES auth_challenges(id,user_id) ON DELETE CASCADE,
 CHECK(payload IS NULL OR octet_length(payload) BETWEEN 29 AND 4096),
 CHECK((lease_hash IS NULL)=(lease_until IS NULL)),
 CHECK(completed_at IS NULL OR payload IS NULL)
);
CREATE INDEX identity_outbox_pending_idx ON identity_outbox(available_at,created_at) WHERE completed_at IS NULL;
ALTER TABLE identity_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity_outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY identity_outbox_self_access ON identity_outbox
 USING(user_id=app_current_user_id()) WITH CHECK(user_id=app_current_user_id());
-- This setting is trusted application context, just like app.user_id. It is
-- used only by delivery code; it grants no access to users or challenge rows.
CREATE POLICY identity_outbox_worker_access ON identity_outbox
 USING(current_setting('app.delivery_worker',true)='on')
 WITH CHECK(current_setting('app.delivery_worker',true)='on');
