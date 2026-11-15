package postgres

import (
	"context"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/khordestan/anime-intelligence/internal/modules/intelligence"
	"github.com/khordestan/anime-intelligence/internal/platform/store"
)

var ErrPreference = errors.New("invalid preference weight")
type FeaturePreference struct {
	Key string `json:"key"`
	Name string `json:"name"`
	Category string `json:"category"`
	Weight *float64 `json:"weight"`
}
type PersonalMatches struct {
	Items []intelligence.Recommendation `json:"items"`
	AlgorithmVersion string `json:"algorithmVersion"`
	CandidateLimit int `json:"candidateLimit"`
	RatingLimit int `json:"ratingLimit"`
}
// preferenceTx uses a consistent snapshot for evidence and the owner's inputs,
// preserving account-before-session locking and transaction-local forced RLS.
func(r *Repository) preferenceTx(ctx context.Context)(pgx.Tx,User,error){
	tx,u,err:=r.db.sessionTxOptions(ctx,r.token,pgx.TxOptions{IsoLevel:pgx.RepeatableRead})
	if err!=nil{return nil,User{},err}
	if !u.EmailVerified{rollback(tx);return nil,User{},store.ErrEmailUnverified}
	return tx,u,nil
}
func(r *Repository) FeaturePreferences(ctx context.Context)([]FeaturePreference,error){
	tx,u,err:=r.preferenceTx(ctx)
	if err!=nil{return nil,err}
	defer rollback(tx)
	rows,err:=tx.Query(ctx,`SELECT f.feature_key,f.display_name,f.category,p.preference_weight::float8
FROM anime_features f LEFT JOIN user_preferences p ON p.feature_id=f.id AND p.user_id=$1 AND p.origin='DECLARED'
WHERE f.active AND NOT f.spoiler_sensitive ORDER BY f.feature_key LIMIT 256`,u.ID)
	if err!=nil{return nil,err}
	out:=[]FeaturePreference{}
	for rows.Next(){
		var p FeaturePreference
		if err=rows.Scan(&p.Key,&p.Name,&p.Category,&p.Weight);err!=nil{rows.Close();return nil,err}
		out=append(out,p)
	}
	rows.Close()
	if err=rows.Err();err!=nil{return nil,err}
	return out,tx.Commit(ctx)
}
func(r *Repository) SetFeaturePreference(ctx context.Context,key string,weight *float64)(bool,error){
	if weight!=nil && (math.IsNaN(*weight)||math.IsInf(*weight,0)||*weight < -1||*weight>1){return false,ErrPreference}
	tx,u,err:=r.db.productTx(ctx,r.token)
	if err!=nil{return false,err}
	defer rollback(tx)
	var feature string
	err=tx.QueryRow(ctx,`SELECT id::text FROM anime_features WHERE feature_key=$1 AND active AND NOT spoiler_sensitive`,key).Scan(&feature)
	if errors.Is(err,pgx.ErrNoRows){return false,nil}
	if err!=nil{return false,err}
	if weight==nil{
		_,err=tx.Exec(ctx,`DELETE FROM user_preferences WHERE user_id=$1 AND feature_id=$2 AND origin='DECLARED'`,u.ID,feature)
	}else{
		_,err=tx.Exec(ctx,`INSERT INTO user_preferences(user_id,feature_id,preference_weight,origin,confidence) VALUES($1,$2,$3,'DECLARED',1)
ON CONFLICT(user_id,feature_id,origin) DO UPDATE SET preference_weight=excluded.preference_weight,confidence=1,updated_at=now()`,u.ID,feature,*weight)
	}
	if err!=nil{return false,err}
	return true,tx.Commit(ctx)
}
func(r *Repository) PersonalMatches(ctx context.Context,limit int)(PersonalMatches,error){
	if limit<1{limit=20};if limit>100{limit=100}
	out:=PersonalMatches{Items:[]intelligence.Recommendation{},AlgorithmVersion:intelligence.PersonalMatchAlgorithmVersion,CandidateLimit:500,RatingLimit:1000}
	tx,u,err:=r.preferenceTx(ctx)
	if err!=nil{return out,err}
	defer rollback(tx)
	rows,err:=tx.Query(ctx,`SELECT f.feature_key,p.preference_weight::float8,p.confidence::float8 FROM user_preferences p
JOIN anime_features f ON f.id=p.feature_id WHERE p.user_id=$1 AND p.origin='DECLARED' AND f.active AND NOT f.spoiler_sensitive ORDER BY f.feature_key LIMIT 256`,u.ID)
	if err!=nil{return out,err}
	explicit:=map[string]intelligence.Preference{}
	for rows.Next(){
		var key string;var p intelligence.Preference
		if err=rows.Scan(&key,&p.Weight,&p.Confidence);err!=nil{rows.Close();return out,err}
		explicit[key]=p
	}
	rows.Close();if err=rows.Err();err!=nil{return out,err}
	rows,err=tx.Query(ctx,`SELECT anime_id::text,score FROM user_ratings WHERE user_id=$1 AND scope='SERIES' AND season_key IS NULL ORDER BY rated_at DESC,id LIMIT 1000`,u.ID)
	if err!=nil{return out,err}
	rated:=[]intelligence.RatedAnime{}
	for rows.Next(){
		var a intelligence.RatedAnime
		if err=rows.Scan(&a.AnimeID,&a.Score);err!=nil{rows.Close();return out,err}
		rated=append(rated,a)
	}
	rows.Close();if err=rows.Err();err!=nil{return out,err}
	rows,err=tx.Query(ctx,`SELECT a.id::text,a.canonical_title FROM anime a WHERE NOT EXISTS
(SELECT 1 FROM user_anime ua WHERE ua.user_id=$1 AND ua.anime_id=a.id) ORDER BY a.canonical_title,a.id LIMIT 500`,u.ID)
	if err!=nil{return out,err}
	candidates:=[]intelligence.Candidate{}
	ids:=[]string{}
	for rows.Next(){
		var a intelligence.Candidate
		if err=rows.Scan(&a.AnimeID,&a.Title);err!=nil{rows.Close();return out,err}
		candidates=append(candidates,a);ids=append(ids,a.AnimeID)
	}
	rows.Close();if err=rows.Err();err!=nil{return out,err}
	for _,a:=range rated{ids=append(ids,a.AnimeID)}
	// Only current, referenced, curated series-level non-spoiler observations
	// participate. Provider genres/tags and model guesses are not feature scores.
	rows,err=tx.Query(ctx,`SELECT DISTINCT ON(o.anime_id,o.feature_id) o.anime_id::text,f.feature_key,o.value::float8,o.confidence::float8
FROM anime_feature_observations o JOIN anime_features f ON f.id=o.feature_id
WHERE o.anime_id=ANY($1::uuid[]) AND f.active AND NOT f.spoiler_sensitive AND o.source_kind='CURATED'
AND o.scope='SERIES' AND o.spoiler_level='NONE' AND o.observed_at<=now() AND o.confidence>0
AND jsonb_typeof(o.evidence->'reference')='string' AND length(btrim(o.evidence->>'reference')) BETWEEN 1 AND 2048
ORDER BY o.anime_id,o.feature_id,o.observed_at DESC,o.id DESC`,ids)
	if err!=nil{return out,err}
	features:=map[string][]intelligence.FeatureValue{}
	for rows.Next(){
		var id string;var feature intelligence.FeatureValue
		if err=rows.Scan(&id,&feature.Key,&feature.Value,&feature.Confidence);err!=nil{rows.Close();return out,err}
		features[id]=append(features[id],feature)
	}
	rows.Close();if err=rows.Err();err!=nil{return out,err}
	for i:=range rated{rated[i].Features=features[rated[i].AnimeID]}
	for i:=range candidates{candidates[i].Features=features[candidates[i].AnimeID]}
	profile:=intelligence.BuildTasteProfile(rated,explicit)
	out.Items=intelligence.RankPersonalMatches(profile,candidates,nil,intelligence.Options{Limit:limit})
	return out,tx.Commit(ctx)
}
