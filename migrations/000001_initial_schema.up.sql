CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT UNIQUE,
  display_name TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE anime (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  canonical_title TEXT NOT NULL,
  native_title TEXT,
  media_type TEXT NOT NULL CHECK (media_type IN ('TV','MOVIE','OVA','ONA','SPECIAL')),
  release_year INTEGER,
  release_status TEXT,
  episode_count INTEGER,
  synopsis TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX anime_title_search_idx ON anime USING gin (to_tsvector('simple', canonical_title));

CREATE TABLE anime_external_ids (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  source_key TEXT NOT NULL,
  external_id TEXT NOT NULL,
  external_url TEXT,
  UNIQUE(source_key, external_id)
);

CREATE TABLE anime_features (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  feature_key TEXT NOT NULL UNIQUE,
  label TEXT NOT NULL,
  category TEXT NOT NULL,
  description TEXT,
  spoiler_sensitive BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE anime_feature_observations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  feature_id UUID NOT NULL REFERENCES anime_features(id) ON DELETE CASCADE,
  value JSONB NOT NULL,
  source_key TEXT NOT NULL,
  confidence NUMERIC(4,3),
  observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(anime_id, feature_id, source_key)
);

CREATE TABLE user_anime (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  watch_status TEXT NOT NULL CHECK (watch_status IN ('PLANNING','WATCHING','COMPLETED','ON_HOLD','DROPPED')),
  episodes_watched INTEGER NOT NULL DEFAULT 0 CHECK (episodes_watched >= 0),
  started_at DATE,
  completed_at DATE,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, anime_id)
);

CREATE TABLE user_ratings (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  score SMALLINT NOT NULL CHECK (score BETWEEN 1 AND 10),
  rated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, anime_id)
);

CREATE TABLE user_preferences (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  explicit_preferences JSONB NOT NULL DEFAULT '{}'::jsonb,
  inferred_preferences JSONB NOT NULL DEFAULT '{}'::jsonb,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE anime_relations (
  from_anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  to_anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  relation_type TEXT NOT NULL,
  PRIMARY KEY(from_anime_id, to_anime_id, relation_type),
  CHECK(from_anime_id <> to_anime_id)
);

CREATE TABLE airing_schedule (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  episode_number INTEGER,
  release_track TEXT NOT NULL DEFAULT 'SUB',
  scheduled_at TIMESTAMPTZ,
  actual_released_at TIMESTAMPTZ,
  schedule_status TEXT NOT NULL DEFAULT 'SCHEDULED',
  source_key TEXT NOT NULL,
  last_verified_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notifications (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  read_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_unread_idx ON notifications(user_id, created_at DESC) WHERE read_at IS NULL;
