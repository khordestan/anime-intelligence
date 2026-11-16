package catalogsync

import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)

var ErrHistory = errors.New("catalog history unavailable")

type HistoryEntry struct {
 ObservedAt time.Time `json:"observedAt"`
 ChangedFields []string `json:"changedFields"`
 Baseline bool `json:"baseline,omitempty"`
}
type HistoryResult struct {
 Entries []HistoryEntry `json:"entries"`
 Truncated bool `json:"truncated"`
 NextBefore *time.Time `json:"nextBefore,omitempty"`
}
func historyIdentifier(s string) bool {
 if len(s)==0||len(s)>128{return false}
 for _,c:=range s{if !((c>='a'&&c<='z')||(c>='A'&&c<='Z')||(c>='0'&&c<='9')||c=='-'||c=='_'||c=='.'){return false}}
 return true
}
// History summarizes retained selected observations, never raw source responses.
// The extra older row establishes a comparison across each page boundary.
func History(ctx context.Context,conn *pgx.Conn,provider,external string,limit int,before *time.Time)(HistoryResult,error){
 out:=HistoryResult{Entries:[]HistoryEntry{}}
 if !historyIdentifier(provider)||!historyIdentifier(external)||limit<1||limit>100||(before!=nil&&before.IsZero()){return out,ErrHistory}
 tx,err:=conn.BeginTx(ctx,pgx.TxOptions{IsoLevel:pgx.RepeatableRead,AccessMode:pgx.ReadOnly})
 if err!=nil{return out,ErrHistory}
 defer func(){cleanup,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();_=tx.Rollback(cleanup)}()
 if _,err=tx.Exec(ctx,"SET LOCAL statement_timeout='2s'");err!=nil{return out,ErrHistory}
 rows,err:=tx.Query(ctx,`SELECT CASE WHEN octet_length(r.payload::text)<=$5 THEN r.payload ELSE NULL END
 FROM source_records r JOIN data_sources s ON s.id=r.source_id
 WHERE s.provider_key=$1 AND r.external_id=$2 AND r.record_type='CATALOG_METADATA'
 AND ($3::timestamptz IS NULL OR (r.payload->>'observedAt')::timestamptz<$3)
 ORDER BY (r.payload->>'observedAt')::timestamptz DESC,r.id DESC LIMIT $4`,provider,external,before,limit+1,feed.MaxBytes)
 if err!=nil{return out,ErrHistory}
 records:=[]metadataArchive{};total:=0
 for rows.Next(){
  var payload []byte
  if err=rows.Scan(&payload);err!=nil{rows.Close();return out,ErrHistory}
  total+=len(payload)
  if len(payload)==0||total>16<<20{rows.Close();return out,ErrHistory}
  record,err:=decodeHistory(payload,provider,external)
  if err!=nil||(len(records)>0&&!record.ObservedAt.Before(records[len(records)-1].ObservedAt)){rows.Close();return out,ErrHistory}
  records=append(records,record)
 }
 rows.Close()
 if rows.Err()!=nil{return out,ErrHistory}
 if err=tx.Commit(ctx);err!=nil{return out,ErrHistory}
 n:=len(records);if n>limit{n=limit;out.Truncated=true}
 for i:=0;i<n;i++{
  entry:=HistoryEntry{ObservedAt:records[i].ObservedAt,ChangedFields:[]string{}}
  if i+1<len(records){entry.ChangedFields=historyChanges(records[i].Title,records[i+1].Title)}else{entry.Baseline=true}
  out.Entries=append(out.Entries,entry)
 }
 if out.Truncated{at:=out.Entries[n-1].ObservedAt;out.NextBefore=&at}
 return out,nil
}
func decodeHistory(payload []byte,provider,external string)(metadataArchive,error){
 var r metadataArchive
 if json.Unmarshal(payload,&r)!=nil||r.Kind!=metadataRecordType||r.Provider!=provider||r.Title.ExternalID!=external||r.AnimeID==""||r.ObservedAt.IsZero(){return metadataArchive{},ErrHistory}
 r.ObservedAt=r.ObservedAt.UTC()
 return r,nil
}
func historyChanges(current,previous feed.Title)[]string{
 a,_:=json.Marshal(current);b,_:=json.Marshal(previous)
 var newer,older map[string]json.RawMessage
 _=json.Unmarshal(a,&newer);_=json.Unmarshal(b,&older)
 out:=[]string{}
 // Fixed, sorted field names prevent archived values from entering diagnostics.
 for _,key:=range []string{"demographics","episodeCount","episodes","genres","mediaType","nativeTitle","relations","releaseYear","season","status","studios","synonyms","synopsis","tags","themes","title"}{
  if !bytes.Equal(newer[key],older[key]){out=append(out,key)}
 }
 return out
}
