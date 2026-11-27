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
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

func TestViewingHistorySummaryBoundsUTCAndOwner(t *testing.T) {
	db, admin, _, _ := testDB(t)
	ctx := context.Background()
	register := func(email string) postgres.Session {
		t.Helper()
		session, err := db.Register(ctx, email, "synthetic-viewing-summary-password")
		if err != nil {
			t.Fatal(err)
		}
		return session
	}
	one, two, unverified := register("first-history@example.test"), register("other-history@example.test"), register("unverified-history@example.test")
	const title = "10000000-0000-4000-8000-000000000001"
	for _, account := range []postgres.Session{one, two} {
		verifyAccount(t, db, account.Token)
		if _, err := db.Repository(account.Token).Add(ctx, title, "WATCHING", 0); err != nil {
			t.Fatal(err)
		}
	}
	queueExec(t, admin, `WITH now_month AS (
			SELECT date_trunc('month',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS utc_start
		)
		INSERT INTO watch_cycles(user_id,anime_id,cycle_number,episodes_watched,rating,archived_at)
		SELECT v.user_id,$2,v.pass,12,v.rating,v.archived_at
		FROM now_month CROSS JOIN LATERAL (VALUES
		  ($1::uuid,1,3,utc_start),($1::uuid,2,NULL,utc_start),($1::uuid,3,5,utc_start),
		  ($1::uuid,4,2,((utc_start AT TIME ZONE 'UTC')-interval '1 month') AT TIME ZONE 'UTC'),
		  ($1::uuid,5,1,((utc_start AT TIME ZONE 'UTC')-interval '12 months') AT TIME ZONE 'UTC'),
		  ($1::uuid,6,1,now()+interval '1 day'),($3::uuid,1,2,utc_start)
		) v(user_id,pass,rating,archived_at)`, one.User.ID, title, two.User.ID)
	h := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	get := func(token string, status int) []postgres.ViewingHistoryMonth {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/v1/tracker/viewing-history-summary", nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, r)
		if recorder.Code != status || recorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("history summary status=%d, want=%d, body=%s", recorder.Code, status, recorder.Body)
		}
		var out struct {
			Months []postgres.ViewingHistoryMonth
		}
		if status == 200 && (json.Unmarshal(recorder.Body.Bytes(), &out) != nil || len(out.Months) != 12) {
			t.Fatalf("unbounded or invalid summary: %s", recorder.Body)
		}
		return out.Months
	}
	get("", 401)
	get(unverified.Token, 403)
	months := get(one.Token, 200)
	if first := months[0]; first.Archived != 3 || first.Rated != 2 || first.AverageRating == nil || *first.AverageRating != 4 {
		t.Fatalf("current UTC history: %+v", first)
	}
	if prior := months[1]; prior.Archived != 1 || prior.Rated != 1 || prior.AverageRating == nil || *prior.AverageRating != 2 {
		t.Fatalf("previous UTC history: %+v", prior)
	}
	for _, empty := range months[2:] {
		if empty.Archived != 0 || empty.Rated != 0 || empty.AverageRating != nil {
			t.Fatalf("empty month returned evidence: %+v", empty)
		}
	}
	if first := get(two.Token, 200)[0]; first.Archived != 1 || first.Rated != 1 || first.AverageRating == nil || *first.AverageRating != 2 {
		t.Fatalf("an owner's summary included another owner: %+v", first)
	}

	type analysisAnswer struct {
		Engine                  string
		TopicMatched            bool
		FromMonth, ThroughMonth string
		ArchivedPasses          *int
		ArchiveRatings          *struct {
			Rated   int
			Average *float64
		}
	}
	ask := func(token, body string, status int) analysisAnswer {
		t.Helper()
		request := httptest.NewRequest("POST", "/api/v1/analysis/library", strings.NewReader(body))
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, request)
		var answer analysisAnswer
		if recorder.Code != status || recorder.Header().Get("Cache-Control") != "no-store" ||
			(status == 200 && json.Unmarshal(recorder.Body.Bytes(), &answer) != nil) ||
			strings.Contains(recorder.Body.String(), title) || strings.Contains(recorder.Body.String(), "question") && status == 200 {
			t.Fatalf("private analysis status=%d, want=%d, body=%s", recorder.Code, status, recorder.Body)
		}
		return answer
	}
	const question = `{"question":"How many passes and what are my ratings?"}`
	ask("", question, 401)
	ask(unverified.Token, question, 403)
	ask(one.Token, `{"question":"    "}`, 400)
	ask(one.Token, `{"question":"My ratings?","userId":"different"}`, 400)
	answer := ask(one.Token, question, 200)
	if answer.Engine != "library-history-v1" || !answer.TopicMatched || answer.FromMonth != months[11].Month ||
		answer.ThroughMonth != months[0].Month || answer.ArchivedPasses == nil || *answer.ArchivedPasses != 4 ||
		answer.ArchiveRatings == nil || answer.ArchiveRatings.Rated != 3 || answer.ArchiveRatings.Average == nil ||
		*answer.ArchiveRatings.Average != 10.0/3 {
		t.Fatalf("unexpected combined private answer: %+v", answer)
	}
	otherAnswer := ask(two.Token, `{"question":"What are my archived ratings?"}`, 200)
	if !otherAnswer.TopicMatched || otherAnswer.ArchivedPasses != nil || otherAnswer.ArchiveRatings == nil ||
		otherAnswer.ArchiveRatings.Rated != 1 || otherAnswer.ArchiveRatings.Average == nil || *otherAnswer.ArchiveRatings.Average != 2 {
		t.Fatalf("private ratings scope failed: %+v", otherAnswer)
	}
	unsupported := ask(one.Token, `{"question":"Where can I stream television?"}`, 200)
	if unsupported.TopicMatched || unsupported.ArchivedPasses != nil || unsupported.ArchiveRatings != nil || unsupported.FromMonth != "" {
		t.Fatalf("unsupported question returned private evidence: %+v", unsupported)
	}

}
