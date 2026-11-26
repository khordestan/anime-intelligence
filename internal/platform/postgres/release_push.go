package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/khordestan/anime-intelligence/internal/platform/releasepush"
)

var ErrPushRegistrationConflict = errors.New("browser push subscription is unavailable")

type PushRegistration struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

func (r *Repository) PushRegistration(ctx context.Context) (*PushRegistration, error) {
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	registration, err := pushRegistration(ctx, tx, owner.ID)
	if err != nil {
		return nil, err
	}
	return registration, tx.Commit(ctx)
}

func (r *Repository) RegisterPush(ctx context.Context, sender *releasepush.Sender, subscription releasepush.Subscription) (*PushRegistration, error) {
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	payload, err := sender.Seal(owner.ID, subscription)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(subscription)
	if err != nil {
		return nil, err
	}
	endpointDigest, subscriptionDigest := sha256.Sum256([]byte(subscription.Endpoint)), sha256.Sum256(body)
	if err = lockReleaseOwner(ctx, tx, owner.ID); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO release_push_subscriptions(user_id,endpoint_digest,subscription_digest,payload)
	VALUES($1,$2,$3,$4) ON CONFLICT(user_id) DO UPDATE SET
	id=gen_random_uuid(),endpoint_digest=excluded.endpoint_digest,subscription_digest=excluded.subscription_digest,
	payload=excluded.payload,created_at=now(),available_at=now(),event_id=NULL,attempts=0,lease=NULL
	WHERE release_push_subscriptions.subscription_digest<>excluded.subscription_digest`, owner.ID, endpointDigest[:], subscriptionDigest[:], payload)
	var constraint *pgconn.PgError
	if errors.As(err, &constraint) && constraint.Code == "23505" && constraint.ConstraintName == "release_push_subscriptions_endpoint_digest_key" {
		return nil, ErrPushRegistrationConflict
	}
	if err != nil {
		return nil, err
	}
	registration, err := pushRegistration(ctx, tx, owner.ID)
	if err != nil {
		return nil, err
	}
	return registration, tx.Commit(ctx)
}

func (r *Repository) RemovePush(ctx context.Context, registrationID string) error {
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if err = lockReleaseOwner(ctx, tx, owner.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM release_push_subscriptions WHERE user_id=$1 AND id=$2`, owner.ID, registrationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func pushRegistration(ctx context.Context, tx pgx.Tx, owner string) (*PushRegistration, error) {
	var registration PushRegistration
	err := tx.QueryRow(ctx, `SELECT id::text,created_at FROM release_push_subscriptions WHERE user_id=$1`, owner).Scan(&registration.ID, &registration.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &registration, err
}

func lockReleaseOwner(ctx context.Context, tx pgx.Tx, owner string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,742019685))`, owner)
	return err
}
