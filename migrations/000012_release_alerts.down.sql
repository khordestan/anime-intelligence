DROP INDEX IF EXISTS domain_events_release_lookup_idx;
DROP INDEX IF EXISTS notifications_event_channel_idx;
ALTER TABLE user_follows DROP COLUMN IF EXISTS release_track;
