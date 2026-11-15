// Package feed validates provider-neutral catalog snapshots before persistence.
// It performs no network calls and never infers availability from a schedule.
package feed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"time"
)

const MaxBytes = 8 << 20
var ErrInvalid = errors.New("invalid catalog snapshot")

type Snapshot struct {
	Version int `json:"version"`
	Provider string `json:"provider"`
	ObservedAt time.Time `json:"observedAt"`
	Titles []Title `json:"titles"`
}
type Title struct {
	ExternalID string `json:"externalId"`
	Title string `json:"title"`
	NativeTitle string `json:"nativeTitle,omitempty"`
	Synonyms []string `json:"synonyms,omitempty"`
	Synopsis string `json:"synopsis,omitempty"`
	MediaType string `json:"mediaType"`
	Status string `json:"status"`
	ReleaseYear int `json:"releaseYear,omitempty"`
	Season string `json:"season,omitempty"`
	EpisodeCount *int `json:"episodeCount,omitempty"`
	Genres []string `json:"genres,omitempty"`
	Tags []string `json:"tags,omitempty"`
	Studios []string `json:"studios,omitempty"`
	Relations []Relation `json:"relations,omitempty"`
	Episodes []Episode `json:"episodes,omitempty"`
}
type Relation struct {
	ExternalID string `json:"externalId"`
	Type string `json:"type"`
}
type Episode struct {
	Number int `json:"number"`
	Track string `json:"track"`
	ScheduledAt *time.Time `json:"scheduledAt,omitempty"`
	ReleasedAt *time.Time `json:"releasedAt,omitempty"`
}

// Decode rejects unknown fields, trailing documents and oversized input. Errors
// deliberately exclude raw provider content, which could contain credentials.
func Decode(r io.Reader, now time.Time) (Snapshot, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil || len(b) > MaxBytes { return Snapshot{}, ErrInvalid }
	var s Snapshot
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if d.Decode(&s) != nil { return Snapshot{}, ErrInvalid }
	var extra any
	if d.Decode(&extra) != io.EOF { return Snapshot{}, ErrInvalid }
	if s.Validate(now) != nil { return Snapshot{}, ErrInvalid }
	return s, nil
}

func oneOf(s string, values ...string) bool {
	for _, v := range values { if s == v { return true } }
	return false
}
func identifier(s string) bool {
	if len(s) == 0 || len(s) > 128 { return false }
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') { return false }
	}
	return true
}
func labels(xs []string) bool {
	if len(xs) > 256 { return false }
	seen := map[string]bool{}
	for _, x := range xs {
		key := strings.ToLower(strings.TrimSpace(x))
		if key == "" || len(x) > 256 || seen[key] { return false }
		seen[key] = true
	}
	return true
}

// Relations may reference titles outside a partial snapshot. They are provider
// identities, never canonical IDs; entity resolution must precede database writes.
func (s Snapshot) Validate(now time.Time) error {
	if s.Version != 1 || !identifier(s.Provider) || s.ObservedAt.IsZero() || s.ObservedAt.After(now) || len(s.Titles) > 10000 { return ErrInvalid }
	ids := map[string]bool{}
	for _, t := range s.Titles {
		if !identifier(t.ExternalID) || ids[t.ExternalID] || strings.TrimSpace(t.Title) == "" || len(t.Title) > 1024 || len(t.NativeTitle) > 1024 || len(t.Synopsis) > 65536 { return ErrInvalid }
		ids[t.ExternalID] = true
		if !oneOf(t.MediaType,"TV","MOVIE","OVA","ONA","SPECIAL","MUSIC","UNKNOWN") || !oneOf(t.Status,"UNKNOWN","ANNOUNCED","RELEASING","FINISHED","CANCELLED","HIATUS") { return ErrInvalid }
		if t.ReleaseYear != 0 && (t.ReleaseYear < 1900 || t.ReleaseYear > 9999) { return ErrInvalid }
		if t.Season != "" && !oneOf(t.Season,"WINTER","SPRING","SUMMER","FALL") { return ErrInvalid }
		if t.EpisodeCount != nil && (*t.EpisodeCount < 0 || *t.EpisodeCount > 100000) { return ErrInvalid }
		if !labels(t.Synonyms) || !labels(t.Genres) || !labels(t.Tags) || !labels(t.Studios) || len(t.Episodes) > 100000 || len(t.Relations) > 1000 { return ErrInvalid }
		relations := map[string]bool{}
		for _, r := range t.Relations {
			key := r.Type + ":" + r.ExternalID
			if !identifier(r.ExternalID) || r.ExternalID == t.ExternalID || !oneOf(r.Type,"PREQUEL","SEQUEL","SIDE_STORY","ALTERNATIVE","PARENT","SUMMARY") || relations[key] { return ErrInvalid }
			relations[key] = true
		}
		episodes := map[string]bool{}
		for _, e := range t.Episodes {
			key := episodeKey(e)
			if e.Number < 1 || e.Number > 100000 || !oneOf(e.Track,"RAW","SUB","DUB","UNKNOWN") || episodes[key] { return ErrInvalid }
			if t.EpisodeCount != nil && e.Number > *t.EpisodeCount { return ErrInvalid }
			if e.ScheduledAt != nil && e.ScheduledAt.IsZero() { return ErrInvalid }
			if e.ReleasedAt != nil && (e.ReleasedAt.IsZero() || e.ReleasedAt.After(s.ObservedAt)) { return ErrInvalid }
			episodes[key] = true
		}
	}
	return nil
}

func episodeKey(e Episode) string {
	// JSON is unambiguous and independent of locale or time formatting.
	b, _ := json.Marshal([]any{e.Number,e.Track})
	return string(b)
}

type Event struct {
	Key string `json:"key"`
	Type string `json:"type"`
	Provider string `json:"provider"`
	ExternalID string `json:"externalId"`
	Number int `json:"number"`
	Track string `json:"track"`
	ReleasedAt time.Time `json:"releasedAt"`
}

// Releases compares snapshots of the same provider. A first snapshot establishes
// a baseline, avoiding a historical alert flood. Missing records never retract
// releases. Durable consumers must retain event keys across partial snapshots.
// A corrected release timestamp has the same key and must not notify twice.
func Releases(previous *Snapshot, next Snapshot, now time.Time) ([]Event,error) {
	if next.Validate(now) != nil { return nil, ErrInvalid }
	out := []Event{}
	if previous == nil { return out,nil }
	if previous.Validate(now) != nil || previous.Provider != next.Provider || !next.ObservedAt.After(previous.ObservedAt) { return nil,ErrInvalid }
	released := map[string]bool{}
	for _, t := range previous.Titles {
		for _, e := range t.Episodes {
			if e.ReleasedAt != nil { released[t.ExternalID+":"+episodeKey(e)] = true }
		}
	}
	for _, t := range next.Titles {
		for _, e := range t.Episodes {
			if e.ReleasedAt == nil || released[t.ExternalID+":"+episodeKey(e)] { continue }
			b, _ := json.Marshal([]any{"episode_released",next.Provider,t.ExternalID,e.Number,e.Track})
			sum := sha256.Sum256(b)
			out = append(out,Event{Key:hex.EncodeToString(sum[:]),Type:"EPISODE_RELEASED",Provider:next.Provider,ExternalID:t.ExternalID,Number:e.Number,Track:e.Track,ReleasedAt:*e.ReleasedAt})
		}
	}
	sort.Slice(out,func(i,j int) bool { return out[i].Key < out[j].Key })
	return out,nil
}
