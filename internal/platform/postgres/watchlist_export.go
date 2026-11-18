package postgres

import (
 "context"
 "time"
)

type WatchListExport struct {
 Version int `json:"version"`
 Entries []WatchListExportEntry `json:"entries"`
}

type WatchListExportEntry struct {
 AnimeID string `json:"animeId"`
 Title string `json:"title"`
 WatchStatus string `json:"watchStatus"`
 EpisodesWatched int `json:"episodesWatched"`
 WatchedEpisodeNumbers []int32 `json:"watchedEpisodeNumbers"`
 Rating *int `json:"rating"`
}

// ExportWatchList reads one consistent projection of current tracker state.
// Account data and completed viewing passes belong to the full account export.
func (r *Repository) ExportWatchList(ctx context.Context) (WatchListExport, error) {
 ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
 defer cancel()
 tx, owner, err := r.db.productTx(ctx, r.token)
 if err != nil {
  return WatchListExport{}, err
 }
 defer rollback(tx)

 rows, err := tx.Query(ctx, `SELECT ua.anime_id::text, a.canonical_title, ua.watch_status,
  COALESCE(wp.episodes_watched, 0), wp.watched_episode_numbers, ur.score
 FROM user_anime ua
 JOIN anime a ON a.id=ua.anime_id
 LEFT JOIN watch_progress wp ON wp.user_id=ua.user_id AND wp.anime_id=ua.anime_id
 LEFT JOIN user_ratings ur ON ur.user_id=ua.user_id AND ur.anime_id=ua.anime_id
  AND ur.scope='SERIES' AND ur.season_key IS NULL
 WHERE ua.user_id=$1 ORDER BY ua.anime_id`, owner.ID)
 if err != nil {
  return WatchListExport{}, err
 }
 defer rows.Close()

 export := WatchListExport{Version: 1, Entries: []WatchListExportEntry{}}
 for rows.Next() {
  var entry WatchListExportEntry
  if err := rows.Scan(&entry.AnimeID, &entry.Title, &entry.WatchStatus, &entry.EpisodesWatched, &entry.WatchedEpisodeNumbers, &entry.Rating); err != nil {
   return WatchListExport{}, err
  }
  export.Entries = append(export.Entries, entry)
 }
 if err := rows.Err(); err != nil {
  return WatchListExport{}, err
 }
 if err := tx.Commit(ctx); err != nil {
  return WatchListExport{}, err
 }
 return export, nil
}
