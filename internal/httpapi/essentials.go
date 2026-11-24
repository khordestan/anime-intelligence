package httpapi

import (
	"net/http"

	"github.com/khordestan/anime-intelligence/internal/modules/intelligence"
)

func registerGlobalEssentials(mux *http.ServeMux, ranked []intelligence.EssentialsMatch) {
	mux.HandleFunc("GET /api/v1/recommendations/essentials", func(w http.ResponseWriter, _ *http.Request) {
		if len(ranked) == 0 {
			authError(w, http.StatusNotFound, "global_essentials_unavailable")
			return
		}
		writeJSON(w, http.StatusOK, struct {
			AlgorithmVersion string                         `json:"algorithmVersion"`
			Items            []intelligence.EssentialsMatch `json:"items"`
		}{AlgorithmVersion: intelligence.GlobalEssentialsAlgorithmVersion, Items: ranked})
	})
}
