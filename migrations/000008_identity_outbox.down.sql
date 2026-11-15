DROP TABLE IF EXISTS identity_outbox;
ALTER TABLE auth_challenges DROP CONSTRAINT IF EXISTS auth_challenges_id_user_key;
