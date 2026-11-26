package feed

import (
	"strings"
	"testing"
)

func TestReviewedCreditValidationRejectsAmbiguousRecords(t *testing.T) {
	valid := []Credit{{Name: "Synthetic One", Role: "Director"}, {Name: "Synthetic One", Role: "Writer"}}
	if !validCredits(valid) {
		t.Fatal("different roles for the same name must be allowed")
	}
	for _, credits := range [][]Credit{
		{{Name: " Name", Role: "Director"}},
		{{Name: "Name", Role: ""}},
		{{Name: "Name", Role: "With\x00control"}},
		{{Name: strings.Repeat("é", 129), Role: "Role"}},
		{{Name: "Name", Role: "Role"}, {Name: "name", Role: "role"}},
		make([]Credit, 129),
	} {
		if validCredits(credits) {
			t.Fatalf("accepted invalid credit list of %d entries", len(credits))
		}
	}
}
