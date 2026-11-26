package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/releasepush"
)

type PushClaim struct {
	owner, registration, event, lease string
	attempts                          int
	encrypted                         []byte
}

func (PushClaim) String() string     { return "browser push claim (redacted)" }
func (c PushClaim) GoString() string { return c.String() }

func (c *PushClaim) Subscription(sender *releasepush.Sender) (releasepush.Subscription, error) {
	return sender.Open(c.owner, c.encrypted)
}

func (d *DB) ClaimPush(ctx context.Context) (*PushClaim, error) {
	tx, err := d.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.release_push_worker','on',true)`); err != nil {
		return nil, err
	}
	var claim PushClaim
	err = tx.QueryRow(ctx, `SELECT user_id::text FROM release_push_subscriptions
		WHERE available_at<=now() ORDER BY available_at,user_id LIMIT 1`).Scan(&claim.owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT set_config('app.release_push_worker','off',true)`); err != nil {
		return nil, err
	}
	if err = identity(ctx, tx, claim.owner); err != nil {
		return nil, err
	}
	var eligible bool
	err = tx.QueryRow(ctx, `SELECT email_verified_at IS NOT NULL AND disabled_at IS NULL AND
		(locked_until IS NULL OR locked_until<=now()) FROM users WHERE id=$1 FOR SHARE`, claim.owner).Scan(&eligible)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err = lockReleaseOwner(ctx, tx, claim.owner); err != nil {
		return nil, err
	}
	var pending *string
	var consentAt time.Time
	err = tx.QueryRow(ctx, `SELECT id::text,created_at,event_id::text,attempts,payload
		FROM release_push_subscriptions WHERE user_id=$1 AND available_at<=now() FOR UPDATE`, claim.owner).
		Scan(&claim.registration, &consentAt, &pending, &claim.attempts, &claim.encrypted)
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
				WHERE e.aggregate_type='ANIME' AND e.occurred_at>=f.created_at AND e.occurred_at>=$2
				  AND e.occurred_at>=now()-interval '24 hours' AND e.occurred_at<=now()
				  AND ((e.event_type='EPISODE_RELEASED' AND (f.release_track='ALL' OR f.release_track=e.payload->>'track'))
				    OR (e.event_type='SEASON_ANNOUNCED' AND f.scope='FRANCHISE'))
				  AND NOT EXISTS(SELECT 1 FROM notifications n WHERE n.user_id=$1 AND n.event_id=e.id AND n.channel='PUSH')
			) candidate ORDER BY (candidate.id=$3::uuid) DESC NULLS LAST,candidate.occurred_at,candidate.id LIMIT 1`,
			claim.owner, consentAt, pending).Scan(&claim.event)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	if claim.event == "" {
		_, err = tx.Exec(ctx, `UPDATE release_push_subscriptions SET available_at=now()+interval '5 minutes',
			event_id=NULL,attempts=0,lease=NULL WHERE user_id=$1`, claim.owner)
		if err != nil {
			return nil, err
		}
		return nil, tx.Commit(ctx)
	}
	if pending == nil || *pending != claim.event {
		claim.attempts = 0
	}
	if claim.attempts == 5 {
		return nil, finishPush(ctx, tx, &claim, releasepush.Retry, 0)
	}
	claim.attempts++
	err = tx.QueryRow(ctx, `UPDATE release_push_subscriptions SET event_id=$2,attempts=$3,
		lease=gen_random_uuid(),available_at=clock_timestamp()+interval '1 minute'
		WHERE user_id=$1 RETURNING lease::text`, claim.owner, claim.event, claim.attempts).Scan(&claim.lease)
	if err != nil {
		return nil, err
	}
	return &claim, tx.Commit(ctx)
}

// PushClaimCurrent checks that the live registration and consent still match
// just before the network call. An already in-flight send may complete.
func (d *DB) PushClaimCurrent(ctx context.Context, claim *PushClaim) (bool, error) {
	tx, err := d.begin(ctx)
	if err != nil {
		return false, err
	}
	defer rollback(tx)
	if err = identity(ctx, tx, claim.owner); err != nil {
		return false, err
	}
	var current bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM release_push_subscriptions p
		JOIN users u ON u.id=p.user_id WHERE p.user_id=$1 AND p.id=$2 AND p.lease=$3 AND p.available_at>clock_timestamp()
		AND u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL AND (u.locked_until IS NULL OR u.locked_until<=now()))`,
		claim.owner, claim.registration, claim.lease).Scan(&current); err != nil {
		return false, err
	}
	return current, tx.Commit(ctx)
}

func (d *DB) FinishPush(ctx context.Context, claim *PushClaim, outcome releasepush.Delivery, retryAfter time.Duration) error {
	tx, err := d.begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if err = identity(ctx, tx, claim.owner); err != nil {
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
	if err = lockReleaseOwner(ctx, tx, claim.owner); err != nil {
		return err
	}
	var matched bool
	err = tx.QueryRow(ctx, `SELECT true FROM release_push_subscriptions
		WHERE user_id=$1 AND id=$2 AND lease=$3 AND available_at>clock_timestamp() FOR UPDATE`,
		claim.owner, claim.registration, claim.lease).Scan(&matched)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return finishPush(ctx, tx, claim, outcome, retryAfter)
}

func finishPush(ctx context.Context, tx pgx.Tx, claim *PushClaim, outcome releasepush.Delivery, retryAfter time.Duration) error {
	switch {
	case outcome == releasepush.Gone:
		if _, err := tx.Exec(ctx, `DELETE FROM release_push_subscriptions WHERE user_id=$1`, claim.owner); err != nil {
			return err
		}
	case outcome == releasepush.Delivered || claim.attempts == 5:
		title := "Browser push exhausted retries"
		if outcome == releasepush.Delivered {
			title = "Browser push accepted"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO notifications(user_id,event_id,channel,title,body)
			VALUES($1,$2,'PUSH',$3,'') ON CONFLICT(user_id,event_id,channel) WHERE event_id IS NOT NULL DO NOTHING`,
			claim.owner, claim.event, title); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE release_push_subscriptions SET lease=NULL,event_id=NULL,attempts=0,
			available_at=now()+interval '1 minute' WHERE user_id=$1`, claim.owner); err != nil {
			return err
		}
	default:
		delay := max(retryAfter, time.Duration(claim.attempts)*time.Minute)
		if _, err := tx.Exec(ctx, `UPDATE release_push_subscriptions SET lease=NULL,
			available_at=now()+($2::bigint*interval '1 second') WHERE user_id=$1`, claim.owner, int64((delay+time.Second-1)/time.Second)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
