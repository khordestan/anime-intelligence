-- Provider-neutral starter fixtures. No third-party descriptions or images are copied.
CREATE UNIQUE INDEX IF NOT EXISTS user_ratings_series_unique
  ON user_ratings(user_id, anime_id) WHERE scope = 'SERIES';
CREATE UNIQUE INDEX IF NOT EXISTS user_ratings_season_unique
  ON user_ratings(user_id, anime_id, season_key) WHERE scope = 'SEASON' AND season_key IS NOT NULL;

INSERT INTO anime (id, canonical_title, native_title, media_type, release_year, status, episode_count)
VALUES
('10000000-0000-4000-8000-000000000001','Solo Leveling','나 혼자만 레벨업','TV',2024,'FINISHED',12),
('10000000-0000-4000-8000-000000000002','Attack on Titan','進撃の巨人','TV',2013,'FINISHED',25),
('10000000-0000-4000-8000-000000000003','Death Note','デスノート','TV',2006,'FINISHED',37),
('10000000-0000-4000-8000-000000000004','Fullmetal Alchemist: Brotherhood','鋼の錬金術師 FULLMETAL ALCHEMIST','TV',2009,'FINISHED',64),
('10000000-0000-4000-8000-000000000005','Hunter x Hunter (2011)','HUNTER×HUNTER','TV',2011,'FINISHED',148),
('10000000-0000-4000-8000-000000000006','One Punch Man','ワンパンマン','TV',2015,'RELEASING',12),
('10000000-0000-4000-8000-000000000007','Steins;Gate','シュタインズ・ゲート','TV',2011,'FINISHED',24),
('10000000-0000-4000-8000-000000000008','Overlord','オーバーロード','TV',2015,'FINISHED',13)
ON CONFLICT (id) DO NOTHING;
