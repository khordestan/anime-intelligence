package catalogsync

import (
 "context"
 "encoding/json"
 "strings"
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
func projectLabelFields(ctx context.Context,tx pgx.Tx,id string,base feed.Title)(feed.Title,error){
 rows,err:=tx.Query(ctx,`SELECT field,value FROM catalog_field_choices WHERE anime_id=$1 AND mode='SELECT' ORDER BY field`,id)
 if err!=nil{return feed.Title{},err}
 defer rows.Close()
 for rows.Next(){
  var field string;var raw []byte;var values []string
  if err=rows.Scan(&field,&raw);err!=nil{return feed.Title{},err}
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
