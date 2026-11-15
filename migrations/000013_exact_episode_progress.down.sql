ALTER TABLE watch_progress DROP CONSTRAINT IF EXISTS watch_progress_exact_numbers_check;
ALTER TABLE watch_progress DROP COLUMN IF EXISTS watched_episode_numbers;
DROP FUNCTION IF EXISTS app_valid_episode_numbers(INTEGER[],INTEGER);
