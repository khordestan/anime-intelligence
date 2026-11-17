package catalogidentity

import (
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "io"
 "strings"
 "time"

 "github.com/jackc/pgx/v5"
)

type Retraction struct{
 ExternalID string `json:"externalId"`
 AnimeID string `json:"animeId"`
 ExpectedMappingID string `json:"expectedMappingId"`
}
type RetractionReview struct{
 Version int `json:"version"`
 Provider string `json:"provider"`
 ReviewedAt time.Time `json:"reviewedAt"`
 Reference string `json:"reference"`
 Retractions []Retraction `json:"retractions"`
}
type RetractionResult struct{
 Retracted int `json:"retracted"`
 Unchanged int `json:"unchanged"`
 AuditRecords int `json:"auditRecords"`
 Applied bool `json:"applied"`
}
func(s RetractionReview)Validate(now time.Time)error{
 base:=Snapshot{Version:s.Version,Provider:s.Provider,ReviewedAt:s.ReviewedAt,Reference:s.Reference}
 for _,r:=range s.Retractions{
  if !uuid(r.ExpectedMappingID){return ErrInvalid}
  base.Mappings=append(base.Mappings,Mapping{ExternalID:r.ExternalID,AnimeID:r.AnimeID})
 }
 return base.Validate(now)
}
func DecodeRetractions(r io.Reader,now time.Time)(RetractionReview,error){
 b,err:=io.ReadAll(io.LimitReader(r,(1<<20)+1));if err!=nil||len(b)>1<<20{return RetractionReview{},ErrInvalid}
 var s RetractionReview;d:=json.NewDecoder(strings.NewReader(string(b)));d.DisallowUnknownFields();var extra any
 if d.Decode(&s)!=nil||d.Decode(&extra)!=io.EOF||s.Validate(now)!=nil{return RetractionReview{},ErrInvalid}
 return s,nil
}
// Retract only removes exact, unused links. It never erases facts or events,
// moves private data, transfers authority, or permits this source ID to reappear.
func Retract(ctx context.Context,conn *pgx.Conn,s RetractionReview,now time.Time,commit bool)(RetractionResult,error){
 if s.Validate(now)!=nil{return RetractionResult{},ErrInvalid}
 if conn==nil{return RetractionResult{},ErrApply}
 s.ReviewedAt=s.ReviewedAt.UTC().Truncate(time.Microsecond)
 s.Retractions=append([]Retraction(nil),s.Retractions...)
 for i:=range s.Retractions{
  s.Retractions[i].AnimeID=strings.ToLower(s.Retractions[i].AnimeID)
  s.Retractions[i].ExpectedMappingID=strings.ToLower(s.Retractions[i].ExpectedMappingID)
 }
 out,err:=retract(ctx,conn,s,commit)
 if err!=nil{
  if errors.Is(err,ErrConflict)||errors.Is(err,ErrSource){return RetractionResult{},err}
  return RetractionResult{},ErrApply
 }
 return out,nil
}
func retract(ctx context.Context,conn *pgx.Conn,s RetractionReview,commit bool)(RetractionResult,error){
 out:=RetractionResult{}
 tx,err:=conn.Begin(ctx);if err!=nil{return out,err}
 defer func(){cleanup,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();_=tx.Rollback(cleanup)}()
 if _,err=tx.Exec(ctx,"SET LOCAL statement_timeout='2s'");err!=nil{return out,err}
 if _,err=tx.Exec(ctx,"SELECT pg_advisory_xact_lock(742019684)");err!=nil{return out,err}
 var source string
 // Retraction must remain possible after a source is disabled.
 err=tx.QueryRow(ctx,"SELECT id::text FROM data_sources WHERE provider_key=$1 FOR UPDATE",s.Provider).Scan(&source)
 if errors.Is(err,pgx.ErrNoRows){return out,ErrSource};if err!=nil{return out,err}
 for _,r:=range s.Retractions{
  audit:=struct{
   Kind string `json:"kind"`
   Provider string `json:"provider"`
   ReviewedAt time.Time `json:"reviewedAt"`
   Reference string `json:"reference"`
   Retraction Retraction `json:"retraction"`
  }{"IDENTITY_RETRACT",s.Provider,s.ReviewedAt,s.Reference,r}
  payload,err:=json.Marshal(audit);if err!=nil{return out,err}
  sum:=sha256.Sum256(payload);digest:=hex.EncodeToString(sum[:])
  var prior string
  err=tx.QueryRow(ctx,"SELECT decision_hash FROM catalog_identity_retractions WHERE provider=$1 AND external_id=$2",s.Provider,r.ExternalID).Scan(&prior)
  if err==nil{
   if prior!=digest{return out,ErrConflict}
   out.Unchanged++;continue
  }
  if !errors.Is(err,pgx.ErrNoRows){return out,err}
  var mapping,anime string
  err=tx.QueryRow(ctx,"SELECT id::text,anime_id::text FROM anime_external_ids WHERE provider=$1 AND external_id=$2",s.Provider,r.ExternalID).Scan(&mapping,&anime)
  if errors.Is(err,pgx.ErrNoRows){return out,ErrConflict};if err!=nil{return out,err}
  if mapping!=r.ExpectedMappingID||anime!=r.AnimeID{return out,ErrConflict}
  var used bool
  err=tx.QueryRow(ctx,`SELECT
 EXISTS(SELECT 1 FROM catalog_metadata WHERE source_id=$1 AND anime_id=$2)
 OR EXISTS(SELECT 1 FROM catalog_base_metadata WHERE source_id=$1 AND anime_id=$2)
 OR EXISTS(SELECT 1 FROM catalog_field_choices WHERE source_id=$1 AND anime_id=$2)
 OR EXISTS(SELECT 1 FROM catalog_field_choice_audits WHERE anime_id=$2
   AND payload->'selection'->>'provider'=$3 AND payload->'selection'->>'externalId'=$4)
 OR EXISTS(SELECT 1 FROM catalog_schedule_observations WHERE source_id=$1 AND anime_id=$2)
 OR EXISTS(SELECT 1 FROM airing_schedule WHERE source_id=$1 AND anime_id=$2)
 OR EXISTS(SELECT 1 FROM anime_relations WHERE source_id=$1 AND (source_anime_id=$2 OR target_anime_id=$2))
 OR EXISTS(SELECT 1 FROM catalog_fetch_jobs WHERE source_id=$1 AND external_id=$4)
 OR EXISTS(SELECT 1 FROM source_records WHERE source_id=$1 AND external_id=$4 AND record_type<>'IDENTITY_MAP')
 OR EXISTS(SELECT 1 FROM catalog_base_metadata m,
   LATERAL jsonb_array_elements(COALESCE(m.payload->'relations','[]'::jsonb)) relation
   WHERE m.source_id=$1 AND relation->>'externalId'=$4)`,source,r.AnimeID,s.Provider,r.ExternalID).Scan(&used)
  if err!=nil{return out,err};if used{return out,ErrConflict}
  if _,err=tx.Exec(ctx,`INSERT INTO catalog_identity_retractions(provider,external_id,anime_id,mapping_id,reviewed_at,reference,decision_hash)
 VALUES($1,$2,$3,$4,$5,$6,$7)`,s.Provider,r.ExternalID,r.AnimeID,r.ExpectedMappingID,s.ReviewedAt,s.Reference,digest);err!=nil{return out,err}
  tag,err:=tx.Exec(ctx,"DELETE FROM anime_external_ids WHERE id=$1 AND anime_id=$2 AND provider=$3 AND external_id=$4",r.ExpectedMappingID,r.AnimeID,s.Provider,r.ExternalID)
  if err!=nil{return out,err};if tag.RowsAffected()!=1{return out,ErrConflict}
  out.Retracted++;out.AuditRecords++
 }
 if !commit{if err=tx.Rollback(ctx);err!=nil{return out,err};return out,nil}
 if err=tx.Commit(ctx);err!=nil{return out,err}
 out.Applied=true;return out,nil
}
