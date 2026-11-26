package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

func (r *Repository) CatalogDetails(ctx context.Context, id string) (store.CatalogDetails, bool, error) {
	out := store.EmptyCatalogDetails(store.Anime{})
	tx, err := r.db.beginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, false, err
	}
	defer rollback(tx)
	a := &out.Anime
	err = tx.QueryRow(ctx, `SELECT `+animeColumns+` FROM anime WHERE id=`+canonicalAnimeID, id).Scan(&a.ID, &a.Title, &a.NativeTitle, &a.MediaType, &a.ReleaseYear, &a.Status, &a.EpisodeCount, &a.Synopsis, &a.Season, &a.CoverImageURL, &a.EpisodeCountKnown)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, false, nil
	}
	if err != nil {
		return out, false, err
	}
	id = a.ID
	var payload []byte
	err = tx.QueryRow(ctx, `SELECT s.provider_key,m.observed_at,m.payload FROM catalog_metadata m JOIN data_sources s ON s.id=m.source_id WHERE m.anime_id=$1`, id).Scan(&out.Provider, &out.ObservedAt, &payload)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, false, err
	}
	if len(payload) > 0 {
		var metadata struct {
			SeasonNumber     int32                 `json:"seasonNumber"`
			PartNumber       int32                 `json:"partNumber"`
			Synonyms         []string              `json:"synonyms"`
			Genres           []string              `json:"genres"`
			Tags             []string              `json:"tags"`
			Themes           []string              `json:"themes"`
			Demographics     []string              `json:"demographics"`
			Studios          []string              `json:"studios"`
			StaffCredits     []store.CatalogCredit `json:"staffCredits"`
			CharacterCredits []json.RawMessage     `json:"characterCredits"`
		}
		if err = json.Unmarshal(payload, &metadata); err != nil {
			return out, false, err
		}
		if metadata.SeasonNumber > 0 {
			out.SeasonNumber = metadata.SeasonNumber
			if metadata.PartNumber > 0 {
				out.PartNumber = metadata.PartNumber
			}
		}
		if metadata.Synonyms != nil {
			out.Synonyms = metadata.Synonyms
		}
		if metadata.Genres != nil {
			out.Genres = metadata.Genres
		}
		if metadata.Tags != nil {
			out.Tags = metadata.Tags
		}
		if metadata.Themes != nil {
			out.Themes = metadata.Themes
		}
		if metadata.Demographics != nil {
			out.Demographics = metadata.Demographics
		}
		if metadata.Studios != nil {
			out.Studios = metadata.Studios
		}
		out.StaffCredits = metadata.StaffCredits
		out.HasCharacterCredits = len(metadata.CharacterCredits) > 0
	}
	sources, err := tx.Query(ctx, `SELECT c.field,c.mode,c.revision,COALESCE(s.provider_key,''),c.observed_at
 FROM catalog_field_choices c LEFT JOIN data_sources s ON s.id=c.source_id WHERE c.anime_id=$1 ORDER BY c.field`, id)
	if err != nil {
		return out, false, err
	}
	for sources.Next() {
		var source store.CatalogFieldSource
		if err = sources.Scan(&source.Field, &source.Mode, &source.Revision, &source.Provider, &source.ObservedAt); err != nil {
			sources.Close()
			return out, false, err
		}
		if source.Mode == "BASE" {
			source.Provider = out.Provider
			source.ObservedAt = out.ObservedAt
		}
		out.FieldSources = append(out.FieldSources, source)
	}
	sources.Close()
	if err = sources.Err(); err != nil {
		return out, false, err
	}
	identities, err := tx.Query(ctx, `SELECT provider,external_id FROM anime_external_ids WHERE anime_id=$1 ORDER BY provider,external_id`, id)
	if err != nil {
		return out, false, err
	}
	for identities.Next() {
		var identity store.CatalogExternalID
		if err = identities.Scan(&identity.Provider, &identity.ExternalID); err != nil {
			identities.Close()
			return out, false, err
		}
		out.ExternalIDs = append(out.ExternalIDs, identity)
	}
	identities.Close()
	if err = identities.Err(); err != nil {
		return out, false, err
	}
	rows, err := tx.Query(ctx, `WITH linked(anime_id,relation_type) AS (
SELECT target_anime_id,relation_type FROM anime_relations WHERE source_anime_id=$1
UNION
SELECT source_anime_id,CASE relation_type WHEN 'PREQUEL' THEN 'SEQUEL' ELSE 'PREQUEL' END
FROM anime_relations WHERE target_anime_id=$1 AND relation_type IN ('PREQUEL','SEQUEL')
)
SELECT a.id::text,l.relation_type,a.canonical_title,a.media_type,a.status
FROM linked l JOIN anime a ON a.id=l.anime_id ORDER BY l.relation_type,a.canonical_title,a.id`, id)
	if err != nil {
		return out, false, err
	}
	for rows.Next() {
		var relation store.CatalogRelation
		if err = rows.Scan(&relation.AnimeID, &relation.Type, &relation.Title, &relation.MediaType, &relation.Status); err != nil {
			rows.Close()
			return out, false, err
		}
		out.Relations = append(out.Relations, relation)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, false, err
	}
	rows, err = tx.Query(ctx, `SELECT DISTINCT ON (a.episode_number,a.release_track) a.episode_number,a.release_track,a.scheduled_at,a.actual_released_at,a.last_verified_at,a.revision,COALESCE(s.provider_key,'') FROM airing_schedule a LEFT JOIN data_sources s ON s.id=a.source_id WHERE a.anime_id=$1 ORDER BY a.episode_number,a.release_track,a.revision DESC`, id)
	if err != nil {
		return out, false, err
	}
	for rows.Next() {
		var episode store.CatalogEpisode
		if err = rows.Scan(&episode.Number, &episode.Track, &episode.ScheduledAt, &episode.ReleasedAt, &episode.VerifiedAt, &episode.Revision, &episode.Provider); err != nil {
			rows.Close()
			return out, false, err
		}
		out.Episodes = append(out.Episodes, episode)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, false, err
	}
	return out, true, tx.Commit(ctx)
}
