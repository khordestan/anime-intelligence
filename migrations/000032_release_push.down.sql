DO $rollback$
BEGIN
  PERFORM set_config('row_security','off',true);
  IF EXISTS (SELECT 1 FROM release_push_subscriptions) OR EXISTS (SELECT 1 FROM notifications WHERE channel='PUSH') THEN
    RAISE EXCEPTION 'refusing to drop retained browser push subscriptions or history';
  END IF;
  PERFORM set_config('row_security','on',true);
END
$rollback$;
DROP TABLE release_push_subscriptions;
