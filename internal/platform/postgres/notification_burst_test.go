package postgres_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestConcurrentNotificationBacklogCatchUp(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	const animeID = "10000000-0000-4000-8000-000000000001"
	const backlog = 230
	const pollsPerAccount = 8
	var accounts []postgres.User
	var repositories []*postgres.Repository
	tracks := []string{"SUB", "DUB"}
	for _, track := range tracks {
		session, err := db.Register(ctx, fmt.Sprintf("burst-%s@example.test", track), "synthetic-alert-burst-password")
		if err != nil {
			t.Fatal(err)
		}
		verifyAccount(t, db, session.Token)
		repository := db.Repository(session.Token)
		if followed, err := repository.Follow(ctx, animeID, track); err != nil || !followed {
			t.Fatalf("%s follow failed: %t, %v", track, followed, err)
		}
		accounts = append(accounts, session.User)
		repositories = append(repositories, repository)
	}

	if _, err := owner.Exec(ctx, `INSERT INTO domain_events(event_key,event_type,aggregate_type,aggregate_id,payload,occurred_at)
		SELECT 'burst:'||track||':'||n,'EPISODE_RELEASED','ANIME',$1::uuid,
			jsonb_build_object('number',n,'track',track),now()
		FROM unnest(ARRAY['SUB','DUB']) AS tracks(track)
		CROSS JOIN generate_series(1,$2::int) AS episodes(n)`, animeID, backlog); err != nil {
		t.Fatal(err)
	}

	type outcome struct {
		index int
		page  postgres.NotificationPage
		err   error
	}
	start := make(chan struct{})
	results := make(chan outcome, len(repositories)*pollsPerAccount)
	var workers sync.WaitGroup
	for index, repository := range repositories {
		for range pollsPerAccount {
			workers.Go(func() {
				<-start
				page, err := repository.NotificationSummary(ctx)
				results <- outcome{index: index, page: page, err: err}
			})
		}
	}
	close(start)
	workers.Wait()
	close(results)

	created := make([]int, len(accounts))
	for result := range results {
		status := result.page.Materialization
		if result.err != nil || status == nil || status.Created > 100 || result.page.UnreadCount > backlog {
			t.Fatalf("owner %d returned invalid summary: %+v, %v", result.index, result.page, result.err)
		}
		created[result.index] += status.Created
	}
	for index, account := range accounts {
		if created[index] != backlog {
			t.Fatalf("owner %d materialized %d events, want %d", index, created[index], backlog)
		}
		var notices, foreign int
		track := tracks[index]
		if err := owner.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE e.payload->>'track' IS DISTINCT FROM $2)
			FROM notifications n JOIN domain_events e ON e.id=n.event_id
			WHERE n.user_id=$1 AND n.channel='IN_APP'`, account.ID, track).Scan(&notices, &foreign); err != nil {
			t.Fatal(err)
		}
		if notices != backlog || foreign != 0 {
			t.Fatalf("owner %d has %d stored notices and %d wrong-track notices", index, notices, foreign)
		}
	}
}
