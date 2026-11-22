package postgres

import (
	"context"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
type PreferenceSignal struct {
	Weight float64 `json:"weight"`
	Confidence float64 `json:"confidence"`
	RatingCount int `json:"ratingCount,omitempty"`
}
type FeatureTaste struct {
	Key string `json:"key"`
	Declared *PreferenceSignal `json:"declared,omitempty"`
	Inferred *PreferenceSignal `json:"inferred,omitempty"`
}
type PersonalMatches struct {
	Profile []FeatureTaste `json:"profile"`
	Items []intelligence.Recommendation `json:"items"`
	AlgorithmVersion string `json:"algorithmVersion"`
	CandidateLimit int `json:"candidateLimit"`
	RatingLimit int `json:"ratingLimit"`
}
// verifiedSnapshotTx uses a consistent snapshot for public facts and the owner's inputs,
// preserving account-before-session locking and transaction-local forced RLS.
func(r *Repository) verifiedSnapshotTx(ctx context.Context)(pgx.Tx,User,error){
	tx,u,err:=r.db.sessionTxOptions(ctx,r.token,pgx.TxOptions{IsoLevel:pgx.RepeatableRead})
	if err!=nil{return nil,User{},err}
	if !u.EmailVerified{rollback(tx);return nil,User{},store.ErrEmailUnverified}
	return tx,u,nil
}
func retrySnapshotRead[T any](ctx context.Context,read func()(T,error))(T,error){
	var out T;var err error
	for range 3{
		out,err=read()
		var conflict *pgconn.PgError
		if !errors.As(err,&conflict)||(conflict.Code!="40001"&&conflict.Code!="40P01")||ctx.Err()!=nil{return out,err}
	}
	return out,err
}
func(r *Repository) FeaturePreferences(ctx context.Context)([]FeaturePreference,error){
	return retrySnapshotRead(ctx,func()([]FeaturePreference,error){return r.featurePreferencesSnapshot(ctx)})
}
func(r *Repository) featurePreferencesSnapshot(ctx context.Context)([]FeaturePreference,error){
	tx,u,err:=r.verifiedSnapshotTx(ctx)
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
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return retrySnapshotRead(ctx,func()(PersonalMatches,error){return r.personalMatchesSnapshot(ctx,limit)})
}
func(r *Repository) personalMatchesSnapshot(ctx context.Context,limit int)(PersonalMatches,error){
	if limit<1{limit=20};if limit>100{limit=100}
	out:=PersonalMatches{Items:[]intelligence.Recommendation{},Profile:[]FeatureTaste{},AlgorithmVersion:intelligence.PersonalMatchAlgorithmVersion,CandidateLimit:500,RatingLimit:1000}
	tx,u,err:=r.verifiedSnapshotTx(ctx)
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
	ratedIDs := make([]string, 0, len(rated))
	for _, anime := range rated {
		ratedIDs = append(ratedIDs, anime.AnimeID)
	}
	features, err := recommendationFeatures(ctx, tx, ratedIDs)
	if err != nil {
		return out, err
	}
	for i := range rated {
		rated[i].Features = features[rated[i].AnimeID]
	}
	profile := intelligence.BuildTasteProfile(rated, explicit)
	keys:=map[string]bool{}
	for key:=range profile.Explicit{keys[key]=true}
	for key:=range profile.Inferred{keys[key]=true}
	ordered:=make([]string,0,len(keys))
	for key:=range keys{ordered=append(ordered,key)}
	sort.Strings(ordered)
	if len(ordered)>256{ordered=ordered[:256]}
	for _,key:=range ordered{
		signal:=FeatureTaste{Key:key}
		if p,ok:=profile.Explicit[key];ok{signal.Declared=&PreferenceSignal{Weight:p.Weight,Confidence:p.Confidence}}
		if p,ok:=profile.Inferred[key];ok{signal.Inferred=&PreferenceSignal{Weight:p.Weight,Confidence:p.Confidence,RatingCount:profile.InferredRatingCounts[key]}}
		out.Profile=append(out.Profile,signal)
	}
	if scoringKeys := intelligence.PersonalMatchFeatureKeys(profile); len(scoringKeys) != 0 {
		candidates, err := recommendationCandidates(ctx, tx, u.ID, scoringKeys, out.CandidateLimit)
		if err != nil {
			return out, err
		}
		out.Items = intelligence.RankPersonalMatches(profile, candidates, nil, intelligence.Options{Limit: limit})
	}
	return out,tx.Commit(ctx)
}

const eligibleRecommendationObservation = `f.active AND NOT f.spoiler_sensitive AND o.source_kind='CURATED'
	AND o.scope='SERIES' AND o.spoiler_level='NONE' AND o.observed_at<=now() AND o.confidence>0
	AND jsonb_typeof(o.evidence->'reference')='string' AND length(btrim(o.evidence->>'reference')) BETWEEN 1 AND 2048`

func recommendationCandidates(ctx context.Context, tx pgx.Tx, ownerID string, keys []string, limit int) ([]intelligence.Candidate, error) {
	rows, err := tx.Query(ctx, `WITH supported AS (
		SELECT o.anime_id, count(DISTINCT o.feature_id) AS overlap
		FROM anime_feature_observations o JOIN anime_features f ON f.id=o.feature_id
		WHERE f.feature_key=ANY($2::text[]) AND `+eligibleRecommendationObservation+`
		AND NOT EXISTS (SELECT 1 FROM user_anime ua WHERE ua.user_id=$1 AND ua.anime_id=o.anime_id)
		AND NOT EXISTS (SELECT 1 FROM personal_match_dismissals d WHERE d.user_id=$1 AND d.anime_id=o.anime_id)
		GROUP BY o.anime_id
	)
	SELECT a.id::text,a.canonical_title FROM supported JOIN anime a ON a.id=supported.anime_id
	ORDER BY supported.overlap DESC,a.id LIMIT $3`, ownerID, keys, limit)
	if err != nil {
		return nil, err
	}
	var candidates []intelligence.Candidate
	var ids []string
	for rows.Next() {
		var candidate intelligence.Candidate
		if err := rows.Scan(&candidate.AnimeID, &candidate.Title); err != nil {
			rows.Close()
			return nil, err
		}
		candidates = append(candidates, candidate)
		ids = append(ids, candidate.AnimeID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	features, err := recommendationFeatures(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	for i := range candidates {
		candidates[i].Features = features[candidates[i].AnimeID]
	}
	return candidates, nil
}

func recommendationFeatures(ctx context.Context, tx pgx.Tx, ids []string) (map[string][]intelligence.FeatureValue, error) {
	features := make(map[string][]intelligence.FeatureValue)
	if len(ids) == 0 {
		return features, nil
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT ON(o.anime_id,o.feature_id)
		o.anime_id::text,f.feature_key,o.value::float8,o.confidence::float8
		FROM anime_feature_observations o JOIN anime_features f ON f.id=o.feature_id
		WHERE o.anime_id=ANY($1::uuid[]) AND `+eligibleRecommendationObservation+`
		ORDER BY o.anime_id,o.feature_id,o.observed_at DESC,o.id DESC`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var feature intelligence.FeatureValue
		if err := rows.Scan(&id,&feature.Key,&feature.Value,&feature.Confidence); err != nil {
			return nil, err
		}
		features[id] = append(features[id], feature)
	}
	return features, rows.Err()
}

func (r *Repository) SetPersonalMatchDismissed(ctx context.Context, animeID string, dismissed bool) (bool, error) {
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return false, err
	}
	defer rollback(tx)
	if dismissed {
		var found bool
		err = tx.QueryRow(ctx, `WITH title AS (
			SELECT id FROM anime WHERE id=$2
		), added AS (
			INSERT INTO personal_match_dismissals(user_id,anime_id)
			SELECT $1,id FROM title ON CONFLICT (user_id,anime_id) DO NOTHING
		)
		SELECT EXISTS(SELECT 1 FROM title)`, owner.ID, animeID).Scan(&found)
		if err != nil || !found {
			return false, err
		}
	} else {
		if _, err = tx.Exec(ctx, `DELETE FROM personal_match_dismissals WHERE user_id=$1 AND anime_id=$2`, owner.ID, animeID); err != nil {
			return false, err
		}
	}
	return true, tx.Commit(ctx)
}

type DismissedMatch struct {
	AnimeID string `json:"animeId"`
	Title string `json:"title"`
}

type DismissedMatchPage struct {
	Items []DismissedMatch `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}

func (r *Repository) PersonalMatchDismissals(ctx context.Context, after string) (DismissedMatchPage, error) {
	out := DismissedMatchPage{Items: []DismissedMatch{}}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, owner, err := r.db.productTx(ctx, r.token)
	if err != nil {
		return out, err
	}
	defer rollback(tx)
	rows, err := tx.Query(ctx, `SELECT d.anime_id::text,a.canonical_title
		FROM personal_match_dismissals d JOIN anime a ON a.id=d.anime_id
		WHERE d.user_id=$1 AND (NULLIF($2,'') IS NULL OR d.anime_id>NULLIF($2,'')::uuid)
		ORDER BY d.anime_id LIMIT 51`, owner.ID, after)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var item DismissedMatch
		if err := rows.Scan(&item.AnimeID, &item.Title); err != nil {
			return out, err
		}
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	if len(out.Items) > 50 {
		out.Items = out.Items[:50]
		out.NextCursor = out.Items[49].AnimeID
	}
	return out, tx.Commit(ctx)
}

type DiscoveryTitle struct {
	AnimeID string `json:"animeId"`
	Title string `json:"title"`
	EvidenceCount int `json:"evidenceCount"`
}

// DiscoverTitles offers eligible catalog titles even before a taste profile exists.
func (r *Repository) DiscoverTitles(ctx context.Context, limit int) ([]DiscoveryTitle, error) {
	if limit < 1 || limit > 50 {
		limit = 20
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return retrySnapshotRead(ctx, func() ([]DiscoveryTitle, error) {
		tx, owner, err := r.verifiedSnapshotTx(ctx)
		if err != nil {
			return nil, err
		}
		defer rollback(tx)
		rows, err := tx.Query(ctx, `WITH latest AS (
			SELECT DISTINCT ON (o.anime_id,o.feature_id) o.anime_id,o.feature_id,o.value
			FROM anime_feature_observations o JOIN anime_features f ON f.id=o.feature_id
			WHERE `+eligibleRecommendationObservation+`
			  AND NOT EXISTS (SELECT 1 FROM user_anime ua WHERE ua.user_id=$1 AND ua.anime_id=o.anime_id)
			  AND NOT EXISTS (SELECT 1 FROM personal_match_dismissals d WHERE d.user_id=$1 AND d.anime_id=o.anime_id)
			ORDER BY o.anime_id,o.feature_id,o.observed_at DESC,o.id DESC
		), supported AS (
			SELECT anime_id,count(*) AS feature_count FROM latest WHERE value>0 GROUP BY anime_id
		)
		SELECT a.id::text,a.canonical_title,s.feature_count
		FROM supported s JOIN anime a ON a.id=s.anime_id
		ORDER BY s.feature_count DESC,a.id LIMIT $2`, owner.ID, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := []DiscoveryTitle{}
		for rows.Next() {
			var item DiscoveryTitle
			if err := rows.Scan(&item.AnimeID, &item.Title, &item.EvidenceCount); err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return items, tx.Commit(ctx)
	})
}
