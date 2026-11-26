package postgres_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestCanonicalSeasonRatingsRemainIndependentAndOwnerScoped(t *testing.T) {
	ctx := context.Background()
	db, admin, _, _ := testDB(t)
	const anime = "10000000-0000-4000-8000-000000000001"
	const unrelated = "10000000-0000-4000-8000-000000000002"
	if _, err := admin.Exec(ctx, `INSERT INTO data_sources(provider_key,display_name) VALUES ('season-rating-fixture','Synthetic');
		INSERT INTO catalog_metadata(anime_id, source_id, payload, observed_at)
		SELECT a.id, s.id, '{"seasonNumber":2,"partNumber":1}', now() FROM anime a, data_sources s
		WHERE a.id='10000000-0000-4000-8000-000000000001' AND s.provider_key='season-rating-fixture'`); err != nil {
		t.Fatal(err)
	}
	account, err := db.Register(ctx, "season-rating@example.test", "synthetic-season-rating-password")
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.Register(ctx, "other-season-rating@example.test", "synthetic-season-rating-password")
	if err != nil {
		t.Fatal(err)
	}
	verifyAccount(t, db, account.Token)
	repo := db.Repository(account.Token)
	entry, err := repo.Add(ctx, anime, "COMPLETED", 12)
	if err != nil || !entry.SeasonRatingAvailable {
		t.Fatalf("numbered title: %+v %v", entry, err)
	}
	unrated, err := repo.Add(ctx, unrelated, "WATCHING", 1)
	if err != nil || unrated.SeasonRatingAvailable {
		t.Fatalf("unclassified title: %+v %v", unrated, err)
	}
	if _, _, err := repo.Rate(ctx, anime, 3); err != nil {
		t.Fatal(err)
	}
	handler := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	write := func(token, id, input string, status int) store.Entry {
		t.Helper()
		request := httptest.NewRequest("PUT", "/api/v1/tracker/"+id+"/season-rating", strings.NewReader(input))
		request.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != status {
			t.Fatalf("status=%d want=%d: %s", recorder.Code, status, recorder.Body)
		}
		var entry store.Entry
		if status == 200 && json.Unmarshal(recorder.Body.Bytes(), &entry) != nil {
			t.Fatalf("entry response: %s", recorder.Body)
		}
		return entry
	}
	const score = `{"score":5,"expectedCycle":1}`
	write(other.Token, entry.ID, score, 403)
	verifyAccount(t, db, other.Token)
	write(other.Token, entry.ID, score, 404)
	write(account.Token, unrated.ID, score, 409)
	write(account.Token, entry.ID, `{"expectedCycle":1}`, 400)
	if rated := write(account.Token, entry.ID, score, 200); rated.SeasonRating != 5 || rated.Rating != 3 {
		t.Fatalf("ratings conflated: %+v", rated)
	}
	if _, err := admin.Exec(ctx, `UPDATE catalog_metadata SET payload='{"seasonNumber":7,"partNumber":3}' WHERE anime_id=$1`, anime); err != nil {
		t.Fatal(err)
	}
	if rewatch, _, err := repo.StartRewatch(ctx, entry.ID, 1); err != nil || rewatch.WatchCycle != 2 || rewatch.SeasonRating != 5 {
		t.Fatalf("rewatch lost season rating: %+v %v", rewatch, err)
	}
	write(account.Token, entry.ID, score, 409)
	if _, err := admin.Exec(ctx, `UPDATE catalog_metadata SET payload='{}' WHERE anime_id=$1`, anime); err != nil {
		t.Fatal(err)
	}
	export, err := db.ExportAccount(ctx, account.Token)
	if err != nil || !strings.Contains(string(export.Data["ratings"]), `"scope": "SEASON"`) {
		t.Fatalf("season rating missing from account export: %s %v", export.Data["ratings"], err)
	}
	for range 2 {
		cleared := write(account.Token, entry.ID, `{"score":null,"expectedCycle":2}`, 200)
		if cleared.SeasonRating != 0 || cleared.SeasonRatingAvailable || cleared.Rating != 3 {
			t.Fatalf("clear affected wrong score or classification: %+v", cleared)
		}
	}
}
