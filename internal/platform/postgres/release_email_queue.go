package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ReleaseEmailClaim holds an in-memory SMTP recipient. The durable job stores
// only canonical references, lease metadata and attempt counts.
type ReleaseEmailClaim struct {
	owner, event, lease string
	attempts            int
	Recipient           string `json:"-"`
}

func (ReleaseEmailClaim) String() string { return "release email claim (redacted)" }

func (d *DB) ClaimReleaseEmail(ctx context.Context) (*ReleaseEmailClaim, error) {
	tx, err := d.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.release_email_worker','on',true)`); err != nil {
		return nil, err
	}
	var owner string
	err = tx.QueryRow(ctx, `SELECT user_id::text FROM release_email_jobs
		WHERE available_at<=now() ORDER BY available_at,user_id LIMIT 1`).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT set_config('app.release_email_worker','off',true)`); err != nil {
		return nil, err
	}
	if err := identity(ctx, tx, owner); err != nil {
		return nil, err
	}
	claim := &ReleaseEmailClaim{owner: owner}
	var eligible bool
	err = tx.QueryRow(ctx, `SELECT email, release_email_since IS NOT NULL AND email_verified_at IS NOT NULL
		AND disabled_at IS NULL AND (locked_until IS NULL OR locked_until<=now())
		FROM users WHERE id=$1 FOR SHARE`, owner).Scan(&claim.Recipient, &eligible)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,742019685))`, owner); err != nil {
		return nil, err
	}
	var pending *string
	err = tx.QueryRow(ctx, `SELECT event_id::text,attempts FROM release_email_jobs
		WHERE user_id=$1 AND available_at<=now() FOR UPDATE`, owner).Scan(&pending, &claim.attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if eligible {
		err = tx.QueryRow(ctx, releaseTargetsSQL+`
			SELECT candidate.id::text FROM (
				SELECT DISTINCT e.id,e.occurred_at
				FROM domain_events e JOIN targets f ON f.anime_id=e.aggregate_id
				JOIN users u ON u.id=$1
				WHERE e.aggregate_type='ANIME' AND e.occurred_at>=f.created_at
				  AND e.occurred_at>=u.release_email_since
				  AND e.occurred_at>=now()-interval '24 hours' AND e.occurred_at<=now()
				  AND ((e.event_type='EPISODE_RELEASED' AND (f.release_track='ALL' OR f.release_track=e.payload->>'track'))
				    OR (e.event_type='SEASON_ANNOUNCED' AND f.scope='FRANCHISE'))
				  AND NOT EXISTS(SELECT 1 FROM notifications n
				    WHERE n.user_id=$1 AND n.event_id=e.id AND n.channel='EMAIL')
			) candidate
			ORDER BY (candidate.id=$2::uuid) DESC NULLS LAST,candidate.occurred_at,candidate.id LIMIT 1`, owner, pending).Scan(&claim.event)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	if claim.event == "" {
		_, err = tx.Exec(ctx, `UPDATE release_email_jobs SET available_at=now()+interval '5 minutes',
			event_id=NULL,attempts=0,lease=NULL WHERE user_id=$1`, owner)
		if err != nil {
			return nil, err
		}
		return nil, tx.Commit(ctx)
	}
	if pending == nil || *pending != claim.event {
		claim.attempts = 0
	}
	if claim.attempts == 5 {
		return nil, finishReleaseEmail(ctx, tx, claim, false)
	}
	claim.attempts++
	err = tx.QueryRow(ctx, `UPDATE release_email_jobs SET event_id=$2,attempts=$3,
		lease=gen_random_uuid(),available_at=clock_timestamp()+interval '1 minute'
		WHERE user_id=$1 RETURNING lease::text`, owner, claim.event, claim.attempts).Scan(&claim.lease)
	if err != nil {
		return nil, err
	}
	return claim, tx.Commit(ctx)
}

func (d *DB) FinishReleaseEmail(ctx context.Context, claim *ReleaseEmailClaim, delivered bool) error {
	tx, err := d.begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if err := identity(ctx, tx, claim.owner); err != nil {
		return err
	}
	var owner string
	err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE id=$1 FOR SHARE`, claim.owner).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var matched bool
	err = tx.QueryRow(ctx, `SELECT true FROM release_email_jobs
		WHERE user_id=$1 AND lease=$2 AND available_at>clock_timestamp() FOR UPDATE`, claim.owner, claim.lease).Scan(&matched)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if delivered || claim.attempts == 5 {
		return finishReleaseEmail(ctx, tx, claim, delivered)
	}
	_, err = tx.Exec(ctx, `UPDATE release_email_jobs SET lease=NULL,
		available_at=now()+($2::int*interval '1 minute') WHERE user_id=$1`, claim.owner, claim.attempts)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func finishReleaseEmail(ctx context.Context, tx pgx.Tx, claim *ReleaseEmailClaim, delivered bool) error {
	title := "Release email exhausted retries"
	if delivered {
		title = "Release email sent"
	}
	_, err := tx.Exec(ctx, `INSERT INTO notifications(user_id,event_id,channel,title,body)
		VALUES($1,$2,'EMAIL',$3,'')
		ON CONFLICT(user_id,event_id,channel) WHERE event_id IS NOT NULL DO NOTHING`, claim.owner, claim.event, title)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE release_email_jobs SET lease=NULL,event_id=NULL,attempts=0,
		available_at=now()+interval '5 minutes' WHERE user_id=$1`, claim.owner)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
