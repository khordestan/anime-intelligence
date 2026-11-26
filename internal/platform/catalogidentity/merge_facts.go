package catalogidentity

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type MergePublicFact struct {
	Field      string  `json:"field"`
	Source     *string `json:"source"`
	Target     *string `json:"target"`
	Comparison string  `json:"comparison"`
}

func inspectMergePublicFacts(ctx context.Context, tx pgx.Tx, source, target pgtype.UUID) ([]MergePublicFact, error) {
	rows, err := tx.Query(ctx, `SELECT fact.field,fact.source,fact.target
	FROM public.anime a CROSS JOIN public.anime b
	LEFT JOIN public.catalog_metadata am ON am.anime_id=a.id
	LEFT JOIN public.catalog_metadata bm ON bm.anime_id=b.id
	LEFT JOIN public.data_sources aps ON aps.id=am.source_id
	LEFT JOIN public.data_sources bps ON bps.id=bm.source_id
	CROSS JOIN LATERAL (VALUES
		('canonicalTitle',a.canonical_title,b.canonical_title),
		('nativeTitle',NULLIF(a.native_title,''),NULLIF(b.native_title,'')),
		('mediaType',NULLIF(a.media_type,'UNKNOWN'),NULLIF(b.media_type,'UNKNOWN')),
		('status',NULLIF(a.status,'UNKNOWN'),NULLIF(b.status,'UNKNOWN')),
		('releaseYear',a.release_year::text,b.release_year::text),
		('releaseQuarter',a.season_name,b.season_name),
		('episodeCount',a.episode_count::text,b.episode_count::text),
		('primaryProvider',aps.provider_key,bps.provider_key)
	) AS fact(field,source,target)
	WHERE a.id=$1 AND b.id=$2 ORDER BY fact.field`, source, target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	facts := make([]MergePublicFact, 0, 8)
	for rows.Next() {
		var fact MergePublicFact
		if err := rows.Scan(&fact.Field, &fact.Source, &fact.Target); err != nil {
			return nil, err
		}
		switch {
		case fact.Source == nil && fact.Target == nil:
			fact.Comparison = "UNKNOWN"
		case fact.Source == nil:
			fact.Comparison = "TARGET_ONLY"
		case fact.Target == nil:
			fact.Comparison = "SOURCE_ONLY"
		case *fact.Source == *fact.Target:
			fact.Comparison = "EQUAL"
		default:
			fact.Comparison = "DIFFERENT"
		}
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}
