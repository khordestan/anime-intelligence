package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type ReleaseEmailDiagnostics struct {
	AsOf         time.Time  `json:"asOf"`
	Subscribers  int64      `json:"subscribers"`
	Due          int64      `json:"due"`
	Leased       int64      `json:"leased"`
	Delayed      int64      `json:"delayed"`
	Attempted    int64      `json:"attempted"`
	ExhaustedDue int64      `json:"exhaustedDue"`
	OldestDueAt  *time.Time `json:"oldestDueAt"`
}

func (d *DB) ReleaseEmailDiagnostics(ctx context.Context) (ReleaseEmailDiagnostics, error) {
	var snapshot ReleaseEmailDiagnostics
	tx, err := d.beginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return snapshot, err
	}
	defer rollback(tx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.release_email_worker','on',true)`); err != nil {
		return snapshot, err
	}
	err = tx.QueryRow(ctx, `SELECT statement_timestamp(), count(*),
		count(*) FILTER (WHERE available_at<=statement_timestamp()),
		count(*) FILTER (WHERE available_at>statement_timestamp() AND lease IS NOT NULL),
		count(*) FILTER (WHERE available_at>statement_timestamp() AND lease IS NULL),
		count(*) FILTER (WHERE attempts>0),
		count(*) FILTER (WHERE attempts=5 AND available_at<=statement_timestamp()),
		min(available_at) FILTER (WHERE available_at<=statement_timestamp())
		FROM release_email_jobs`).Scan(
		&snapshot.AsOf, &snapshot.Subscribers, &snapshot.Due, &snapshot.Leased,
		&snapshot.Delayed, &snapshot.Attempted, &snapshot.ExhaustedDue, &snapshot.OldestDueAt)
	if err != nil {
		return ReleaseEmailDiagnostics{}, err
	}
	return snapshot, tx.Commit(ctx)
}
