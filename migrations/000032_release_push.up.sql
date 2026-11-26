CREATE TABLE release_push_subscriptions (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  id UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
  endpoint_digest BYTEA UNIQUE NOT NULL CHECK (length(endpoint_digest)=32),
  subscription_digest BYTEA NOT NULL CHECK (length(subscription_digest)=32),
  payload BYTEA NOT NULL CHECK (length(payload) BETWEEN 28 AND 4096),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  event_id UUID REFERENCES domain_events(id) ON DELETE SET NULL,
  attempts SMALLINT NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
  lease UUID
);
CREATE INDEX release_push_subscriptions_due ON release_push_subscriptions(available_at,user_id);
ALTER TABLE release_push_subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE release_push_subscriptions FORCE ROW LEVEL SECURITY;
CREATE POLICY release_push_owner ON release_push_subscriptions
  USING (user_id=app_current_user_id()) WITH CHECK (user_id=app_current_user_id());
CREATE POLICY release_push_discover_due ON release_push_subscriptions FOR SELECT
  USING (current_setting('app.release_push_worker',true)='on' AND available_at<=now());
