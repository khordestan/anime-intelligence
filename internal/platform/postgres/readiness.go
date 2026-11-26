package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotReady = errors.New("database runtime is not ready")

// Ready checks the live connection role and required security schema without
// reading account data or modifying identity/session state. Errors are fixed.
func (d *DB) Ready(ctx context.Context) error {
	if d == nil || d.pool == nil {
		return ErrNotReady
	}
	if err := checkRuntime(ctx, d.pool); err != nil {
		return ErrNotReady
	}
	return nil
}
func checkRuntime(ctx context.Context, pool *pgxpool.Pool) error {
	connection, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer connection.Release()
	c := connection.Conn()
	if err = validateRuntimeRole(ctx, c); err != nil {
		return err
	}
	var secured int
	err = c.QueryRow(ctx, `SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname='public' AND c.relname IN ('users','auth_sessions','user_anime','watch_progress','user_ratings','user_preferences','recommendations','recommendation_feedback','user_follows','notifications','personal_match_dismissals','auth_challenges','identity_outbox','identity_requests','watch_cycles','catalog_analysis_owner_budget','release_email_jobs','discovery_title_feedback')
 AND c.relrowsecurity AND c.relforcerowsecurity`).Scan(&secured)
	if err != nil || secured != 18 {
		return errors.New("required forced row-level security is missing; apply schema migrations")
	}
	if _, err = c.Exec(ctx, `SELECT login_failure_started_at,release_email_since FROM users LIMIT 0; SELECT user_id,available_at,event_id,attempts,lease FROM release_email_jobs LIMIT 0`); err != nil {
		return errors.New("required login security schema is missing; apply schema migrations")
	}
	if _, err = c.Exec(ctx, `SELECT anime_id,provider,external_id FROM anime_external_ids LIMIT 0; SELECT anime_id,source_id,observed_at,payload FROM catalog_metadata LIMIT 0; SELECT anime_id,field,revision,mode,source_id,observed_at FROM catalog_field_choices LIMIT 0; SELECT id,provider_key FROM data_sources LIMIT 0; SELECT source_anime_id,target_anime_id,relation_type FROM anime_relations LIMIT 0; SELECT episode_number,release_track,scheduled_at,actual_released_at,last_verified_at,revision FROM airing_schedule LIMIT 0`); err != nil {
		return errors.New("required catalog schema or read grants are missing; apply migrations and runtime grants")
	}
	if _, err = c.Exec(ctx, `SELECT watch_cycle FROM user_anime LIMIT 0; SELECT cycle_number,episodes_watched,total_episodes,watched_episode_numbers,rating,archived_at FROM watch_cycles LIMIT 0; SELECT watched_episode_numbers FROM watch_progress LIMIT 0; SELECT release_track,schedule_notices_since FROM user_follows LIMIT 0; SELECT id,event_key,event_type,aggregate_type,aggregate_id,payload,occurred_at FROM domain_events LIMIT 0`); err != nil {
		return errors.New("required release-alert schema or grants are missing; apply migrations and runtime grants")
	}
	if _, err = c.Exec(ctx, `SELECT user_id,anime_id FROM personal_match_dismissals LIMIT 0; SELECT user_id,anime_id,direction,updated_at FROM discovery_title_feedback LIMIT 0; SELECT feature_key FROM anime_features LIMIT 0; SELECT anime_id,feature_id,value,confidence,source_kind,evidence,scope,spoiler_level,observed_at FROM anime_feature_observations LIMIT 0`); err != nil {
		return errors.New("required recommendation evidence grants are missing; reapply runtime grants")
	}
	if _, err = c.Exec(ctx, `SELECT user_id,window_start,requests FROM catalog_analysis_owner_budget LIMIT 0; SELECT singleton,window_start,requests FROM catalog_analysis_global_budget LIMIT 0`); err != nil {
		return errors.New("required analyst quota schema or grants are missing; apply migrations and runtime grants")
	}
	if _, err = c.Exec(ctx, `SELECT outcome FROM identity_requests LIMIT 0; SELECT outcome FROM identity_outbox LIMIT 0`); err != nil {
		return errors.New("required identity queue outcome schema is missing; apply schema migrations")
	}
	return nil
}
