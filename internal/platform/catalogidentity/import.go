// Package catalogidentity links explicitly reviewed public provider identities.
// It never matches by title, merges entities or transfers metadata authority.
package catalogidentity

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
 "github.com/jackc/pgx/v5/pgconn"
)
var ErrInvalid=errors.New("invalid identity review")
var ErrSource=errors.New("identity source is not approved")
var ErrConflict=errors.New("identity mapping conflicts")
var ErrTarget=errors.New("canonical identity is missing")
var ErrApply=errors.New("identity mapping failed")
type Mapping struct{ExternalID string `json:"externalId"`;AnimeID string `json:"animeId"`}
type Snapshot struct{
 Version int `json:"version"`
 Provider string `json:"provider"`
 ReviewedAt time.Time `json:"reviewedAt"`
 Reference string `json:"reference"`
 Mappings []Mapping `json:"mappings"`
}
type Result struct{Linked int `json:"linked"`;Unchanged int `json:"unchanged"`;AuditRecords int `json:"auditRecords"`;Applied bool `json:"applied"`}
func identifier(s string)bool{
 if len(s)==0||len(s)>128{return false}
 for _,r:=range s{if !((r>='a'&&r<='z')||(r>='A'&&r<='Z')||(r>='0'&&r<='9')||r=='-'||r=='_'||r=='.'){return false}}
 return true
}
func uuid(s string)bool{
 if len(s)!=36{return false}
 for i,r:=range s{if i==8||i==13||i==18||i==23{if r!='-'{return false};continue};if !((r>='0'&&r<='9')||(r>='a'&&r<='f')||(r>='A'&&r<='F')){return false}}
 return true
}
func(s Snapshot)Validate(now time.Time)error{
 ref,err:=url.Parse(s.Reference)
 if s.Version!=1||!identifier(s.Provider)||s.ReviewedAt.IsZero()||s.ReviewedAt.After(now)||len(s.Mappings)==0||len(s.Mappings)>1000||err!=nil||len(s.Reference)>2048||ref.Scheme!="https"||ref.Hostname()==""||ref.User!=nil||ref.RawQuery!=""||ref.Fragment!=""{return ErrInvalid}
 external,canonical:=map[string]bool{},map[string]bool{}
 for _,m:=range s.Mappings{
  key:=strings.ToLower(m.AnimeID)
  if !identifier(m.ExternalID)||!uuid(m.AnimeID)||external[m.ExternalID]||canonical[key]{return ErrInvalid}
  external[m.ExternalID]=true;canonical[key]=true
 }
 return nil
}
func Decode(r io.Reader,now time.Time)(Snapshot,error){
 b,err:=io.ReadAll(io.LimitReader(r,(1<<20)+1));if err!=nil||len(b)>1<<20{return Snapshot{},ErrInvalid}
 var s Snapshot;d:=json.NewDecoder(strings.NewReader(string(b)));d.DisallowUnknownFields();var extra any
 if d.Decode(&s)!=nil||d.Decode(&extra)!=io.EOF||s.Validate(now)!=nil{return Snapshot{},ErrInvalid}
 return s,nil
}
// Apply validates the complete batch under the shared catalog writer lock.
// A preview performs the same writes/constraints but rolls the transaction back.
func Apply(ctx context.Context,conn *pgx.Conn,s Snapshot,now time.Time,commit bool)(Result,error){
 if s.Validate(now)!=nil{return Result{},ErrInvalid}
 s.ReviewedAt=s.ReviewedAt.UTC().Truncate(time.Microsecond)
 out,err:=apply(ctx,conn,s,commit)
 if err!=nil{
  for _,known:=range []error{ErrSource,ErrConflict,ErrTarget}{if errors.Is(err,known){return Result{},known}}
  var pg *pgconn.PgError;if errors.As(err,&pg)&&(pg.Code=="23505"||(pg.Code=="23514"&&pg.ConstraintName=="catalog_identity_retired")){return Result{},ErrConflict}
  return Result{},ErrApply
 }
 return out,nil
}
func apply(ctx context.Context,conn *pgx.Conn,s Snapshot,commit bool)(Result,error){
 out:=Result{}
 tx,err:=conn.Begin(ctx);if err!=nil{return out,err}
 defer func(){cleanup,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();_=tx.Rollback(cleanup)}()
 if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock(742019684)`);err!=nil{return out,err}
 var source string
 err=tx.QueryRow(ctx,`SELECT id::text FROM data_sources WHERE provider_key=$1 AND enabled AND COALESCE(policy_url,'')<>'' FOR UPDATE`,s.Provider).Scan(&source)
 if errors.Is(err,pgx.ErrNoRows){return out,ErrSource};if err!=nil{return out,err}
 for _,m:=range s.Mappings{
  id:=strings.ToLower(m.AnimeID)
  var exists bool
  if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM anime WHERE id=$1)`,id).Scan(&exists);err!=nil{return out,err}
  if !exists{return out,ErrTarget}
  rows,err:=tx.Query(ctx,`SELECT anime_id::text,external_id FROM anime_external_ids WHERE provider=$1 AND (external_id=$2 OR anime_id=$3) FOR UPDATE`,s.Provider,m.ExternalID,id)
  if err!=nil{return out,err}
  same:=false
  for rows.Next(){var target,external string;if err=rows.Scan(&target,&external);err!=nil{rows.Close();return out,err};if target!=id||external!=m.ExternalID{rows.Close();return out,ErrConflict};same=true}
  rows.Close();if err=rows.Err();err!=nil{return out,err}
  if same{out.Unchanged++}else{
   if _,err=tx.Exec(ctx,`INSERT INTO anime_external_ids(anime_id,provider,external_id) VALUES($1,$2,$3)`,id,s.Provider,m.ExternalID);err!=nil{return out,err};out.Linked++
  }
  audit:=struct{Kind string `json:"kind"`;Provider string `json:"provider"`;ExternalID string `json:"externalId"`;AnimeID string `json:"animeId"`;ReviewedAt time.Time `json:"reviewedAt"`;Reference string `json:"reference"`}{"IDENTITY_MAP",s.Provider,m.ExternalID,id,s.ReviewedAt,s.Reference}
  payload,err:=json.Marshal(audit);if err!=nil{return out,err}
  digest:=sha256.Sum256(payload)
  tag,err:=tx.Exec(ctx,`INSERT INTO source_records(source_id,external_id,record_type,payload,content_hash) VALUES($1,$2,'IDENTITY_MAP',$3,$4) ON CONFLICT(source_id,external_id,content_hash) DO NOTHING`,source,m.ExternalID,payload,hex.EncodeToString(digest[:]))
  if err!=nil{return out,err};out.AuditRecords+=int(tag.RowsAffected())
 }
 if !commit{if err=tx.Rollback(ctx);err!=nil{return Result{},err};return out,nil}
 if err=tx.Commit(ctx);err!=nil{return Result{},err};out.Applied=true;return out,nil
}
