package store

import (
 "crypto/rand"
 "encoding/hex"
 "encoding/json"
 "fmt"
 "os"
 "path/filepath"
 "sort"
 "strings"
 "sync"
 "time"
)

type Anime struct { Synopsis string `json:"synopsis,omitempty"`; Season string `json:"season,omitempty"`; CoverImageURL string `json:"coverImageUrl,omitempty"`; EpisodeCountKnown bool `json:"episodeCountKnown"`; ID string `json:"id"`; Title string `json:"title"`; NativeTitle string `json:"nativeTitle,omitempty"`; MediaType string `json:"mediaType"`; ReleaseYear int `json:"releaseYear,omitempty"`; Status string `json:"status"`; EpisodeCount int `json:"episodeCount,omitempty"` }
type Entry struct { WatchCycle int `json:"watchCycle"`; ExactProgress bool `json:"exactProgress"`;  ID string `json:"id"`; AnimeID string `json:"animeId"`; Title string `json:"title"`; WatchStatus string `json:"watchStatus"`; EpisodesWatched int `json:"episodesWatched"`; TotalEpisodes int `json:"totalEpisodes,omitempty"`; Rating int `json:"rating,omitempty"`; UpdatedAt time.Time `json:"updatedAt"` }
type diskState struct { Library []Entry `json:"library"` }
type Store struct { mu sync.RWMutex; path string; catalog []Anime; library map[string]Entry }

func New(path string) (*Store,error) {
 s:=&Store{path:path,library:map[string]Entry{},catalog:[]Anime{
 {ID:"10000000-0000-4000-8000-000000000001",Title:"Solo Leveling",NativeTitle:"나 혼자만 레벨업",MediaType:"TV",ReleaseYear:2024,Status:"FINISHED",EpisodeCount:12},
 {ID:"10000000-0000-4000-8000-000000000002",Title:"Attack on Titan",NativeTitle:"進撃の巨人",MediaType:"TV",ReleaseYear:2013,Status:"FINISHED",EpisodeCount:25},
 {ID:"10000000-0000-4000-8000-000000000003",Title:"Death Note",NativeTitle:"デスノート",MediaType:"TV",ReleaseYear:2006,Status:"FINISHED",EpisodeCount:37},
 {ID:"10000000-0000-4000-8000-000000000004",Title:"Fullmetal Alchemist: Brotherhood",NativeTitle:"鋼の錬金術師 FULLMETAL ALCHEMIST",MediaType:"TV",ReleaseYear:2009,Status:"FINISHED",EpisodeCount:64},
 {ID:"10000000-0000-4000-8000-000000000005",Title:"Hunter x Hunter (2011)",NativeTitle:"HUNTER×HUNTER",MediaType:"TV",ReleaseYear:2011,Status:"FINISHED",EpisodeCount:148},
 {ID:"10000000-0000-4000-8000-000000000006",Title:"One Punch Man",NativeTitle:"ワンパンマン",MediaType:"TV",ReleaseYear:2015,Status:"FINISHED",EpisodeCount:12},
 {ID:"10000000-0000-4000-8000-000000000007",Title:"Steins;Gate",NativeTitle:"シュタインズ・ゲート",MediaType:"TV",ReleaseYear:2011,Status:"FINISHED",EpisodeCount:24},
 {ID:"10000000-0000-4000-8000-000000000008",Title:"Overlord",NativeTitle:"オーバーロード",MediaType:"TV",ReleaseYear:2015,Status:"FINISHED",EpisodeCount:13},
 }}
 for i := range s.catalog { s.catalog[i].EpisodeCountKnown = true }
 if path!="" { if err:=s.load(); err!=nil&&!os.IsNotExist(err){return nil,err} }
 return s,nil
}
func(s *Store)load()error{b,e:=os.ReadFile(s.path);if e!=nil{return e};var d diskState;if e=json.Unmarshal(b,&d);e!=nil{return fmt.Errorf("decode local data file: %w",e)};for _,x:=range d.Library{if x.WatchCycle<1{x.WatchCycle=1};s.library[x.ID]=x};return nil}
func(s *Store)persistLocked()error{if s.path==""{return nil};if e:=os.MkdirAll(filepath.Dir(s.path),0755);e!=nil{return e};d:=diskState{Library:make([]Entry,0,len(s.library))};for _,x:=range s.library{d.Library=append(d.Library,x)};sort.Slice(d.Library,func(i,j int)bool{return d.Library[i].UpdatedAt.After(d.Library[j].UpdatedAt)});b,e:=json.MarshalIndent(d,"","  ");if e!=nil{return e};tmp:=s.path+".tmp";if e=os.WriteFile(tmp,b,0600);e!=nil{return e};return os.Rename(tmp,s.path)}
func(s *Store)Catalog(q string,limit,offset int)[]Anime{s.mu.RLock();defer s.mu.RUnlock();q=strings.ToLower(strings.TrimSpace(q));out:=[]Anime{};for _,a:=range s.catalog{if q==""||strings.Contains(strings.ToLower(a.Title),q)||strings.Contains(strings.ToLower(a.NativeTitle),q){out=append(out,a)}};if offset>=len(out){return []Anime{}};out=out[offset:];if limit<len(out){out=out[:limit]};return out}
func(s *Store)GetAnime(id string)(Anime,bool){s.mu.RLock();defer s.mu.RUnlock();for _,a:=range s.catalog{if a.ID==id{return a,true}};return Anime{},false}
func(s *Store)Library()[]Entry{s.mu.RLock();defer s.mu.RUnlock();out:=make([]Entry,0,len(s.library));for _,e:=range s.library{out=append(out,e)};sort.Slice(out,func(i,j int)bool{return out[i].UpdatedAt.After(out[j].UpdatedAt)});return out}
func(s *Store)Add(animeID,status string,episodes int)(Entry,error){a,ok:=s.GetAnime(animeID);if !ok{return Entry{},fmt.Errorf("anime_not_found")};s.mu.Lock();defer s.mu.Unlock();for _,e:=range s.library{if e.AnimeID==animeID{e.WatchStatus=status;e.EpisodesWatched=episodes;e.UpdatedAt=time.Now().UTC();s.library[e.ID]=e;if err:=s.persistLocked();err!=nil{return Entry{},err};return e,nil}};id,err:=newUUID();if err!=nil{return Entry{},err};e:=Entry{WatchCycle:1,ID:id,AnimeID:animeID,Title:a.Title,WatchStatus:status,EpisodesWatched:episodes,TotalEpisodes:a.EpisodeCount,UpdatedAt:time.Now().UTC()};s.library[id]=e;if err=s.persistLocked();err!=nil{return Entry{},err};return e,nil}
func(s *Store)Update(id string,status *string,episodes *int)(Entry,bool,error){s.mu.Lock();defer s.mu.Unlock();e,ok:=s.library[id];if !ok{return Entry{},false,nil};if status!=nil{e.WatchStatus=*status};if episodes!=nil{e.EpisodesWatched=*episodes};e.UpdatedAt=time.Now().UTC();s.library[id]=e;if err:=s.persistLocked();err!=nil{return Entry{},true,err};return e,true,nil}
func(s *Store)Rate(animeID string,score int)(Entry,bool,error){s.mu.Lock();defer s.mu.Unlock();for id,e:=range s.library{if e.AnimeID==animeID{e.Rating=score;e.UpdatedAt=time.Now().UTC();s.library[id]=e;if err:=s.persistLocked();err!=nil{return Entry{},true,err};return e,true,nil}};return Entry{},false,nil}
func newUUID()(string,error){b:=make([]byte,16);if _,e:=rand.Read(b);e!=nil{return "",e};b[6]=(b[6]&0x0f)|0x40;b[8]=(b[8]&0x3f)|0x80;h:=hex.EncodeToString(b);return h[:8]+"-"+h[8:12]+"-"+h[12:16]+"-"+h[16:20]+"-"+h[20:],nil}
