package postgres_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestCatalogAnalystCuratedFeatureEligibilityAndBound(t *testing.T) {
	db, owner, _, _ := testDB(t)
	const anime = "10000000-0000-4000-8000-000000000001"
	now := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Minute)
	queueExec(t, owner, `INSERT INTO anime_features(feature_key,display_name,category)
		SELECT 'synthetic.extra.'||n,'Synthetic extra '||n,'synthetic' FROM generate_series(1,21) n`)
	queueExec(t, owner, `INSERT INTO anime_feature_observations(anime_id,feature_id,value,confidence,source_kind,evidence,observed_at)
		SELECT $1,id,0.5,0.5,'CURATED','{"reference":"https://example.test/secret-reference"}',$2
		FROM anime_features WHERE feature_key LIKE 'synthetic.extra.%'`, anime, now)
	queueExec(t, owner, `INSERT INTO anime_features(feature_key,display_name,category,spoiler_sensitive,active)
		VALUES ('synthetic.reviewed','Synthetic reviewed','synthetic',false,true),
		       ('synthetic.model','Hidden model feature','synthetic',false,true),
		       ('synthetic.spoiler','Hidden ending feature','synthetic',true,true),
		       ('synthetic.inactive','Hidden inactive feature','synthetic',false,false),
		       ('synthetic.retracted','Hidden removed feature','synthetic',false,true),
		       ('synthetic.unreferenced','Hidden unreferenced feature','synthetic',false,true),
		       ('synthetic.future','Hidden future feature','synthetic',false,true)`)
	queueExec(t, owner, `INSERT INTO anime_feature_observations(anime_id,feature_id,value,confidence,source_kind,evidence,observed_at)
		SELECT $1,f.id, v.value, v.confidence, v.kind, jsonb_build_object('reference', v.reference, 'review', 'Hidden prose'),
			$2::timestamptz+v.offset_minutes*interval '1 minute'
		FROM (VALUES
			('synthetic.reviewed',1,0.8,'CURATED','https://example.test/secret-reference',-5),
			('synthetic.reviewed',1,0.9,'CURATED','https://example.test/secret-reference',0),
			('synthetic.model',1,1,'MODEL','https://example.test/secret-reference',0),
			('synthetic.spoiler',1,1,'CURATED','https://example.test/secret-reference',0),
			('synthetic.inactive',1,1,'CURATED','https://example.test/secret-reference',0),
			('synthetic.retracted',1,1,'CURATED','https://example.test/secret-reference',-5),
			('synthetic.retracted',0,1,'CURATED','https://example.test/secret-reference',0),
			('synthetic.unreferenced',1,1,'CURATED','',0),
			('synthetic.future',1,1,'CURATED','https://example.test/secret-reference',5)
		) v(key,value,confidence,kind,reference,offset_minutes)
		JOIN anime_features f ON f.feature_key=v.key`, anime, now)

	h := httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/analysis/catalog",
		strings.NewReader(`{"topics":["curatedFeatures"],"animeIds":["`+anime+`"]}`)))
	var response struct {
		Items []struct {
			Evidence []struct {
				Field           string
				Truncated       bool
				CuratedFeatures []store.CuratedCatalogFeature
			}
		}
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil ||
		len(response.Items) != 1 || len(response.Items[0].Evidence) != 1 {
		t.Fatalf("unexpected curated facts response: %d %s", w.Code, w.Body)
	}
	fact := response.Items[0].Evidence[0]
	if !fact.Truncated || fact.Field != "curatedFeatures" || len(fact.CuratedFeatures) != 20 ||
		fact.CuratedFeatures[0].Key != "synthetic.reviewed" || fact.CuratedFeatures[0].Confidence != 0.9 ||
		!fact.CuratedFeatures[0].ObservedAt.Equal(now) {
		t.Fatalf("unexpected curated evidence: %+v", fact)
	}
	for _, blocked := range []string{"Hidden", "secret-reference", "synthetic.retracted", "synthetic.unreferenced"} {
		if strings.Contains(w.Body.String(), blocked) {
			t.Fatalf("excluded feature or source details in analyst response: %q", blocked)
		}
	}
}
