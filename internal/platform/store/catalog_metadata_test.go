package store

import "testing"

func TestStarterEpisodeCountsAreKnown(t *testing.T) {
	s,err:=New("")
	if err!=nil {t.Fatal(err)}
	items:=s.Catalog("",100,0)
	if len(items)==0 {t.Fatal("missing starter catalog")}
	for _,a:=range items {
		if !a.EpisodeCountKnown || a.EpisodeCount<=0 {t.Fatal("starter total lost known state")}
	}
}
