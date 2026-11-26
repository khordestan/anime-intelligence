CREATE TABLE discovery_title_feedback (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  direction SMALLINT NOT NULL CHECK (direction IN (-1, 1)),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, anime_id)
);

CREATE INDEX discovery_title_feedback_recent ON discovery_title_feedback (user_id, updated_at DESC, anime_id);

ALTER TABLE discovery_title_feedback ENABLE ROW LEVEL SECURITY;
ALTER TABLE discovery_title_feedback FORCE ROW LEVEL SECURITY;

CREATE POLICY discovery_title_feedback_owner ON discovery_title_feedback
  USING (user_id = app_current_user_id())
  WITH CHECK (user_id = app_current_user_id());
