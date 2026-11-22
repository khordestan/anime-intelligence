package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func importWatchList(w http.ResponseWriter, r *http.Request, repo *postgres.Repository, preview bool) {
	var data postgres.WatchListExport
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&data) != nil || decoder.Decode(new(any)) != io.EOF {
		authError(w, 400, "invalid_watch_list")
		return
	}
	process := repo.ImportWatchList
	if preview {
		process = repo.PreviewWatchList
	}
	result, err := process(r.Context(), data)
	switch {
	case errors.Is(err, store.ErrUnauthorized):
		authError(w, 401, "unauthorized")
	case errors.Is(err, store.ErrEmailUnverified):
		authError(w, 403, "email_verification_required")
	case errors.Is(err, postgres.ErrWatchListImport):
		authError(w, 400, "invalid_watch_list")
	case errors.Is(err, postgres.ErrWatchListCatalog):
		authError(w, 422, "watch_list_catalog_mismatch")
	case err != nil:
		authError(w, 500, "watch_list_import_failed")
	default:
		writeJSON(w, 200, result)
	}
}
