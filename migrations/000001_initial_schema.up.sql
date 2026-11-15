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
  synopsis TEXT,
  media_type TEXT NOT NULL DEFAULT 'TV' CHECK (media_type IN ('TV','MOVIE','OVA','ONA','SPECIAL','MUSIC','UNKNOWN')),
  release_year INT,
  season_name TEXT,
  status TEXT NOT NULL DEFAULT 'UNKNOWN',
  episode_count INT,
  cover_image_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX anime_title_search_idx ON anime USING gin (to_tsvector('simple', canonical_title));

CREATE TABLE anime_external_ids (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  external_id TEXT NOT NULL,
  external_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(provider, external_id),
  UNIQUE(anime_id, provider)
);

CREATE TABLE data_sources (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider_key TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  policy_url TEXT,
  enabled BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE source_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id UUID NOT NULL REFERENCES data_sources(id),
  external_id TEXT NOT NULL,
  record_type TEXT NOT NULL,
  payload JSONB NOT NULL,
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  content_hash TEXT NOT NULL,
  UNIQUE(source_id, external_id, content_hash)
);
CREATE INDEX source_records_payload_idx ON source_records USING gin(payload);

CREATE TABLE anime_features (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  feature_key TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  category TEXT NOT NULL,
  description TEXT,
  spoiler_sensitive BOOLEAN NOT NULL DEFAULT false,
  active BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE anime_feature_observations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  feature_id UUID NOT NULL REFERENCES anime_features(id),
  value NUMERIC(5,4) NOT NULL CHECK (value >= 0 AND value <= 1),
  confidence NUMERIC(5,4) NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  source_kind TEXT NOT NULL CHECK (source_kind IN ('PROVIDER','CURATED','MODEL','COMMUNITY')),
  source_id UUID REFERENCES data_sources(id),
  evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
  scope TEXT NOT NULL DEFAULT 'SERIES',
  spoiler_level TEXT NOT NULL DEFAULT 'NONE',
  observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  model_version TEXT
);
CREATE INDEX anime_feature_obs_lookup_idx ON anime_feature_observations(anime_id, feature_id, observed_at DESC);

CREATE TABLE anime_relations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  target_anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  relation_type TEXT NOT NULL,
  source_id UUID REFERENCES data_sources(id),
  confidence NUMERIC(5,4) NOT NULL DEFAULT 1,
  CHECK (source_anime_id <> target_anime_id),
  UNIQUE(source_anime_id, target_anime_id, relation_type)
);

CREATE TABLE user_anime (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  watch_status TEXT NOT NULL CHECK (watch_status IN ('PLANNING','WATCHING','COMPLETED','ON_HOLD','DROPPED')),
  started_at DATE,
  completed_at DATE,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, anime_id)
);

CREATE TABLE user_ratings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  scope TEXT NOT NULL DEFAULT 'SERIES' CHECK (scope IN ('SERIES','SEASON')),
  season_key TEXT,
  score SMALLINT NOT NULL CHECK (score BETWEEN 1 AND 5),
  rating_label TEXT,
  rated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, anime_id, scope, season_key)
);

CREATE TABLE watch_progress (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  episodes_watched INT NOT NULL DEFAULT 0 CHECK (episodes_watched >= 0),
  total_episodes INT CHECK (total_episodes IS NULL OR total_episodes >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, anime_id)
);

CREATE TABLE user_preferences (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  feature_id UUID NOT NULL REFERENCES anime_features(id),
  preference_weight NUMERIC(6,4) NOT NULL CHECK (preference_weight BETWEEN -1 AND 1),
  origin TEXT NOT NULL CHECK (origin IN ('DECLARED','INFERRED','CONFIRMED')),
  confidence NUMERIC(5,4) NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, feature_id, origin)
);

CREATE TABLE recommendations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  category TEXT NOT NULL CHECK (category IN ('PERSONAL_MATCH','GLOBAL_ESSENTIAL','DISCOVERY')),
  score NUMERIC(8,5) NOT NULL,
  explanation JSONB NOT NULL DEFAULT '{}'::jsonb,
  algorithm_version TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE recommendation_feedback (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  recommendation_id UUID NOT NULL REFERENCES recommendations(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  feedback TEXT NOT NULL CHECK (feedback IN ('INTERESTED','NOT_INTERESTED','ALREADY_WATCHED','STARTED','COMPLETED')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE airing_schedule (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  episode_number INT NOT NULL,
  release_track TEXT NOT NULL CHECK (release_track IN ('RAW','SUB','DUB','UNKNOWN')),
  scheduled_at TIMESTAMPTZ,
  actual_released_at TIMESTAMPTZ,
  schedule_status TEXT NOT NULL DEFAULT 'TENTATIVE',
  source_id UUID REFERENCES data_sources(id),
  last_verified_at TIMESTAMPTZ,
  revision INT NOT NULL DEFAULT 1,
  UNIQUE(anime_id, episode_number, release_track, revision)
);

CREATE TABLE domain_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_key TEXT NOT NULL UNIQUE,
  event_type TEXT NOT NULL,
  aggregate_type TEXT NOT NULL,
  aggregate_id UUID NOT NULL,
  payload JSONB NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at TIMESTAMPTZ
);

CREATE TABLE user_follows (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  follow_scope TEXT NOT NULL DEFAULT 'ANIME' CHECK (follow_scope IN ('ANIME','FRANCHISE')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, anime_id, follow_scope)
);

CREATE TABLE notifications (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  event_id UUID REFERENCES domain_events(id) ON DELETE SET NULL,
  channel TEXT NOT NULL DEFAULT 'IN_APP',
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  read_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
