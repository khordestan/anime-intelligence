DO $$
BEGIN
  PERFORM set_config('row_security', 'off', true);
  IF EXISTS (SELECT 1 FROM user_follows WHERE schedule_notices_since IS NOT NULL) THEN
    RAISE EXCEPTION 'cannot remove retained schedule-notice consent';
  END IF;
END
$$;

ALTER TABLE user_follows DROP COLUMN schedule_notices_since;
