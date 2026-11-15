package feed

// validateContinuity treats PREQUEL as the reverse of SEQUEL. Reciprocal
// assertions agree; a cycle would make chronology contradictory. Other
// relation types deliberately carry no implied viewing order.
func validateContinuity(titles []Title) error {
	edges := map[string][]string{}
	for _, t := range titles {
		for _, r := range t.Relations {
			switch r.Type {
			case "SEQUEL":
				edges[t.ExternalID] = append(edges[t.ExternalID],r.ExternalID)
			case "PREQUEL":
				edges[r.ExternalID] = append(edges[r.ExternalID],t.ExternalID)
			}
		}
	}
	state := map[string]uint8{}
	var visit func(string) bool
	visit = func(id string) bool {
		if state[id] == 1 { return false }
		if state[id] == 2 { return true }
		state[id] = 1
		for _, target := range edges[id] {
			if !visit(target) { return false }
		}
		state[id] = 2
		return true
	}
	for _, t := range titles {
		if !visit(t.ExternalID) { return ErrInvalid }
	}
	return nil
}
