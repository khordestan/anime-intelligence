DROP TABLE IF EXISTS auth_sessions;
DROP INDEX IF EXISTS users_email_lower_unique;
ALTER TABLE users
  DROP CONSTRAINT IF EXISTS users_password_hash_length_check,
  DROP COLUMN IF EXISTS last_login_at,
  DROP COLUMN IF EXISTS locked_until,
  DROP COLUMN IF EXISTS failed_login_count,
  DROP COLUMN IF EXISTS disabled_at,
  DROP COLUMN IF EXISTS email_verified_at,
  DROP COLUMN IF EXISTS password_hash;
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
