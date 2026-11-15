// Package curation applies explicitly reviewed, public, non-spoiler feature
// observations. It fetches no URLs and accepts no user-profile data.
package curation

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)
var ErrInvalid=errors.New("invalid curated feature snapshot")
var ErrImport=errors.New("curated feature import failed")
var ErrConflict=errors.New("curated observation conflicts at the same timestamp")
type Record struct{
	AnimeID string `json:"animeId"`
	FeatureKey string `json:"featureKey"`
	Value *float64 `json:"value"`
	Confidence *float64 `json:"confidence"`
	Reference string `json:"reference"`
}
type Snapshot struct{
	Version int `json:"version"`
	ObservedAt time.Time `json:"observedAt"`
	Records []Record `json:"records"`
}
type Result struct{Inserted int `json:"inserted"`;Unchanged int `json:"unchanged"`}
func(s Snapshot)Validate(now time.Time)error{
	if s.Version!=1||s.ObservedAt.IsZero()||s.ObservedAt.After(now)||len(s.Records)==0||len(s.Records)>1000{return ErrInvalid}
	seen:=map[string]bool{}
	for _,r:=range s.Records{
		var id pgtype.UUID
		if len(r.AnimeID)!=36||id.Scan(r.AnimeID)!=nil||!id.Valid||len(r.FeatureKey)==0||len(r.FeatureKey)>128||strings.TrimSpace(r.FeatureKey)!=r.FeatureKey{return ErrInvalid}
		if r.Value==nil||r.Confidence==nil||math.IsNaN(*r.Value)||math.IsInf(*r.Value,0)||*r.Value<0||*r.Value>1||math.IsNaN(*r.Confidence)||math.IsInf(*r.Confidence,0)||*r.Confidence<=0||*r.Confidence>1{return ErrInvalid}
		if math.Abs(*r.Value*10000-math.Round(*r.Value*10000))>1e-8||math.Abs(*r.Confidence*10000-math.Round(*r.Confidence*10000))>1e-8{return ErrInvalid}
		ref,err:=url.Parse(r.Reference)
		if err!=nil||len(r.Reference)>2048||ref.Scheme!="https"||ref.Hostname()==""||ref.User!=nil||ref.RawQuery!=""||ref.Fragment!=""{return ErrInvalid}
		key:=strings.ToLower(r.AnimeID)+":"+r.FeatureKey
		if seen[key]{return ErrInvalid};seen[key]=true
	}
	return nil
}
func Decode(r io.Reader,now time.Time)(Snapshot,error){
	b,err:=io.ReadAll(io.LimitReader(r,(1<<20)+1))
	if err!=nil||len(b)>1<<20{return Snapshot{},ErrInvalid}
	var s Snapshot
	d:=json.NewDecoder(strings.NewReader(string(b)));d.DisallowUnknownFields()
	var extra any
	if d.Decode(&s)!=nil||d.Decode(&extra)!=io.EOF||s.Validate(now)!=nil{return Snapshot{},ErrInvalid}
	return s,nil
}
func Apply(ctx context.Context,conn *pgx.Conn,s Snapshot,now time.Time)(Result,error){
	if s.Validate(now)!=nil{return Result{},ErrInvalid}
	s.ObservedAt=s.ObservedAt.UTC().Truncate(time.Microsecond)
	out,err:=apply(ctx,conn,s)
	if err!=nil{if errors.Is(err,ErrConflict){return Result{},ErrConflict};return Result{},ErrImport}
	return out,nil
}
func apply(ctx context.Context,conn *pgx.Conn,s Snapshot)(Result,error){
	out:=Result{}
	tx,err:=conn.Begin(ctx)
	if err!=nil{return out,err}
	defer tx.Rollback(context.Background())
	// Coordinate supported catalog/curation writers without locking private rows.
	if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock(742019684)`);err!=nil{return out,err}
	for _,r:=range s.Records{
		var feature string
		err=tx.QueryRow(ctx,`SELECT f.id::text FROM anime_features f WHERE f.feature_key=$1 AND f.active AND NOT f.spoiler_sensitive AND EXISTS(SELECT 1 FROM anime WHERE id=$2)`,r.FeatureKey,r.AnimeID).Scan(&feature)
		if err!=nil{return out,err}
		var existing int
		var same bool
		err=tx.QueryRow(ctx,`SELECT count(*),COALESCE(bool_and(value=$4 AND confidence=$5 AND evidence=jsonb_build_object('reference',$6::text)),false)
FROM anime_feature_observations WHERE anime_id=$1 AND feature_id=$2 AND observed_at=$3 AND source_kind='CURATED' AND scope='SERIES' AND spoiler_level='NONE'`,
			r.AnimeID,feature,s.ObservedAt,*r.Value,*r.Confidence,r.Reference).Scan(&existing,&same)
		if err!=nil{return out,err}
		if existing>0{if !same{return out,ErrConflict};out.Unchanged++;continue}
		if _,err=tx.Exec(ctx,`INSERT INTO anime_feature_observations(anime_id,feature_id,value,confidence,source_kind,evidence,scope,spoiler_level,observed_at)
VALUES($1,$2,$3,$4,'CURATED',jsonb_build_object('reference',$5::text),'SERIES','NONE',$6)`,r.AnimeID,feature,*r.Value,*r.Confidence,r.Reference,s.ObservedAt);err!=nil{return out,err}
		out.Inserted++
	}
	return out,tx.Commit(ctx)
}
