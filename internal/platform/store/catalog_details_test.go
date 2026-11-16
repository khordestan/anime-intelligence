package store

import (
	"context"
	"testing"
)
func TestLocalCatalogDetailsEmptyEnrichment(t *testing.T) {
	s,err:=New("")
	if err!=nil {t.Fatal(err)}
	local:=Local{Store:s}
	details,ok,err:=local.CatalogDetails(context.Background(),"10000000-0000-4000-8000-000000000001")
	if err!=nil || !ok || details.ExternalIDs==nil || len(details.ExternalIDs)!=0 || details.Genres==nil || details.Themes==nil || details.Demographics==nil || details.Episodes==nil || !details.Anime.EpisodeCountKnown {t.Fatal("local details contract",err)}
	if _,ok,err:=local.CatalogDetails(context.Background(),"missing");err!=nil || ok {t.Fatal("missing local details")}
}

func TestLocalExternalIdentityLookupHasNoInventedMappings(t *testing.T){
 s,err:=New("");if err!=nil{t.Fatal(err)}
 if _,ok,err:=(Local{Store:s}).GetAnimeByExternalID(context.Background(),"synthetic","one");err!=nil||ok{t.Fatal("invented local identity",err)}
}
