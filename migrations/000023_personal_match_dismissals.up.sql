CREATE TABLE personal_match_dismissals (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  anime_id UUID NOT NULL REFERENCES anime(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, anime_id)
);

ALTER TABLE personal_match_dismissals ENABLE ROW LEVEL SECURITY;
ALTER TABLE personal_match_dismissals FORCE ROW LEVEL SECURITY;

CREATE POLICY personal_match_dismissals_owner ON personal_match_dismissals
  USING (user_id = app_current_user_id())
  WITH CHECK (user_id = app_current_user_id());
