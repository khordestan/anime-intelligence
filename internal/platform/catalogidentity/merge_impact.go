package catalogidentity

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrMergeImpact = errors.New("could not inspect canonical merge impact")

type MergeImpactCount struct {
	Dependency string `json:"dependency"`
	Source     int64  `json:"source"`
	Target     int64  `json:"target"`
}

type MergeImpactConflict struct {
	Kind  string `json:"kind"`
	Count int64  `json:"count"`
}

type MergeImpact struct {
	SourceID   pgtype.UUID           `json:"sourceId"`
	TargetID   pgtype.UUID           `json:"targetId"`
	References []MergeImpactCount    `json:"references"`
	Conflicts  []MergeImpactConflict `json:"conflicts"`
}

// InspectMergeImpact runs against an isolated review database using an operator
// credential able to see complete RLS-protected counts. No owner IDs are read.
func InspectMergeImpact(ctx context.Context, conn *pgx.Conn, source, target pgtype.UUID) (MergeImpact, error) {
	if conn == nil || !source.Valid || !target.Valid || source == target {
		return MergeImpact{}, ErrMergeImpact
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return MergeImpact{}, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	// PostgreSQL raises an error when a filter would hide rows due to RLS.
	if _, err := tx.Exec(ctx, `SET LOCAL search_path=pg_catalog,public,pg_temp;
		SET LOCAL row_security=off; SET LOCAL statement_timeout='5s'`); err != nil {
		return MergeImpact{}, err
	}
	var found int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.anime WHERE id IN ($1,$2)`, source, target).Scan(&found); err != nil {
		return MergeImpact{}, err
	}
	if found != 2 {
		return MergeImpact{}, ErrMergeImpact
	}

	rows, err := tx.Query(ctx, `SELECT n.nspname,c.relname,a.attname,
		p.conkey=ARRAY[a.attnum] AND p.confkey=ARRAY[id.attnum] AS single_id
		FROM pg_constraint p
		JOIN pg_class c ON c.oid=p.conrelid
		JOIN pg_namespace n ON n.oid=c.relnamespace
		JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum=p.conkey[1]
		JOIN pg_attribute id ON id.attrelid=p.confrelid AND id.attname='id'
		WHERE p.contype='f' AND p.confrelid='public.anime'::regclass AND p.conparentid=0
		ORDER BY n.nspname,c.relname,a.attname`)
	if err != nil {
		return MergeImpact{}, err
	}
	type reference struct{ schema, table, column string }
	var references []reference
	for rows.Next() {
		var ref reference
		var singleID bool
		if err := rows.Scan(&ref.schema, &ref.table, &ref.column, &singleID); err != nil {
			rows.Close()
			return MergeImpact{}, err
		}
		if !singleID {
			rows.Close()
			return MergeImpact{}, ErrMergeImpact
		}
		references = append(references, ref)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return MergeImpact{}, err
	}

	result := MergeImpact{
		SourceID: source, TargetID: target,
		References: make([]MergeImpactCount, 0, len(references)+1),
		Conflicts:  []MergeImpactConflict{},
	}
	for _, ref := range references {
		column := pgx.Identifier{ref.column}.Sanitize()
		table := pgx.Identifier{ref.schema, ref.table}.Sanitize()
		query := `SELECT count(*) FILTER (WHERE ` + column + `=$1),
			count(*) FILTER (WHERE ` + column + `=$2) FROM ` + table + ` WHERE ` + column + ` IN ($1,$2)`
		count := MergeImpactCount{Dependency: ref.schema + "." + ref.table + "." + ref.column}
		if err := tx.QueryRow(ctx, query, source, target).Scan(&count.Source, &count.Target); err != nil {
			return MergeImpact{}, err
		}
		result.References = append(result.References, count)
	}
	for _, item := range mergeIndirectQueries {
		count := MergeImpactCount{Dependency: item.dependency}
		if err := tx.QueryRow(ctx, item.query, source, target).Scan(&count.Source, &count.Target); err != nil {
			return MergeImpact{}, err
		}
		result.References = append(result.References, count)
	}

	for _, item := range mergeConflictQueries {
		count := MergeImpactConflict{Kind: item.kind}
		if err := tx.QueryRow(ctx, item.query, source, target).Scan(&count.Count); err != nil {
			return MergeImpact{}, err
		}
		result.Conflicts = append(result.Conflicts, count)
	}
	if err := tx.Commit(ctx); err != nil {
		return MergeImpact{}, err
	}
	return result, nil
}

var mergeIndirectQueries = []struct{ dependency, query string }{
	{"public.domain_events.aggregate_id (ANIME)", `SELECT count(*) FILTER (WHERE aggregate_id=$1),
		count(*) FILTER (WHERE aggregate_id=$2) FROM public.domain_events
		WHERE aggregate_type='ANIME' AND aggregate_id IN ($1,$2)`},
	{"public.notifications.event_id", `SELECT count(*) FILTER (WHERE e.aggregate_id=$1),
		count(*) FILTER (WHERE e.aggregate_id=$2) FROM public.notifications n JOIN public.domain_events e ON e.id=n.event_id
		WHERE e.aggregate_type='ANIME' AND e.aggregate_id IN ($1,$2)`},
	{"public.release_email_jobs.event_id", `SELECT count(*) FILTER (WHERE e.aggregate_id=$1),
		count(*) FILTER (WHERE e.aggregate_id=$2) FROM public.release_email_jobs j JOIN public.domain_events e ON e.id=j.event_id
		WHERE e.aggregate_type='ANIME' AND e.aggregate_id IN ($1,$2)`},
	{"public.catalog_fetch_jobs (provider identity)", `SELECT count(*) FILTER (WHERE i.anime_id=$1),
		count(*) FILTER (WHERE i.anime_id=$2) FROM public.catalog_fetch_jobs j
		JOIN public.data_sources s ON s.id=j.source_id
		JOIN public.anime_external_ids i ON i.provider=s.provider_key AND i.external_id=j.external_id
		WHERE i.anime_id IN ($1,$2)`},
	{"public.source_records (provider identity)", `SELECT count(*) FILTER (WHERE i.anime_id=$1),
		count(*) FILTER (WHERE i.anime_id=$2) FROM public.source_records r
		JOIN public.data_sources s ON s.id=r.source_id
		JOIN public.anime_external_ids i ON i.provider=s.provider_key AND i.external_id=r.external_id
		WHERE i.anime_id IN ($1,$2)`},
}

var mergeConflictQueries = []struct{ kind, query string }{
	{"shared_provider_mapping", `SELECT count(*) FROM public.anime_external_ids a JOIN public.anime_external_ids b USING(provider)
		WHERE a.anime_id=$1 AND b.anime_id=$2`},
	{"episode_track_overlap", `SELECT count(*) FROM (
		SELECT episode_number,release_track FROM public.airing_schedule WHERE anime_id=$1
		INTERSECT SELECT episode_number,release_track FROM public.airing_schedule WHERE anime_id=$2
	) overlap`},
	{"owners_tracking_both", `SELECT count(*) FROM public.user_anime a JOIN public.user_anime b USING(user_id)
		WHERE a.anime_id=$1 AND b.anime_id=$2`},
	{"overlapping_follow_scope", `SELECT count(*) FROM public.user_follows a JOIN public.user_follows b USING(user_id,follow_scope)
		WHERE a.anime_id=$1 AND b.anime_id=$2`},
	{"links_becoming_self_references", `SELECT count(*) FROM public.anime_relations
		WHERE (source_anime_id=$1 AND target_anime_id=$2) OR (source_anime_id=$2 AND target_anime_id=$1)`},
}
