DO $rollback$
BEGIN
  LOCK TABLE discovery_title_feedback IN ACCESS EXCLUSIVE MODE;
  PERFORM set_config('row_security', 'off', true);
  IF EXISTS (SELECT 1 FROM discovery_title_feedback) THEN
    RAISE EXCEPTION 'refusing to drop retained Discovery title feedback';
  END IF;
  DROP TABLE discovery_title_feedback;
END
$rollback$;
