ALTER TABLE user_follows
  ADD COLUMN schedule_notices_since TIMESTAMPTZ,
  ADD CONSTRAINT user_follows_schedule_notices_start
    CHECK (schedule_notices_since IS NULL OR schedule_notices_since >= created_at);
