package postgres

import (
 "context"
 "sort"
 "strings"
 "time"

 "github.com/khordestan/anime-intelligence/internal/platform/store"
)

// UpdateStatuses applies status-only edits in one verified-owner transaction.
// A missing/stale target rolls back every preceding edit.
func (r *Repository) UpdateStatuses(ctx context.Context, status string, targets []store.BulkStatusTarget) ([]store.Entry, bool, error) {
 if len(targets) == 0 || len(targets) > store.MaxBulkStatusEntries {
  return nil, false, store.ErrBulkStatusInvalid
 }
 switch status {
 case "PLANNING", "WATCHING", "COMPLETED", "ON_HOLD", "DROPPED":
 default:
  return nil, false, store.ErrBulkStatusInvalid
 }
 byID := make(map[string]int, len(targets))
 ids := make([]string, 0, len(targets))
 for index, target := range targets {
  id := strings.ToLower(target.ID)
  if _, exists := byID[id]; exists || target.ExpectedCycle < 1 || target.ExpectedCycle > 1000 {
   return nil, false, store.ErrBulkStatusInvalid
  }
  byID[id] = index
  ids = append(ids, id)
 }
 // Lock tracker entries in the same order across overlapping batches.
 sort.Strings(ids)
 ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
 defer cancel()
 tx, owner, err := r.db.productTx(ctx, r.token)
 if err != nil {
  return nil, false, err
 }
 defer rollback(tx)
 items := make([]store.Entry, len(targets))
 for _, id := range ids {
  index := byID[id]
  entry, found, err := updateCycleEntry(ctx, tx, owner.ID, id, &status, nil, targets[index].ExpectedCycle)
  if err != nil || !found {
   return nil, found, err
  }
  items[index] = entry
 }
 if err := tx.Commit(ctx); err != nil {
  return nil, false, err
 }
 return items, true, nil
}
