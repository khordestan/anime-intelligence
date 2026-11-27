package postgres_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/platform/catalogsync"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestCatalogDetailsPublicAndConsistent(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	if _, err := owner.Exec(ctx, `INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('details-fixture','Synthetic details',true,'https://example.test/policy')`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	s := feed.Snapshot{Version: 1, Provider: "details-fixture", ObservedAt: now, Titles: []feed.Title{
		{ExternalID: "one", Title: "Synthetic Details One", Season: "SPRING", SeasonNumber: 2, PartNumber: 1, MediaType: "TV", Status: "RELEASING", Synonyms: []string{"Synthetic Alias"}, Genres: []string{"Adventure"}, Tags: []string{"Space"}, Themes: []string{"Synthetic theme"}, Demographics: []string{"Synthetic demographic"}, Studios: []string{"Synthetic Studio"}, StaffCredits: []feed.Credit{{Name: "Synthetic Director", Role: "Director"}}, CharacterCredits: []feed.Credit{{Name: "Synthetic Hidden Character", Role: "Supporting"}}, Relations: []feed.Relation{{ExternalID: "two", Type: "SEQUEL"}}, Episodes: []feed.Episode{{Number: 1, Track: "SUB", ScheduledAt: &now}}},
		{ExternalID: "two", Title: "Synthetic Details Two", MediaType: "TV", Status: "ANNOUNCED"},
	}}
	if _, err := catalogsync.Import(ctx, owner, s, now); err != nil {
		t.Fatal(err)
	}
	var id, target string
	if err := owner.QueryRow(ctx, `SELECT anime_id::text FROM anime_external_ids WHERE provider='details-fixture' AND external_id='one'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err := owner.QueryRow(ctx, `SELECT anime_id::text FROM anime_external_ids WHERE provider='details-fixture' AND external_id='two'`).Scan(&target); err != nil {
		t.Fatal(err)
	}
	matches, err := db.Repository("").Catalog(ctx, "Synthetic Alias", 24, 0)
	if err != nil || len(matches) != 1 || matches[0].ID != id {
		t.Fatal("imported alias search", matches, err)
	}
	repo := db.Repository("")
	for _, fixture := range []struct {
		name  string
		count int
	}{
		{"  SYNTHETIC DIRECTOR  ", 1},
		{"Synthetic", 0},
		{"Synthetic Hidden Character", 0},
	} {
		matches, err := repo.FilterCatalog(ctx, store.CatalogFilter{Staff: fixture.name}, 10, 0)
		if err != nil || len(matches) != fixture.count || (fixture.count == 1 && matches[0].ID != id) {
			t.Fatalf("staff filter %q: %v, %v", fixture.name, matches, err)
		}
	}
	facets, err := repo.CatalogFacets(ctx, "staff", "sYnThEtIc", 1)
	if err != nil || len(facets.Items) != 1 || facets.Items[0] != "Synthetic Director" || facets.HasMore {
		t.Fatalf("staff facet: %+v, %v", facets, err)
	}
	h := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	for _, fixture := range []struct {
		query string
		count int
	}{
		{"character=+SYNTHETIC+HIDDEN+CHARACTER+&staff=Synthetic+Director", 1},
		{"character=Synthetic+Hidden", 0},
		{"character=Synthetic+Director", 0},
	} {
		response := httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/anime?"+fixture.query, nil))
		var result struct{ Items []store.Anime }
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &result) != nil ||
			len(result.Items) != fixture.count || (fixture.count == 1 && result.Items[0].ID != id) ||
			strings.Contains(response.Body.String(), "Synthetic Hidden Character") {
			t.Fatalf("invalid character search response: %d %s", response.Code, response.Body)
		}
	}
	requestFacet := func(query string) *httptest.ResponseRecorder {
		out := httptest.NewRecorder()
		h.ServeHTTP(out, httptest.NewRequest("GET", "/api/v1/anime/facets?field=characters"+query, nil))
		return out
	}
	if got := requestFacet("&prefix=+++"); got.Code != 400 {
		t.Fatalf("character suggestions accepted a blank prefix: %d %s", got.Code, got.Body)
	}
	response := requestFacet("&prefix=sYnThEtIc&limit=1")
	var names store.CatalogFacetPage
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &names) != nil ||
		len(names.Items) != 1 || names.Items[0] != "Synthetic Hidden Character" || names.HasMore {
		t.Fatalf("character suggestions: %d %s", response.Code, response.Body)
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/anime/"+id+"/details", nil))
	var out store.CatalogDetails
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil {
		t.Fatalf("detail response: %d %s", w.Code, w.Body.String())
	}
	if len(out.ExternalIDs) != 1 || out.ExternalIDs[0].Provider != "details-fixture" || out.ExternalIDs[0].ExternalID != "one" || out.Anime.ID != id || out.Provider != "details-fixture" || out.ObservedAt == nil || !out.ObservedAt.Equal(now) || len(out.Genres) != 1 || len(out.Synonyms) != 1 || len(out.Tags) != 1 || len(out.Themes) != 1 || out.Themes[0] != "Synthetic theme" || len(out.Demographics) != 1 || out.Demographics[0] != "Synthetic demographic" || len(out.Studios) != 1 || len(out.Relations) != 1 || out.Relations[0].AnimeID != target || out.Relations[0].Title != "Synthetic Details Two" || out.Relations[0].Status != "ANNOUNCED" || out.Relations[0].MediaType != "TV" || len(out.Episodes) != 1 || out.Episodes[0].Revision != 1 || out.Episodes[0].ReleasedAt != nil {
		t.Fatalf("incomplete details: %+v", out)
	}
	if out.SeasonNumber != 2 || out.PartNumber != 1 || out.Anime.Season != "SPRING" {
		t.Fatalf("missing or conflated installment metadata: %+v", out)
	}
	if len(out.StaffCredits) != 1 || out.StaffCredits[0].Name != "Synthetic Director" ||
		!out.HasCharacterCredits || strings.Contains(w.Body.String(), "Synthetic Hidden Character") {
		t.Fatalf("catalog detail credit boundary: %s", w.Body)
	}
	for _, fixture := range []struct {
		id     string
		status int
		count  int
	}{
		{id, 200, 1}, {target, 200, 0}, {"not-a-uuid", 400, 0}, {"00000000-0000-4000-8000-000000000000", 404, 0},
	} {
		response := httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/anime/"+fixture.id+"/character-credits", nil))
		var credits store.CharacterCredits
		if response.Code != fixture.status || (fixture.status == 200 &&
			(json.Unmarshal(response.Body.Bytes(), &credits) != nil || credits.Items == nil || len(credits.Items) != fixture.count)) {
			t.Fatalf("character credits: %d %s", response.Code, response.Body)
		}
		if fixture.count == 1 && (credits.Items[0].Name != "Synthetic Hidden Character" || credits.Items[0].Role != "Supporting" ||
			credits.Provider != "details-fixture" || credits.ObservedAt == nil || !credits.ObservedAt.Equal(now)) {
			t.Fatalf("unattributed character credits: %+v", credits)
		}
	}
	reverse, found, reverseErr := db.Repository("").CatalogDetails(ctx, target)
	if reverseErr != nil || !found || len(reverse.Relations) != 1 || reverse.Relations[0].AnimeID != id || reverse.Relations[0].Type != "PREQUEL" || reverse.Relations[0].Title != "Synthetic Details One" {
		t.Fatal("one-sided season relation lost reverse navigation", reverse, reverseErr)
	}
	lookup := httptest.NewRecorder()
	h.ServeHTTP(lookup, httptest.NewRequest("GET", "/api/v1/anime/lookup?provider=details-fixture&externalId=one", nil))
	var canonical store.Anime
	if lookup.Code != 200 || json.Unmarshal(lookup.Body.Bytes(), &canonical) != nil || canonical.ID != id {
		t.Fatal("exact identity route", lookup.Code, lookup.Body.String())
	}
	for query, want := range map[string]int{"?provider=details-fixture&externalId=missing": 404, "?provider=details-fixture": 400, "?provider=bad/provider&externalId=one": 400, "?provider=details-fixture&provider=other&externalId=one": 400} {
		response := httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/anime/lookup"+query, nil))
		if response.Code != want {
			t.Fatal("lookup validation", query, response.Code)
		}
	}
	if lookup.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("lookup cache boundary")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("catalog response cache boundary lost")
	}
	for path, want := range map[string]int{"/api/v1/anime/not-a-uuid/details": 400, "/api/v1/anime/00000000-0000-4000-8000-000000000000/details": 404} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != want {
			t.Fatal(path, w.Code)
		}
	}
	out, ok, err := db.Repository("").CatalogDetails(ctx, "10000000-0000-4000-8000-000000000001")
	if err != nil || !ok || out.Provider != "" || out.ExternalIDs == nil || len(out.ExternalIDs) != 0 || out.Genres == nil || out.Themes == nil || out.Demographics == nil || out.Episodes == nil || out.Relations == nil {
		t.Fatal("seed details should have empty arrays", err)
	}
}

func TestCatalogContinuityDetailsDeduplicateReciprocalRelations(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := owner.Exec(ctx, `INSERT INTO data_sources(provider_key,display_name,enabled,policy_url) VALUES('continuity-details','Synthetic continuity',true,'https://example.test/policy')`); err != nil {
		t.Fatal(err)
	}
	snapshot := feed.Snapshot{Version: 1, Provider: "continuity-details", ObservedAt: now, Titles: []feed.Title{
		{ExternalID: "one", Title: "Synthetic Continuity One", MediaType: "TV", Status: "FINISHED", Relations: []feed.Relation{{ExternalID: "two", Type: "SEQUEL"}, {ExternalID: "side", Type: "SIDE_STORY"}}},
		{ExternalID: "two", Title: "Synthetic Continuity Two", MediaType: "MOVIE", Status: "ANNOUNCED", Relations: []feed.Relation{{ExternalID: "one", Type: "PREQUEL"}}},
		{ExternalID: "side", Title: "Synthetic Side Story", MediaType: "OVA", Status: "UNKNOWN"},
	}}
	if _, err := catalogsync.Import(ctx, owner, snapshot, now); err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, key := range []string{"one", "two", "side"} {
		var id string
		if err := owner.QueryRow(ctx, `SELECT anime_id::text FROM anime_external_ids WHERE provider='continuity-details' AND external_id=$1`, key).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[key] = id
	}
	for _, fixture := range []struct {
		key   string
		count int
	}{{"one", 2}, {"two", 1}, {"side", 0}} {
		details, ok, err := db.Repository("").CatalogDetails(ctx, ids[fixture.key])
		if err != nil || !ok || len(details.Relations) != fixture.count {
			t.Fatal("reciprocal duplicate or invented side-story inverse", fixture, details, err)
		}
		if fixture.key == "one" && (details.Relations[0].AnimeID != ids["two"] || details.Relations[0].Type != "SEQUEL" || details.Relations[0].MediaType != "MOVIE") {
			t.Fatal("named related media", details.Relations)
		}
	}
}
