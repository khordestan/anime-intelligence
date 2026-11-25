// Package catalogsync applies reviewed provider snapshots with administrator
// credentials. It is not registered as an HTTP mutation endpoint.
package catalogsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)

var (
	ErrSource = errors.New("catalog source is not approved")
	ErrStale = errors.New("catalog snapshot is not newer")
	ErrConflict = errors.New("catalog authority or continuity conflict")
	ErrImport = errors.New("catalog import failed")
)
type ImportOptions struct {
 SchedulesOnly bool // Requires existing reviewed identities; never writes metadata.
 ScheduleChangeEvents bool // Explicitly approved offline schedule event publication.
 CancellationEvents bool // Explicitly approved primary title cancellation notices.
	Preview bool
	Archive bool // Operator must separately approve historical retention rights.
}
type Result struct {
 SchedulesOnly bool `json:"schedulesOnly,omitempty"`
	Preview bool `json:"preview,omitempty"`
	ArchivedRecords int `json:"archivedRecords,omitempty"`
	Titles int `json:"titles"`
	SeasonAnnouncements int `json:"seasonAnnouncements"`
	CancellationEvents int `json:"cancellationEvents,omitempty"`
	ScheduleRevisions int `json:"scheduleRevisions"`
	ScheduleChangeEvents int `json:"scheduleChangeEvents,omitempty"`
	ReleaseEvents int `json:"releaseEvents"`
	UnresolvedRelations int `json:"unresolvedRelations"`
}

// Import commits the high-water mark, canonical records, provenance, relations,
// schedule revisions and unique event ledger atomically. Callers must not log
// database errors or raw source content; returned errors are fixed.
func Import(ctx context.Context, conn *pgx.Conn, s feed.Snapshot, now time.Time) (Result,error) {
 return ImportWithOptions(ctx,conn,s,now,ImportOptions{})
}
func ImportWithOptions(ctx context.Context,conn *pgx.Conn,s feed.Snapshot,now time.Time,options ImportOptions)(Result,error){
	if (options.SchedulesOnly&&(options.Archive||options.CancellationEvents))||s.Validate(now) != nil { return Result{},feed.ErrInvalid }
	s.ObservedAt = s.ObservedAt.UTC().Truncate(time.Microsecond)
	result,err := apply(ctx,conn,s,options)
	if err != nil {
		if errors.Is(err,ErrSource) || errors.Is(err,ErrStale) || errors.Is(err,ErrConflict) { return Result{},err }
		return Result{},ErrImport
	}
	return result,nil
}
func apply(ctx context.Context, conn *pgx.Conn, s feed.Snapshot,options ImportOptions) (Result,error) {
	out := Result{}
	tx,err := conn.Begin(ctx)
	if err != nil { return out,err }
	defer func(){cleanup,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();_=tx.Rollback(cleanup)}()
	result,err:=applyTx(ctx,tx,s,options)
	if err!=nil{return Result{},err}
	if options.Preview{if err=tx.Rollback(ctx);err!=nil{return Result{},err};result.Preview=true;return result,nil}
	if err=tx.Commit(ctx);err!=nil{return Result{},err}
	return result,nil
}
// applyTx lets the durable worker atomically commit imported facts and its
// fenced acknowledgement. Callers validate the snapshot before entering here.
func applyTx(ctx context.Context,tx pgx.Tx,s feed.Snapshot,options ImportOptions)(Result,error){
	out:=Result{}
	var err error
	// Global ordering also serializes imports from different providers which may
	// share canonical IDs or contribute edges to the same continuity graph.
	if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock(742019684)`);err!=nil {return out,err}
	var source string
	err=tx.QueryRow(ctx,`SELECT id::text FROM data_sources WHERE provider_key=$1 AND enabled AND COALESCE(policy_url,'')<>'' FOR UPDATE`,s.Provider).Scan(&source)
	if errors.Is(err,pgx.ErrNoRows) {return out,ErrSource}
	if err!=nil {return out,err}
	var previous time.Time
	err=tx.QueryRow(ctx,`SELECT observed_at FROM catalog_sync_state WHERE source_id=$1 FOR UPDATE`,source).Scan(&previous)
	if err!=nil && !errors.Is(err,pgx.ErrNoRows) {return out,err}
	if !previous.IsZero() && !s.ObservedAt.After(previous) {return out,ErrStale}
 if options.SchedulesOnly{
  out,err=applySelectedSchedules(ctx,tx,s,source,options)
  if err!=nil{return Result{},err}
  if _,err=tx.Exec(ctx,`INSERT INTO catalog_sync_state(source_id,observed_at) VALUES($1,$2)
 ON CONFLICT(source_id) DO UPDATE SET observed_at=excluded.observed_at`,source,s.ObservedAt);err!=nil{return Result{},err}
  return out,nil
 }
 if _,err=tx.Exec(ctx,`SELECT set_config('app.catalog_metadata_projection','1',true)`);err!=nil{return out,err}
	for _,title:=range s.Titles {
		var id string
		err=tx.QueryRow(ctx,`SELECT anime_id::text FROM anime_external_ids WHERE provider=$1 AND external_id=$2 FOR UPDATE`,s.Provider,title.ExternalID).Scan(&id)
		if errors.Is(err,pgx.ErrNoRows) {
			if err=tx.QueryRow(ctx,`INSERT INTO anime(canonical_title) VALUES($1) RETURNING id::text`,title.Title).Scan(&id);err!=nil {return out,err}
			if _,err=tx.Exec(ctx,`INSERT INTO anime_external_ids(anime_id,provider,external_id) VALUES($1,$2,$3)`,id,s.Provider,title.ExternalID);err!=nil {return out,err}
		} else if err!=nil {return out,err}
		var authority, priorStatus string
		var titleObserved time.Time
		err=tx.QueryRow(ctx,`SELECT source_id::text,COALESCE((SELECT b.payload->>'status' FROM catalog_base_metadata b WHERE b.anime_id=$1),payload->>'status','UNKNOWN'),observed_at FROM catalog_metadata WHERE anime_id=$1 FOR UPDATE`,id).Scan(&authority,&priorStatus,&titleObserved)
		known := err==nil
		if err!=nil && !errors.Is(err,pgx.ErrNoRows) {return out,err}
		if known && authority!=source {return out,ErrConflict}
  basePayload,err:=json.Marshal(title)
  if err!=nil{return out,err}
  if _,err=tx.Exec(ctx,`INSERT INTO catalog_base_metadata(anime_id,source_id,observed_at,payload) VALUES($1,$2,$3,$4)
 ON CONFLICT(anime_id) DO UPDATE SET source_id=excluded.source_id,observed_at=excluded.observed_at,payload=excluded.payload`,id,source,s.ObservedAt,basePayload);err!=nil{return out,err}
  effective,err:=projectMetadataFields(ctx,tx,id,title)
  if err!=nil{return out,err}
  if err=validateResolvedMetadata(ctx,tx,id,effective,s.ObservedAt);err!=nil{return out,err}
  payload,err:=json.Marshal(effective)
  if err!=nil{return out,err}
		if _,err=tx.Exec(ctx,`INSERT INTO catalog_metadata(anime_id,source_id,observed_at,payload) VALUES($1,$2,$3,$4)
ON CONFLICT(anime_id) DO UPDATE SET observed_at=excluded.observed_at,payload=excluded.payload`,id,source,s.ObservedAt,payload);err!=nil {return out,err}
		if _,err=tx.Exec(ctx,`UPDATE anime SET canonical_title=$2,native_title=NULLIF($3,''),synopsis=NULLIF($4,''),media_type=$5,status=$6,release_year=NULLIF($7,0),season_name=NULLIF($8,''),episode_count=$9,updated_at=now() WHERE id=$1`,
			id,effective.Title,effective.NativeTitle,effective.Synopsis,effective.MediaType,effective.Status,effective.ReleaseYear,effective.Season,effective.EpisodeCount);err!=nil {return out,err}
		if !previous.IsZero() && title.Status=="ANNOUNCED" && (!known || priorStatus=="UNKNOWN") {
			tag,err:=tx.Exec(ctx,`INSERT INTO domain_events(event_key,event_type,aggregate_type,aggregate_id,payload,occurred_at)
VALUES($1,'SEASON_ANNOUNCED','ANIME',$2,'{}'::jsonb,$3) ON CONFLICT(event_key) DO NOTHING`,"season-announced:"+id,id,s.ObservedAt)
			if err!=nil {return out,err}
			out.SeasonAnnouncements+=int(tag.RowsAffected())
		}
		if options.CancellationEvents && known && title.Status == "CANCELLED" &&
			(priorStatus == "ANNOUNCED" || priorStatus == "RELEASING" || priorStatus == "HIATUS") {
			inserted, err := recordCancellation(ctx, tx, id, s.Provider, s.ObservedAt)
			if err != nil {
				return out, err
			}
			out.CancellationEvents += inserted
		}
		comparison:=titleObserved
		if !known {comparison=previous}
		observation := episodeObservation{
			source: source, provider: s.Provider, animeID: id, mediaType: effective.MediaType,
			observed: s.ObservedAt, previous: comparison, eligible: !previous.IsZero(),
			scheduleChanges: options.ScheduleChangeEvents,
		}
		for _, episode := range title.Episodes {
			result, err := syncEpisode(ctx, tx, observation, episode)
			if err != nil {
				return out, err
			}
			out.ScheduleRevisions += result.revisions
			out.ReleaseEvents += result.releases
			out.ScheduleChangeEvents += result.scheduleChanges
		}
		if options.Archive{
   archived,err:=archiveMetadata(ctx,tx,source,id,s.Provider,s.ObservedAt,title)
   if err!=nil{return out,err};out.ArchivedRecords+=archived
  }
		out.Titles++
	}
	unresolved,err:=syncRelations(ctx,tx,source,s.Provider)
	if err!=nil {return out,err}
	out.UnresolvedRelations=unresolved
	if _,err=tx.Exec(ctx,`INSERT INTO catalog_sync_state(source_id,observed_at) VALUES($1,$2)
ON CONFLICT(source_id) DO UPDATE SET observed_at=excluded.observed_at`,source,s.ObservedAt);err!=nil {return out,err}
	return out,nil
}
func equalTime(a,b *time.Time) bool {
	return (a==nil && b==nil) || (a!=nil && b!=nil && a.Truncate(time.Microsecond).Equal(b.Truncate(time.Microsecond)))
}
type episodeObservation struct {
	source, provider, animeID, mediaType string
	observed, previous                   time.Time
	eligible, scheduleChanges           bool
}

type episodeSyncResult struct {
	revisions, releases, scheduleChanges int
}

func syncEpisode(ctx context.Context, tx pgx.Tx, observation episodeObservation, e feed.Episode) (episodeSyncResult, error) {
	var result episodeSyncResult
	id := observation.animeID
	var revision int
	var scheduled,released *time.Time
	var authority string
	err:=tx.QueryRow(ctx,`SELECT revision,scheduled_at,actual_released_at,COALESCE(source_id::text,'') FROM airing_schedule
WHERE anime_id=$1 AND episode_number=$2 AND release_track=$3 ORDER BY revision DESC LIMIT 1 FOR UPDATE`,id,e.Number,e.Track).Scan(&revision,&scheduled,&released,&authority)
	if err!=nil && !errors.Is(err,pgx.ErrNoRows) {return result, err}
	if revision>0 && authority!=observation.source {return result, ErrConflict}
	// Missing confirmation cannot retract a previously confirmed release.
	actual:=e.ReleasedAt
	if actual==nil {actual=released}
	if revision>0 && equalTime(scheduled,e.ScheduledAt) && equalTime(released,actual) {
		_,err=tx.Exec(ctx,`UPDATE airing_schedule SET last_verified_at=$4 WHERE anime_id=$1 AND episode_number=$2 AND release_track=$3 AND revision=$5`,id,e.Number,e.Track,observation.observed,revision)
		return result, err
	}
	status:="UNKNOWN"
	if e.ScheduledAt!=nil {status="SCHEDULED"}
	if actual!=nil {status="RELEASED"}
	if _,err=tx.Exec(ctx,`INSERT INTO airing_schedule(anime_id,episode_number,release_track,scheduled_at,actual_released_at,schedule_status,source_id,last_verified_at,revision)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,id,e.Number,e.Track,e.ScheduledAt,actual,status,observation.source,observation.observed,revision+1);err!=nil {return result, err}
	result.revisions = 1
	if observation.scheduleChanges && observation.eligible && !observation.previous.IsZero() &&
		scheduled != nil && e.ScheduledAt != nil && actual == nil && !equalTime(scheduled, e.ScheduledAt) {
		inserted, err := insertScheduleChangeEvent(ctx, tx, observation, e, *scheduled, revision+1)
		if err != nil {
			return result, err
		}
		result.scheduleChanges = inserted
	}
	// First provider imports baseline history. A new title can still report a
	// genuinely new first release since the prior provider observation; known
	// titles compare their own prior observation rather than unrelated updates.
	if !observation.eligible || observation.previous.IsZero() || released!=nil || actual==nil || !actual.Truncate(time.Microsecond).After(observation.previous) {return result,nil}
	identity,_:=json.Marshal([]any{"EPISODE_RELEASED",id,e.Number,e.Track})
	sum:=sha256.Sum256(identity)
	key:="episode:"+hex.EncodeToString(sum[:])
	payload,_:=json.Marshal(struct{
		Number int `json:"number"`
		Track string `json:"track"`
		ReleasedAt time.Time `json:"releasedAt"`
		MediaType string `json:"mediaType"`
	}{e.Number,e.Track,*actual,observation.mediaType})
	tag,err:=tx.Exec(ctx,`INSERT INTO domain_events(event_key,event_type,aggregate_type,aggregate_id,payload,occurred_at)
VALUES($1,'EPISODE_RELEASED','ANIME',$2,$3,$4) ON CONFLICT(event_key) DO NOTHING`,key,id,payload,*actual)
	result.releases = int(tag.RowsAffected())
	return result, err
}
func syncRelations(ctx context.Context,tx pgx.Tx,source,provider string) (int,error) {
	// Revisit retained observations so a later partial feed can resolve a target
	// omitted from the earlier feed without requiring its source title again.
	rows,err:=tx.Query(ctx,`SELECT m.anime_id::text,m.payload FROM catalog_metadata m WHERE source_id=$1 ORDER BY m.anime_id`,source)
	if err!=nil {return 0,err}
	type record struct{id string; title feed.Title}
	records:=[]record{}
	for rows.Next() {
		var id string
		var payload []byte
		if err=rows.Scan(&id,&payload);err!=nil {rows.Close();return 0,err}
		var title feed.Title
		if err=json.Unmarshal(payload,&title);err!=nil {rows.Close();return 0,err}
		records=append(records,record{id,title})
	}
	rows.Close()
	if err=rows.Err();err!=nil {return 0,err}
	unresolved:=0
	for _,record:=range records {
		for _,relation:=range record.title.Relations {
			var target string
			var retired bool
			err = tx.QueryRow(ctx, `SELECT a.anime_id::text, EXISTS (
				SELECT 1 FROM catalog_continuity_retractions r
				WHERE $4::text IN ('PREQUEL','SEQUEL')
				  AND r.prequel_id=CASE $4 WHEN 'PREQUEL' THEN a.anime_id ELSE $3::uuid END
				  AND r.sequel_id=CASE $4 WHEN 'PREQUEL' THEN $3::uuid ELSE a.anime_id END
			) FROM anime_external_ids a WHERE a.provider=$1 AND a.external_id=$2`,
				provider, relation.ExternalID, record.id, relation.Type).Scan(&target, &retired)
			if errors.Is(err, pgx.ErrNoRows) {
				unresolved++
				continue
			}
			if err != nil {
				return 0, err
			}
			if target == record.id {
				return 0, ErrConflict
			}
			if retired {
				continue
			}
			tag,err:=tx.Exec(ctx,`INSERT INTO anime_relations(source_anime_id,target_anime_id,relation_type,source_id)
VALUES($1,$2,$3,$4) ON CONFLICT(source_anime_id,target_anime_id,relation_type)
DO UPDATE SET confidence=1 WHERE anime_relations.source_id=excluded.source_id`,record.id,target,relation.Type,source)
			if err!=nil {return 0,err}
			if tag.RowsAffected()!=1 {return 0,ErrConflict}
		}
	}
	rows,err=tx.Query(ctx,`SELECT source_anime_id::text,target_anime_id::text,relation_type FROM anime_relations WHERE relation_type IN ('PREQUEL','SEQUEL')`)
	if err!=nil {return 0,err}
	graph:=map[string][]feed.Relation{}
	for rows.Next() {
		var from,to,kind string
		if err=rows.Scan(&from,&to,&kind);err!=nil {rows.Close();return 0,err}
		graph[from]=append(graph[from],feed.Relation{ExternalID:to,Type:kind})
	}
	rows.Close()
	if err=rows.Err();err!=nil {return 0,err}
	titles:=make([]feed.Title,0,len(graph))
	for id,relations:=range graph {titles=append(titles,feed.Title{ExternalID:id,Relations:relations})}
	if feed.ValidateContinuity(titles)!=nil {return 0,ErrConflict}
	return unresolved,nil
}
