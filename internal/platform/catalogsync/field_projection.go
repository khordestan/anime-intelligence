package catalogsync

import (
 "context"
 "encoding/json"
 "strings"
 "time"
 "unicode/utf8"

 "github.com/jackc/pgx/v5"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)

func labelField(field string)bool{
 switch field{case "genres","studios","synonyms","themes","demographics","tags":return true}
 return false
}
func validFieldValues(values []string)bool{
 if values==nil||len(values)>256{return false}
 seen:=map[string]bool{}
 for _,value:=range values{
  key:=strings.ToLower(value)
  if value==""||!utf8.ValidString(value)||strings.ContainsRune(value,0)||value!=strings.TrimSpace(value)||len(value)>256||seen[key]{return false}
  seen[key]=true
 }
 return true
}
func projectMetadataFields(ctx context.Context,tx pgx.Tx,id string,base feed.Title)(feed.Title,error){
 rows,err:=tx.Query(ctx,`SELECT field,value FROM catalog_field_choices WHERE anime_id=$1 AND mode='SELECT' ORDER BY field`,id)
 if err!=nil{return feed.Title{},err}
 defer rows.Close()
 for rows.Next(){
  var field string;var raw []byte;var values []string
  if err=rows.Scan(&field,&raw);err!=nil{return feed.Title{},err}
  if scalarField(field){
   value,ok:=scalarValue(field,raw);if !ok{return feed.Title{},ErrImport}
   switch field{
   case "releaseYear":base.ReleaseYear=value.(int)
   case "season":base.Season=value.(string)
   case "mediaType":base.MediaType=value.(string)
   case "episodeCount":if value==nil{base.EpisodeCount=nil}else{count:=value.(int);base.EpisodeCount=&count}
   }
   continue
  }
  if json.Unmarshal(raw,&values)!=nil||!labelField(field)||!validFieldValues(values){return feed.Title{},ErrImport}
  switch field{
  case "genres":base.Genres=values
  case "studios":base.Studios=values
  case "synonyms":base.Synonyms=values
  case "themes":base.Themes=values
  case "demographics":base.Demographics=values
  case "tags":base.Tags=values
  }
 }
 if err=rows.Err();err!=nil{return feed.Title{},err}
 return base,nil
}

func scalarField(field string)bool{
 switch field{case "releaseYear","season","mediaType","episodeCount":return true}
 return false
}
// scalarValue accepts explicit JSON types only. null is meaningful solely for
// an unknown episode total; zero is a separate known-zero total.
func scalarValue(field string,raw json.RawMessage)(any,bool){
 if len(raw)==0{return nil,false}
 switch field{
 case "releaseYear","episodeCount":
  var number *int
  if json.Unmarshal(raw,&number)!=nil{return nil,false}
  if number==nil{return nil,field=="episodeCount"}
  if field=="releaseYear"{if *number!=0&&(*number<1900||*number>9999){return nil,false}}
  if field=="episodeCount"&&(*number<0||*number>100000){return nil,false}
  return *number,true
 case "season","mediaType":
  var text *string
  if json.Unmarshal(raw,&text)!=nil||text==nil{return nil,false}
  allowed:=map[string]bool{}
  if field=="season"{for _,s:=range []string{"","WINTER","SPRING","SUMMER","FALL"}{allowed[s]=true}}else{
   for _,s:=range []string{"TV","MOVIE","OVA","ONA","SPECIAL","MUSIC","UNKNOWN"}{allowed[s]=true}
  }
  return *text,allowed[*text]
 }
 return nil,false
}
func validateResolvedMetadata(ctx context.Context,tx pgx.Tx,id string,title feed.Title,observed time.Time)error{
 if (feed.Snapshot{Version:1,Provider:"resolved",ObservedAt:observed,Titles:[]feed.Title{title}}).Validate(time.Now().UTC())!=nil{return ErrConflict}
 if title.EpisodeCount!=nil{
  var conflict bool
  if err:=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM airing_schedule WHERE anime_id=$1 AND episode_number>$2)`,id,*title.EpisodeCount).Scan(&conflict);err!=nil{return err}
  if conflict{return ErrConflict}
 }
 return nil
}
