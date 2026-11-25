package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

type ReleaseEmailConsent struct {
	EnabledSince *time.Time `json:"enabledSince"`
}

func (r *Repository) ReleaseEmailConsent(ctx context.Context) (ReleaseEmailConsent, error) {
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return ReleaseEmailConsent{}, err
	}
	defer rollback(tx)
	var consent ReleaseEmailConsent
	if err := tx.QueryRow(ctx, `SELECT release_email_since FROM users WHERE id=$1`, owner.ID).Scan(&consent.EnabledSince); err != nil {
		return ReleaseEmailConsent{}, err
	}
	return consent, tx.Commit(ctx)
}

func (r *Repository) SetReleaseEmailConsent(ctx context.Context, enabled bool) (ReleaseEmailConsent, error) {
	tx, owner, err := r.db.sessionTxWithUserLock(ctx, r.token, pgx.TxOptions{}, true)
	if err != nil {
		return ReleaseEmailConsent{}, err
	}
	defer rollback(tx)
	if !owner.EmailVerified {
		return ReleaseEmailConsent{}, store.ErrEmailUnverified
	}
	var consent ReleaseEmailConsent
	err = tx.QueryRow(ctx, `UPDATE users SET release_email_since=
		CASE WHEN $2 THEN COALESCE(release_email_since,statement_timestamp()) END
		WHERE id=$1 RETURNING release_email_since`, owner.ID, enabled).Scan(&consent.EnabledSince)
	if err != nil {
		return ReleaseEmailConsent{}, err
	}
	return consent, tx.Commit(ctx)
}
