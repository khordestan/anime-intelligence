package catalogsync

import (
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)
const metadataRecordType="CATALOG_METADATA"
type metadataArchive struct{
 Kind string `json:"kind"`
 AnimeID string `json:"animeId"`
 Provider string `json:"provider"`
 ObservedAt time.Time `json:"observedAt"`
 Title feed.Title `json:"title"`
}
func archiveMetadata(ctx context.Context,tx pgx.Tx,source,id,provider string,observed time.Time,title feed.Title)(int,error){
 record:=metadataArchive{Kind:metadataRecordType,AnimeID:id,Provider:provider,ObservedAt:observed,Title:title}
 payload,err:=json.Marshal(record);if err!=nil{return 0,err}
 sum:=sha256.Sum256(payload)
 tag,err:=tx.Exec(ctx,`INSERT INTO source_records(source_id,external_id,record_type,payload,content_hash)
 VALUES($1,$2,$3,$4,$5) ON CONFLICT(source_id,external_id,content_hash) DO NOTHING`,source,title.ExternalID,metadataRecordType,payload,hex.EncodeToString(sum[:]))
 if err!=nil{return 0,err}
 return int(tag.RowsAffected()),nil
}
