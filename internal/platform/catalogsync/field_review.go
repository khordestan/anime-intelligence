package catalogsync

import (
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "io"
 "net/url"
 "strings"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)

var ErrFieldReview=errors.New("invalid catalog field review")
var ErrFieldConflict=errors.New("catalog field review conflicts")
var ErrFieldApply=errors.New("catalog field review failed")
type FieldSelection struct{
 AnimeID string `json:"animeId"`
 Field string `json:"field"`
 Mode string `json:"mode"`
 ExpectedRevision *int `json:"expectedRevision"`
 Provider string `json:"provider,omitempty"`
 ExternalID string `json:"externalId,omitempty"`
 Values []string `json:"values"`
 Scalar json.RawMessage `json:"scalar,omitempty"`
 ObservedAt time.Time `json:"observedAt,omitempty"`
}
type FieldReview struct{
 Version int `json:"version"`
 ReviewedAt time.Time `json:"reviewedAt"`
 Reference string `json:"reference"`
 Selections []FieldSelection `json:"selections"`
}
type FieldReviewResult struct{
 Applied bool `json:"applied"`
 Selected int `json:"selected"`
 Restored int `json:"restored"`
 Unchanged int `json:"unchanged"`
 AuditRecords int `json:"auditRecords"`
}
func fieldUUID(s string)bool{
 if len(s)!=36{return false}
 for i,c:=range s{
  if i==8||i==13||i==18||i==23{if c!='-'{return false};continue}
  if !((c>='a'&&c<='f')||(c>='A'&&c<='F')||(c>='0'&&c<='9')){return false}
 }
 return true
}
func fieldIdentifier(s string)bool{
 if len(s)==0||len(s)>128{return false}
 for _,c:=range s{if !((c>='a'&&c<='z')||(c>='A'&&c<='Z')||(c>='0'&&c<='9')||c=='-'||c=='_'||c=='.'){return false}}
 return true
}
func(s FieldReview)Validate(now time.Time)error{
 ref,err:=url.Parse(s.Reference)
 if s.Version!=1||s.ReviewedAt.IsZero()||s.ReviewedAt.After(now)||len(s.Selections)==0||len(s.Selections)>100||
 err!=nil||len(s.Reference)>2048||ref.Scheme!="https"||ref.Hostname()==""||ref.User!=nil||ref.RawQuery!=""||ref.Fragment!=""{return ErrFieldReview}
 seen:=map[string]bool{}
 for _,v:=range s.Selections{
  key:=strings.ToLower(v.AnimeID)+":"+v.Field
  if !fieldUUID(v.AnimeID)||(!labelField(v.Field)&&!scalarField(v.Field))||v.ExpectedRevision==nil||*v.ExpectedRevision<0||*v.ExpectedRevision>=1000000000||seen[key]{return ErrFieldReview}
  seen[key]=true
  switch v.Mode{
  case "SELECT":
   if !fieldIdentifier(v.Provider)||!fieldIdentifier(v.ExternalID)||v.ObservedAt.IsZero()||v.ObservedAt.After(s.ReviewedAt){return ErrFieldReview}
   if labelField(v.Field){if !validFieldValues(v.Values)||len(v.Scalar)!=0{return ErrFieldReview}}else{
    if v.Values!=nil{return ErrFieldReview}
    if _,ok:=scalarValue(v.Field,v.Scalar);!ok{return ErrFieldReview}
   }
  case "BASE":
   if v.Provider!=""||v.ExternalID!=""||v.Values!=nil||len(v.Scalar)!=0||!v.ObservedAt.IsZero(){return ErrFieldReview}
  default:return ErrFieldReview
  }
 }
 b,err:=json.Marshal(s);if err!=nil||len(b)>1<<20{return ErrFieldReview}
 return nil
}
func DecodeFieldReview(r io.Reader,now time.Time)(FieldReview,error){
 b,err:=io.ReadAll(io.LimitReader(r,(1<<20)+1))
 if err!=nil||len(b)>1<<20{return FieldReview{},ErrFieldReview}
 var s FieldReview
 d:=json.NewDecoder(strings.NewReader(string(b)));d.DisallowUnknownFields()
 var extra any
 if d.Decode(&s)!=nil||d.Decode(&extra)!=io.EOF||s.Validate(now)!=nil{return FieldReview{},ErrFieldReview}
 return s,nil
}
// ReviewFields previews by default. References/times identify a public-data
// decision, never a user account. All writes use one catalog transaction lock.
func ReviewFields(ctx context.Context,conn *pgx.Conn,s FieldReview,now time.Time,commit bool)(FieldReviewResult,error){
 if s.Validate(now)!=nil{return FieldReviewResult{},ErrFieldReview}
 if conn==nil||CheckWriter(ctx,conn)!=nil{return FieldReviewResult{},ErrFieldApply}
 s.ReviewedAt=s.ReviewedAt.UTC().Truncate(time.Microsecond)
 s.Selections=append([]FieldSelection(nil),s.Selections...)
 for i:=range s.Selections{s.Selections[i].AnimeID=strings.ToLower(s.Selections[i].AnimeID);s.Selections[i].ObservedAt=s.Selections[i].ObservedAt.UTC().Truncate(time.Microsecond)}
 for i:=range s.Selections{
  if s.Selections[i].Mode=="SELECT"&&scalarField(s.Selections[i].Field){
   value,_:=scalarValue(s.Selections[i].Field,s.Selections[i].Scalar)
   raw,err:=json.Marshal(value);if err!=nil{return FieldReviewResult{},ErrFieldReview}
   s.Selections[i].Scalar=raw
  }
 }
 out,err:=reviewFields(ctx,conn,s,commit)
 if err!=nil{
  if errors.Is(err,ErrConflict){return FieldReviewResult{},ErrFieldConflict}
  if errors.Is(err,ErrSource)||errors.Is(err,ErrFieldConflict){return FieldReviewResult{},err}
  return FieldReviewResult{},ErrFieldApply
 }
 return out,nil
}
func reviewFields(ctx context.Context,conn *pgx.Conn,s FieldReview,commit bool)(FieldReviewResult,error){
 out:=FieldReviewResult{}
 tx,err:=conn.Begin(ctx);if err!=nil{return out,err}
 defer rollbackFetch(tx)
 if _,err=tx.Exec(ctx,"SET LOCAL statement_timeout='2s'");err!=nil{return out,err}
 if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock(742019684)`);err!=nil{return out,err}
 if _,err=tx.Exec(ctx,`SELECT set_config('app.catalog_metadata_projection','1',true)`);err!=nil{return out,err}
 for _,v:=range s.Selections{
  var source,external,value,observed any
  if v.Mode=="SELECT"{
   var id string
   err=tx.QueryRow(ctx,`SELECT id::text FROM data_sources WHERE provider_key=$1 AND enabled AND COALESCE(policy_url,'')<>'' FOR UPDATE`,v.Provider).Scan(&id)
   if errors.Is(err,pgx.ErrNoRows){return out,ErrSource};if err!=nil{return out,err}
   var matches bool
   if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM anime_external_ids WHERE anime_id=$1 AND provider=$2 AND external_id=$3)`,v.AnimeID,v.Provider,v.ExternalID).Scan(&matches);err!=nil{return out,err}
   if !matches{return out,ErrFieldConflict}
   raw,err:=json.Marshal(v.Values);if err!=nil{return out,err}
   if scalarField(v.Field){raw=v.Scalar}
   source=id;external=v.ExternalID;value=raw;observed=v.ObservedAt
  }
  var primary,baseSource string;var primaryAt,baseAt time.Time;var basePayload []byte
  err=tx.QueryRow(ctx,`SELECT m.source_id::text,m.observed_at,b.source_id::text,b.observed_at,b.payload
 FROM catalog_metadata m JOIN catalog_base_metadata b ON b.anime_id=m.anime_id
 WHERE m.anime_id=$1 FOR UPDATE OF m`,v.AnimeID).Scan(&primary,&primaryAt,&baseSource,&baseAt,&basePayload)
  if errors.Is(err,pgx.ErrNoRows){return out,ErrFieldConflict};if err!=nil{return out,err}
  if primary!=baseSource||!primaryAt.Equal(baseAt){return out,ErrFieldConflict}
  audit:=struct{Version int `json:"version"`;ReviewedAt time.Time `json:"reviewedAt"`;Reference string `json:"reference"`;Selection FieldSelection `json:"selection"`}{1,s.ReviewedAt,s.Reference,v}
  payload,err:=json.Marshal(audit);if err!=nil{return out,err}
  hash:=sha256.Sum256(payload);digest:=hex.EncodeToString(hash[:])
  var revision int;var priorHash string;var priorAt time.Time
  err=tx.QueryRow(ctx,`SELECT revision,decision_hash,reviewed_at FROM catalog_field_choices WHERE anime_id=$1 AND field=$2 FOR UPDATE`,v.AnimeID,v.Field).Scan(&revision,&priorHash,&priorAt)
  if err!=nil&&!errors.Is(err,pgx.ErrNoRows){return out,err}
  if revision>0&&priorHash==digest{out.Unchanged++;continue}
  if revision!=*v.ExpectedRevision||(!priorAt.IsZero()&&!s.ReviewedAt.After(priorAt)){return out,ErrFieldConflict}
  revision++
  _,err=tx.Exec(ctx,`INSERT INTO catalog_field_choices(anime_id,field,revision,mode,source_id,external_id,value,observed_at,reviewed_at,reference,decision_hash)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(anime_id,field) DO UPDATE SET
 revision=excluded.revision,mode=excluded.mode,source_id=excluded.source_id,external_id=excluded.external_id,
 value=excluded.value,observed_at=excluded.observed_at,reviewed_at=excluded.reviewed_at,reference=excluded.reference,decision_hash=excluded.decision_hash`,
 v.AnimeID,v.Field,revision,v.Mode,source,external,value,observed,s.ReviewedAt,s.Reference,digest)
  if err!=nil{return out,err}
  if _,err=tx.Exec(ctx,`INSERT INTO catalog_field_choice_audits(anime_id,field,revision,payload) VALUES($1,$2,$3,$4)`,v.AnimeID,v.Field,revision,payload);err!=nil{return out,err}
  var base feed.Title
  if json.Unmarshal(basePayload,&base)!=nil{return out,ErrFieldApply}
  if (feed.Snapshot{Version:1,Provider:"base",ObservedAt:baseAt,Titles:[]feed.Title{base}}).Validate(time.Now().UTC())!=nil{return out,ErrFieldApply}
  effective,err:=projectMetadataFields(ctx,tx,v.AnimeID,base);if err!=nil{return out,err}
  if err=validateResolvedMetadata(ctx,tx,v.AnimeID,effective,baseAt);err!=nil{return out,err}
  switch v.Field{
  case "title":_,err=tx.Exec(ctx,`UPDATE anime SET canonical_title=$2,updated_at=now() WHERE id=$1`,v.AnimeID,effective.Title)
  case "nativeTitle":_,err=tx.Exec(ctx,`UPDATE anime SET native_title=NULLIF($2::text,''),updated_at=now() WHERE id=$1`,v.AnimeID,effective.NativeTitle)
  case "synopsis":_,err=tx.Exec(ctx,`UPDATE anime SET synopsis=NULLIF($2::text,''),updated_at=now() WHERE id=$1`,v.AnimeID,effective.Synopsis)
  case "status":_,err=tx.Exec(ctx,`UPDATE anime SET status=$2,updated_at=now() WHERE id=$1`,v.AnimeID,effective.Status)
  case "releaseYear":_,err=tx.Exec(ctx,`UPDATE anime SET release_year=NULLIF($2::int,0),updated_at=now() WHERE id=$1`,v.AnimeID,effective.ReleaseYear)
  case "season":_,err=tx.Exec(ctx,`UPDATE anime SET season_name=NULLIF($2::text,''),updated_at=now() WHERE id=$1`,v.AnimeID,effective.Season)
  case "mediaType":_,err=tx.Exec(ctx,`UPDATE anime SET media_type=$2,updated_at=now() WHERE id=$1`,v.AnimeID,effective.MediaType)
  case "episodeCount":_,err=tx.Exec(ctx,`UPDATE anime SET episode_count=$2,updated_at=now() WHERE id=$1`,v.AnimeID,effective.EpisodeCount)
  }
  if err!=nil{return out,err}
  projected,err:=json.Marshal(effective);if err!=nil{return out,err}
  if _,err=tx.Exec(ctx,`UPDATE catalog_metadata SET payload=$2 WHERE anime_id=$1`,v.AnimeID,projected);err!=nil{return out,err}
  out.AuditRecords++
  if v.Mode=="SELECT"{out.Selected++}else{out.Restored++}
 }
 if !commit{if err=tx.Rollback(ctx);err!=nil{return out,err};return out,nil}
 if err=tx.Commit(ctx);err!=nil{return out,err}
 out.Applied=true
 return out,nil
}
