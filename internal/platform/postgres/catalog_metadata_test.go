package postgres_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func TestPublicCatalogMetadata(t *testing.T) {
	db, owner, _, _ := testDB(t)
	ctx := context.Background()
	const id = "10000000-0000-4000-8000-000000000001"
	if _,err:=owner.Exec(ctx,`UPDATE anime SET synopsis='A synthetic catalog synopsis.',season_name='WINTER',cover_image_url='https://example.test/synthetic.png',episode_count=NULL WHERE id=$1`,id);err!=nil {t.Fatal(err)}
	// Public reads use the restricted runtime with no account/session context.
	repo:=db.Repository("")
	a,ok,err:=repo.GetAnime(ctx,id)
	if err!=nil || !ok || a.Synopsis!="A synthetic catalog synopsis." || a.Season!="WINTER" || a.CoverImageURL!="https://example.test/synthetic.png" || a.EpisodeCountKnown || a.EpisodeCount!=0 {t.Fatalf("detail metadata: %+v %v",a,err)}
	items,err:=repo.Catalog(ctx,"Solo Leveling",24,0)
	if err!=nil || len(items)!=1 || items[0]!=a {t.Fatalf("list/detail mismatch: %+v %v",items,err)}
	h:=httpapi.NewPostgresRouter(slog.New(slog.NewTextHandler(io.Discard,nil)),db)
	w:=httptest.NewRecorder()
	h.ServeHTTP(w,httptest.NewRequest("GET","/api/v1/anime/"+id,nil))
	var body store.Anime
	if w.Code!=200 || json.Unmarshal(w.Body.Bytes(),&body)!=nil || body!=a {t.Fatalf("public metadata response: %d %s",w.Code,w.Body.String())}
	if _,err=owner.Exec(ctx,`UPDATE anime SET synopsis=NULL,season_name=NULL,cover_image_url=NULL,episode_count=0 WHERE id=$1`,id);err!=nil {t.Fatal(err)}
	a,ok,err=repo.GetAnime(ctx,id)
	if err!=nil || !ok || a.Synopsis!="" || a.Season!="" || a.CoverImageURL!="" || !a.EpisodeCountKnown || a.EpisodeCount!=0 {t.Fatalf("nullable fields/known zero: %+v %v",a,err)}
	if _,ok,err=repo.GetAnime(ctx,"00000000-0000-4000-8000-000000000000");err!=nil || ok {t.Fatal("missing title semantics changed",err)}
}
