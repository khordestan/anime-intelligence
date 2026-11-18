package catalog

import (
 "crypto/sha256"
 "encoding/base64"
 "encoding/json"
 "net/http"
 "strings"
 "unicode/utf8"

 "github.com/khordestan/anime-intelligence/internal/platform/store"
)

type catalogCursor struct {
 Version int
 Filter [sha256.Size]byte
 Position store.CatalogPosition
}

func (h *Handler) listAfter(w http.ResponseWriter, r *http.Request, filter store.CatalogFilter, limit int) {
 query := r.URL.Query()
 values := query["cursor"]
 _, hasOffset := query["offset"]
 invalid := func() { writeError(w, 400, "invalid_catalog_cursor", "Check the cursor and catalog filters") }
 if hasOffset || len(values) != 1 || values[0] == "" || len(values[0]) > 16384 {
  invalid()
  return
 }
 // Treat explicit TITLE and the default as the same ordering.
 if filter.Sort == "" {
  filter.Sort = "TITLE"
 }
 filterJSON, _ := json.Marshal(filter)
 digest := sha256.Sum256(filterJSON)
 var after *store.CatalogPosition
 if values[0] != "start" {
  raw, err := base64.RawURLEncoding.Strict().DecodeString(values[0])
  var token catalogCursor
  if err != nil || json.Unmarshal(raw, &token) != nil || token.Version != 1 || token.Filter != digest ||
   !validUUID(token.Position.ID) || !utf8.ValidString(token.Position.Title) || strings.ContainsRune(token.Position.Title, 0) {
   invalid()
   return
  }
  after = &token.Position
 }
 data, ok := h.data.(store.CursorCatalog)
 if !ok {
  writeError(w, 503, "catalog_unavailable", "Catalog pagination is unavailable")
  return
 }
 items, err := data.CatalogAfter(r.Context(), filter, limit+1, after)
 if err != nil {
  writeError(w, 500, "catalog_read_failed", "Could not read catalog")
  return
 }
 response := map[string]any{"limit": limit}
 if len(items) > limit {
  items = items[:limit]
  token, _ := json.Marshal(catalogCursor{Version: 1, Filter: digest, Position: filter.Position(items[limit-1])})
  response["nextCursor"] = base64.RawURLEncoding.EncodeToString(token)
 }
 response["items"] = items
 response["count"] = len(items)
 writeJSON(w, 200, response)
}
