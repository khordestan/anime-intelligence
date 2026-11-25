DO $$
BEGIN
  PERFORM set_config('row_security', 'off', true);
  IF EXISTS (SELECT 1 FROM users WHERE release_email_since IS NOT NULL) THEN
    RAISE EXCEPTION 'cannot remove retained release email consent';
  END IF;
END
$$;
ALTER TABLE users DROP COLUMN release_email_since;
