package intelligence

import "testing"

func TestBuildTasteProfileSeparatesExplicitAndInferred(t *testing.T) {
	rated := []RatedAnime{
		{AnimeID:"watched-a",Score:5,Features:[]FeatureValue{{Key:"power.progression",Value:1,Confidence:1}}},
		{AnimeID:"watched-b",Score:1,Features:[]FeatureValue{{Key:"slow.pacing",Value:1,Confidence:1}}},
	}
	profile:=BuildTasteProfile(rated,map[string]Preference{"narrative.mystery":{Weight:0.9,Confidence:1}})
	if profile.Explicit["narrative.mystery"].Weight!=0.9 { t.Fatal("explicit preference was not retained") }
	if profile.Inferred["power.progression"].Weight<=0 { t.Fatal("high-rated feature should be positive") }
	if profile.Inferred["slow.pacing"].Weight>=0 { t.Fatal("low-rated feature should be negative") }
}

func TestRankPersonalMatchesExplainsAndExcludes(t *testing.T) {
	profile:=TasteProfile{
		Explicit:map[string]Preference{"power.progression":{Weight:1,Confidence:1},"slow.pacing":{Weight:-0.8,Confidence:1}},
	}
	candidates:=[]Candidate{
		{AnimeID:"a",Title:"Progression",Features:[]FeatureValue{{Key:"power.progression",Value:1,Confidence:1}}},
		{AnimeID:"b",Title:"Slow burn",Features:[]FeatureValue{{Key:"slow.pacing",Value:1,Confidence:1}}},
		{AnimeID:"c",Title:"Mixed",Features:[]FeatureValue{{Key:"power.progression",Value:0.5,Confidence:1},{Key:"slow.pacing",Value:0.5,Confidence:1}}},
		{AnimeID:"a",Title:"Duplicate",Features:[]FeatureValue{{Key:"power.progression",Value:1,Confidence:1}}},
	}
	got:=RankPersonalMatches(profile,candidates,map[string]struct{}{"c":{}},Options{Limit:10})
	if len(got)!=2 { t.Fatalf("want 2 recommendations, got %d",len(got)) }
	if got[0].AnimeID!="a" || got[1].AnimeID!="b" { t.Fatalf("unexpected ranking: %+v",got) }
	if len(got[0].Evidence)==0 || got[0].Evidence[0].Direction!="match" { t.Fatalf("missing match evidence: %+v",got[0].Evidence) }
	if got[0].AlgorithmVersion!=PersonalMatchAlgorithmVersion { t.Fatal("algorithm version missing") }
}

func TestRankReturnsEmptyWithoutProfileEvidence(t *testing.T) {
	got:=RankPersonalMatches(TasteProfile{},[]Candidate{{AnimeID:"a",Title:"A",Features:[]FeatureValue{{Key:"x",Value:1,Confidence:1}}}},nil,Options{})
	if len(got)!=0 { t.Fatalf("expected no unsupported recommendations, got %+v",got) }
}

func TestStableTiesUseFeatureKeysAndCanonicalIDs(t *testing.T){
	p:=TasteProfile{Explicit:map[string]Preference{"a":{Weight:1,Confidence:1},"b":{Weight:1,Confidence:1}}}
	got:=RankPersonalMatches(p,[]Candidate{
		{AnimeID:"z",Title:"Same",Features:[]FeatureValue{{Key:"b",Value:1,Confidence:1},{Key:"a",Value:1,Confidence:1}}},
		{AnimeID:"a",Title:"Same",Features:[]FeatureValue{{Key:"b",Value:1,Confidence:1},{Key:"a",Value:1,Confidence:1}}},
	},nil,Options{})
	if len(got)!=2||got[0].AnimeID!="a"||got[0].Evidence[0].FeatureKey!="a"{t.Fatal("unstable ties",got)}
}

func TestPersonalMatchFeatureKeysExcludeNeutralSignals(t *testing.T) {
	profile := TasteProfile{
		Explicit: map[string]Preference{
			"neutral": {Weight: 0, Confidence: 1},
			"mixed": {Weight: 0, Confidence: 1},
			"unreliable": {Weight: 1, Confidence: 0},
		},
		Inferred: map[string]Preference{
			"mixed": {Weight: 1, Confidence: 1},
			"inferred": {Weight: -1, Confidence: 1},
		},
	}
	keys := PersonalMatchFeatureKeys(profile)
	if len(keys) != 2 || keys[0] != "inferred" || keys[1] != "mixed" {
		t.Fatalf("scoring feature keys = %v", keys)
	}
}
