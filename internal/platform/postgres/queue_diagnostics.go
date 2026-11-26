package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

const identityRequestCapacity = 256

// QueueDiagnostics contains operational aggregates only. Completed counts are
// terminal work, not successful deliveries: discarded/expired work is included.
type QueueDiagnostics struct {
	Version         int         `json:"version"`
	AsOf            time.Time   `json:"asOf"`
	PayloadCutoff   *time.Time  `json:"payloadCutoff,omitempty"`
	RequestCapacity int         `json:"requestCapacity"`
	Requests        QueueCounts `json:"requests"`
	Deliveries      QueueCounts `json:"deliveries"`
}

// Outcome counts describe retained rows and decrease when cleanup purges them.
// Accepted means the sender accepted the mail, not confirmed mailbox delivery.
type QueueOutcomes struct {
	Issued        int64 `json:"issued"`
	NoAction      int64 `json:"noAction"`
	Accepted      int64 `json:"accepted"`
	Invalidated   int64 `json:"invalidated"`
	Expired       int64 `json:"expired"`
	Exhausted     int64 `json:"exhausted"`
	LegacyUnknown int64 `json:"legacyUnknown"`
}
type QueueCounts struct {
	Outcomes                 QueueOutcomes `json:"outcomes"`
	Active                   int64         `json:"active"`
	Ready                    int64         `json:"ready"`
	Leased                   int64         `json:"leased"`
	Delayed                  int64         `json:"delayed"`
	AttemptedActive          int64         `json:"attemptedActive"`
	AwaitingCleanup          int64         `json:"awaitingCleanup"`
	Completed                int64         `json:"completed"`
	PayloadRows              int64         `json:"payloadRows"`
	PayloadRowsThroughCutoff *int64        `json:"payloadRowsThroughCutoff,omitempty"`
	OldestActiveAt           *time.Time    `json:"oldestActiveAt"`
	OldestPayloadAt          *time.Time    `json:"oldestPayloadAt"`
	LatestPayloadExpiry      *time.Time    `json:"latestPayloadExpiry"`
}

var ErrIdentityPayloadCutoff = errors.New("identity payload cutoff must be at or before the database snapshot")

func (d *DB) IdentityQueueDiagnostics(ctx context.Context) (QueueDiagnostics, error) {
	return d.identityQueueDiagnostics(ctx, nil)
}

func (d *DB) IdentityQueueDiagnosticsThrough(ctx context.Context, cutoff time.Time) (QueueDiagnostics, error) {
	if cutoff.IsZero() {
		return QueueDiagnostics{}, ErrIdentityPayloadCutoff
	}
	cutoff = cutoff.UTC()
	return d.identityQueueDiagnostics(ctx, &cutoff)
}

func (d *DB) identityQueueDiagnostics(ctx context.Context, cutoff *time.Time) (QueueDiagnostics, error) {
	tx, err := d.beginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return QueueDiagnostics{}, err
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.identity_request_worker','on',true),set_config('app.delivery_worker','on',true)`); err != nil {
		return QueueDiagnostics{}, err
	}
	snapshot := QueueDiagnostics{Version: 1, RequestCapacity: identityRequestCapacity}
	if err = tx.QueryRow(ctx, `SELECT now()`).Scan(&snapshot.AsOf); err != nil {
		return QueueDiagnostics{}, err
	}
	if cutoff != nil {
		if cutoff.After(snapshot.AsOf) {
			return QueueDiagnostics{}, ErrIdentityPayloadCutoff
		}
		snapshot.PayloadCutoff = cutoff
	}
	// The table identifiers below are a fixed internal allowlist, never input.
	for _, queue := range []struct {
		table  string
		counts *QueueCounts
	}{{"identity_requests", &snapshot.Requests}, {"identity_outbox", &snapshot.Deliveries}} {
		q := queue.counts
		var throughCutoff int64
		err = tx.QueryRow(ctx, `SELECT
 count(*) FILTER(WHERE completed_at IS NULL AND expires_at>now()),
 count(*) FILTER(WHERE completed_at IS NULL AND payload IS NOT NULL AND expires_at>now() AND attempts<5 AND available_at<=now() AND (lease_until IS NULL OR lease_until<=now())),
 count(*) FILTER(WHERE completed_at IS NULL AND expires_at>now() AND lease_until>now()),
 count(*) FILTER(WHERE completed_at IS NULL AND expires_at>now() AND attempts<5 AND available_at>now() AND (lease_until IS NULL OR lease_until<=now())),
 count(*) FILTER(WHERE completed_at IS NULL AND expires_at>now() AND attempts>0),
 count(*) FILTER(WHERE completed_at IS NULL AND (expires_at<=now() OR (attempts>=5 AND (lease_until IS NULL OR lease_until<=now())))),
 count(*) FILTER(WHERE completed_at IS NOT NULL),
 count(*) FILTER(WHERE payload IS NOT NULL),
 count(*) FILTER(WHERE payload IS NOT NULL AND created_at<=$1::timestamptz),
 count(*) FILTER(WHERE completed_at IS NOT NULL AND outcome='ISSUED'),
 count(*) FILTER(WHERE completed_at IS NOT NULL AND outcome='NO_ACTION'),
 count(*) FILTER(WHERE completed_at IS NOT NULL AND outcome='ACCEPTED'),
 count(*) FILTER(WHERE completed_at IS NOT NULL AND outcome='INVALIDATED'),
 count(*) FILTER(WHERE completed_at IS NOT NULL AND outcome='EXPIRED'),
 count(*) FILTER(WHERE completed_at IS NOT NULL AND outcome='EXHAUSTED'),
 count(*) FILTER(WHERE completed_at IS NOT NULL AND (outcome IS NULL OR outcome='LEGACY_UNKNOWN')),
 min(created_at) FILTER(WHERE completed_at IS NULL AND expires_at>now()),
 min(created_at) FILTER(WHERE payload IS NOT NULL),
 max(expires_at) FILTER(WHERE payload IS NOT NULL)
 FROM `+queue.table, cutoff).Scan(&q.Active, &q.Ready, &q.Leased, &q.Delayed, &q.AttemptedActive, &q.AwaitingCleanup, &q.Completed, &q.PayloadRows, &throughCutoff, &q.Outcomes.Issued, &q.Outcomes.NoAction, &q.Outcomes.Accepted, &q.Outcomes.Invalidated, &q.Outcomes.Expired, &q.Outcomes.Exhausted, &q.Outcomes.LegacyUnknown, &q.OldestActiveAt, &q.OldestPayloadAt, &q.LatestPayloadExpiry)
		if err != nil {
			return QueueDiagnostics{}, err
		}
		if cutoff != nil {
			q.PayloadRowsThroughCutoff = &throughCutoff
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return QueueDiagnostics{}, err
	}
	return snapshot, nil
}
