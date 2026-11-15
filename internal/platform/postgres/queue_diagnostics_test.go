package postgres_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
)

func TestQueueDiagnosticsReportStatesWithoutPrivateDataOrMutations(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	snapshot, err := db.IdentityQueueDiagnostics(ctx)
	if err != nil || snapshot.Version != 1 || snapshot.RequestCapacity != 256 || snapshot.Requests.Active != 0 || snapshot.Requests.OldestActiveAt != nil || snapshot.Deliveries.PayloadRows != 0 {
		t.Fatal("empty diagnostics", err)
	}
	session, err := db.Register(ctx, "diagnostic-private@example.test", "synthetic-diagnostic-password")
	if err != nil {
		t.Fatal(err)
	}
	outbox, _ := db.IdentityOutbox(envelopeCipher(t, 1))
	if _, err = outbox.IssueEmailVerification(ctx, session.Token); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if err = outbox.SubmitRequest(ctx, identitymail.PasswordReset, "diagnostic-private@example.test"); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := owner.Query(ctx, `SELECT id::text FROM identity_requests ORDER BY created_at,id`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	// One each: ready, leased, delayed, expired, exhausted, completed.
	if _, err = owner.Exec(ctx, `UPDATE identity_requests SET attempts=1,lease_hash=decode(repeat('ab',32),'hex'),lease_until=now()+interval '1 minute' WHERE id=$1`, ids[1]); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, `UPDATE identity_requests SET attempts=1,available_at=now()+interval '1 minute' WHERE id=$1`, ids[2]); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, `UPDATE identity_requests SET expires_at=now()-interval '1 second' WHERE id=$1`, ids[3]); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, `UPDATE identity_requests SET attempts=5 WHERE id=$1`, ids[4]); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Exec(ctx, `UPDATE identity_requests SET payload=NULL,completed_at=now() WHERE id=$1`, ids[5]); err != nil {
		t.Fatal(err)
	}
	var before, after string
	if err = owner.QueryRow(ctx, `SELECT md5(string_agg(row_to_json(q)::text,',' ORDER BY id)) FROM identity_requests q`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	snapshot, err = db.IdentityQueueDiagnostics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	q := snapshot.Requests
	if q.Active != 4 || q.Ready != 1 || q.Leased != 1 || q.Delayed != 1 || q.AttemptedActive != 3 || q.AwaitingCleanup != 2 || q.Completed != 1 || q.PayloadRows != 5 || q.OldestActiveAt == nil || q.OldestPayloadAt == nil || q.LatestPayloadExpiry == nil {
		t.Fatalf("incorrect aggregate states: %+v", q)
	}
	if snapshot.Deliveries.Active != 1 || snapshot.Deliveries.Ready != 1 || snapshot.Deliveries.PayloadRows != 1 || snapshot.AsOf.IsZero() {
		t.Fatal("delivery snapshot omitted")
	}
	if err = owner.QueryRow(ctx, `SELECT md5(string_agg(row_to_json(q)::text,',' ORDER BY id)) FROM identity_requests q`).Scan(&after); err != nil || before != after {
		t.Fatal("diagnostics changed request state", err)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range append(ids, session.User.Email, session.User.ID, session.Token, "payload\"", "lease_hash", "token_hash") {
		if strings.Contains(string(encoded), private) {
			t.Fatal("diagnostics disclosed private fields")
		}
	}
	if _, err = db.Authenticate(ctx, session.Token); err != nil {
		t.Fatal("diagnostics leaked worker context into authentication", err)
	}
}
