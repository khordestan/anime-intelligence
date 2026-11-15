-- Public catalog ingestion is administrator-only. No API runtime write grants.
CREATE TABLE catalog_sync_state (
  source_id UUID PRIMARY KEY REFERENCES data_sources(id),
  observed_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE catalog_metadata (
  anime_id UUID PRIMARY KEY REFERENCES anime(id) ON DELETE CASCADE,
  source_id UUID NOT NULL REFERENCES data_sources(id),
  observed_at TIMESTAMPTZ NOT NULL,
  payload JSONB NOT NULL CHECK (jsonb_typeof(payload) = 'object')
);
CREATE INDEX catalog_metadata_source_idx ON catalog_metadata(source_id);
-- Schedule revisions are unique already; all importer writers use one advisory lock.
CREATE INDEX airing_schedule_latest_idx
  ON airing_schedule(anime_id,episode_number,release_track,revision DESC);
