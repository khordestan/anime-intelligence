-- Anonymous encrypted intent: accepting recovery must not look up an account.
-- Only trusted queue code may access it. No raw bearer credentials are stored.
CREATE TABLE identity_requests (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 payload BYTEA,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 expires_at TIMESTAMPTZ NOT NULL DEFAULT now()+interval '15 minutes',
 available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),
 lease_hash BYTEA CHECK(lease_hash IS NULL OR octet_length(lease_hash)=32),
 lease_until TIMESTAMPTZ,
 completed_at TIMESTAMPTZ,
 CHECK(payload IS NULL OR octet_length(payload)=540),
 CHECK((lease_hash IS NULL)=(lease_until IS NULL)),
 CHECK(completed_at IS NULL OR payload IS NULL)
);
CREATE INDEX identity_requests_pending_idx ON identity_requests(available_at,created_at) WHERE completed_at IS NULL;
CREATE INDEX identity_requests_completed_idx ON identity_requests(completed_at) WHERE completed_at IS NOT NULL;
ALTER TABLE identity_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY identity_requests_worker_access ON identity_requests
 USING(current_setting('app.identity_request_worker',true)='on')
 WITH CHECK(current_setting('app.identity_request_worker',true)='on');
