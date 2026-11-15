package feed

import "testing"

func TestContinuityValidation(t *testing.T) {
	s:=fixture()
	second:=Title{ExternalID:"season-2",Title:"Synthetic Sky Two",MediaType:"TV",Status:"ANNOUNCED",Relations:[]Relation{{ExternalID:"season-1",Type:"PREQUEL"}}}
	s.Titles=append(s.Titles,second)
	if s.Validate(clock)!=nil {t.Fatal("reciprocal sequel/prequel assertions should agree")}
	s.Titles[1].Relations[0].Type="SEQUEL"
	if s.Validate(clock)==nil {t.Fatal("two-node chronology cycle accepted")}
	s.Titles[1].Relations=[]Relation{{ExternalID:"season-3",Type:"SEQUEL"}}
	s.Titles=append(s.Titles,Title{ExternalID:"season-3",Title:"Synthetic Sky Three",MediaType:"TV",Status:"ANNOUNCED",Relations:[]Relation{{ExternalID:"season-1",Type:"SEQUEL"}}})
	if s.Validate(clock)==nil {t.Fatal("three-node chronology cycle accepted")}
	s.Titles[2].Relations[0].Type="SIDE_STORY"
	if s.Validate(clock)!=nil {t.Fatal("side stories must not imply chronology")}
	s.Titles=s.Titles[:1]
	if s.Validate(clock)!=nil {t.Fatal("partial graph with external target should remain valid")}
}
