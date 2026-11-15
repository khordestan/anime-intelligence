ALTER TABLE watch_progress ADD COLUMN watched_episode_numbers INTEGER[];
CREATE FUNCTION app_valid_episode_numbers(INTEGER[],INTEGER) RETURNS BOOLEAN
LANGUAGE SQL IMMUTABLE PARALLEL SAFE AS $$
SELECT $1 IS NULL OR (
  cardinality($1)=$2 AND cardinality($1)<=10000
  AND COALESCE(array_ndims($1),1)=1 AND COALESCE(array_lower($1,1),1)=1
  AND (SELECT count(*)=count(DISTINCT n) AND COALESCE(bool_and(n BETWEEN 1 AND 100000),true) FROM unnest($1) n)
);
$$;
ALTER TABLE watch_progress ADD CONSTRAINT watch_progress_exact_numbers_check
  CHECK (app_valid_episode_numbers(watched_episode_numbers,episodes_watched));
