package httpapi

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
)

type archivedRatingFact struct {
	Rated   int      `json:"rated"`
	Average *float64 `json:"average"`
}

type libraryAnalysis struct {
	Engine         string              `json:"engine"`
	TopicMatched   bool                `json:"topicMatched"`
	FromMonth      string              `json:"fromMonth,omitempty"`
	ThroughMonth   string              `json:"throughMonth,omitempty"`
	ArchivedPasses *int                `json:"archivedPasses,omitempty"`
	ArchiveRatings *archivedRatingFact `json:"archiveRatings,omitempty"`
}

func privateLibraryAnalysis(db *postgres.DB) func(http.ResponseWriter, *http.Request, string, postgres.User) {
	return func(w http.ResponseWriter, r *http.Request, token string, user postgres.User) {
		if !user.EmailVerified {
			authError(w, http.StatusForbidden, "email_verification_required")
			return
		}
		var request struct {
			Question string `json:"question"`
		}
		if decodeAuth(w, r, &request) != nil || strings.TrimSpace(request.Question) == "" || len(request.Question) > 400 {
			authError(w, http.StatusBadRequest, "invalid_library_question")
			return
		}
		passes, ratings := libraryAnalysisTopics(request.Question)
		out := libraryAnalysis{Engine: "library-history-v1", TopicMatched: passes || ratings}
		if !out.TopicMatched {
			writeJSON(w, http.StatusOK, out)
			return
		}
		months, err := db.Repository(token).ViewingHistorySummary(r.Context())
		if err != nil {
			rewatchError(w, err)
			return
		}
		out.FromMonth, out.ThroughMonth = months[len(months)-1].Month, months[0].Month
		var archived, rated, ratingTotal int
		for _, month := range months {
			archived += month.Archived
			rated += month.Rated
			ratingTotal += month.RatingTotal
		}
		if passes {
			out.ArchivedPasses = &archived
		}
		if ratings {
			out.ArchiveRatings = &archivedRatingFact{Rated: rated}
			if rated > 0 {
				average := float64(ratingTotal) / float64(rated)
				out.ArchiveRatings.Average = &average
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func libraryAnalysisTopics(question string) (passes, ratings bool) {
	for _, word := range strings.FieldsFunc(strings.ToLower(question), func(r rune) bool { return !unicode.IsLetter(r) }) {
		switch word {
		case "rewatch", "rewatches", "rewatched", "pass", "passes", "viewing", "history":
			passes = true
		case "rating", "ratings", "rated", "score", "scores":
			ratings = true
		}
	}
	return passes, ratings
}
