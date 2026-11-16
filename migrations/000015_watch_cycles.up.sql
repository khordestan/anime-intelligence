ALTER TABLE user_anime ADD COLUMN watch_cycle INTEGER NOT NULL DEFAULT 1 CHECK (watch_cycle BETWEEN 1 AND 1000);
CREATE TABLE watch_cycles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  cycle_number INTEGER NOT NULL CHECK (cycle_number BETWEEN 1 AND 999),
  episodes_watched INTEGER NOT NULL CHECK (episodes_watched >= 0),
  total_episodes INTEGER CHECK (total_episodes IS NULL OR total_episodes >= 0),
  watched_episode_numbers INTEGER[],
  rating SMALLINT CHECK (rating BETWEEN 1 AND 5),
  archived_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id,anime_id,cycle_number),
  FOREIGN KEY(user_id,anime_id) REFERENCES user_anime(user_id,anime_id) ON DELETE CASCADE,
  CHECK (app_valid_episode_numbers(watched_episode_numbers,episodes_watched))
);
ALTER TABLE watch_cycles ENABLE ROW LEVEL SECURITY;
ALTER TABLE watch_cycles FORCE ROW LEVEL SECURITY;
CREATE POLICY watch_cycles_self_access ON watch_cycles
  USING (user_id=app_current_user_id()) WITH CHECK (user_id=app_current_user_id());
