ALTER TABLE user_follows ADD COLUMN release_track TEXT NOT NULL DEFAULT 'SUB'
  CHECK (release_track IN ('RAW','SUB','DUB','UNKNOWN','ALL'));
-- Deduplication applies to event-backed in-app alerts; legacy/manual notices
-- without an event retain their existing behavior.
CREATE UNIQUE INDEX notifications_event_channel_idx
  ON notifications(user_id,event_id,channel) WHERE event_id IS NOT NULL;
CREATE INDEX domain_events_release_lookup_idx
  ON domain_events(aggregate_id,occurred_at,id) WHERE event_type='EPISODE_RELEASED';
