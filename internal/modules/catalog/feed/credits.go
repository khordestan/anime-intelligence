package feed

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type Credit struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

func validCredits(credits []Credit) bool {
	if len(credits) > 128 {
		return false
	}
	seen := make(map[[2]string]bool, len(credits))
	for _, credit := range credits {
		if !validCreditLabel(credit.Name) || !validCreditLabel(credit.Role) {
			return false
		}
		key := [2]string{strings.ToLower(credit.Name), strings.ToLower(credit.Role)}
		if seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}

func validCreditLabel(label string) bool {
	return label != "" && len(label) <= 256 && utf8.ValidString(label) &&
		label == strings.TrimSpace(label) && strings.IndexFunc(label, unicode.IsControl) < 0
}
