CREATE TABLE catalog_analysis_owner_budget (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  window_start TIMESTAMPTZ NOT NULL,
  requests SMALLINT NOT NULL CHECK (requests BETWEEN 1 AND 3)
);
ALTER TABLE catalog_analysis_owner_budget ENABLE ROW LEVEL SECURITY;
ALTER TABLE catalog_analysis_owner_budget FORCE ROW LEVEL SECURITY;
CREATE POLICY catalog_analysis_owner_scope ON catalog_analysis_owner_budget
  USING (user_id = app_current_user_id())
  WITH CHECK (user_id = app_current_user_id());

CREATE TABLE catalog_analysis_global_budget (
  singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
  window_start TIMESTAMPTZ NOT NULL,
  requests SMALLINT NOT NULL CHECK (requests BETWEEN 0 AND 12)
);
INSERT INTO catalog_analysis_global_budget (window_start, requests) VALUES ('-infinity', 0);
