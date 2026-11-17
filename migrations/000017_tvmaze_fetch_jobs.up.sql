-- Keep queue identities bounded while supporting explicit reviewed TVmaze seasons.
-- Source approval remains administrator-controlled and is rechecked by the worker.
ALTER TABLE catalog_fetch_jobs DROP CONSTRAINT catalog_fetch_jobs_external_id_check;
ALTER TABLE catalog_fetch_jobs ADD CONSTRAINT catalog_fetch_jobs_external_id_check
 CHECK (external_id ~ '^Q[1-9][0-9]{0,11}$'
 OR external_id ~ '^show-[1-9][0-9]{0,8}-season-[1-9][0-9]{0,8}$');
