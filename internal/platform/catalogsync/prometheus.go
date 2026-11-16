package catalogsync

import("fmt";"strings")

// Prometheus exports only fixed aggregate gauges, never provider identifiers,
// raw metadata, source URLs, or any account data.
func Prometheus(status Status)(string,error){
 if status.AsOf.IsZero()||status.AsOf.Unix()<0||status.StaleAfterSeconds<60||status.StaleAfterSeconds>2592000{return "",ErrStatus}
 gauges:=[]struct{name,help string;value int64}{
 {"snapshot_timestamp_seconds","Unix timestamp of the diagnostic snapshot.",status.AsOf.Unix()},
 {"stale_after_seconds","Configured title and source freshness threshold.",status.StaleAfterSeconds},
 {"titles","Canonical catalog titles.",int64(status.Titles)},
 {"observed_titles","Titles with selected source observations.",int64(status.ObservedTitles)},
 {"unobserved_titles","Titles without selected source observations.",int64(status.UnobservedTitles)},
 {"stale_titles","Selected title observations outside the freshness window.",int64(status.StaleTitles)},
 {"future_titles","Selected title observations in the future.",int64(status.FutureTitles)},
 {"stale_sources","Enabled sources with missing policy, missing observations or stale clocks.",int64(status.StaleSources)},
 {"episode_schedules","Latest episode and track schedule records.",int64(status.EpisodeSchedules)},
 {"confirmed_episodes","Latest episode and track records with release confirmation.",int64(status.ConfirmedEpisodes)},
 {"metadata_episode_totals","Selected observations with a numeric episode total.",int64(status.Coverage.EpisodeTotals)},
 {"metadata_synopses","Selected observations with a synopsis.",int64(status.Coverage.Synopses)},
 {"metadata_synonyms","Selected observations with synonyms.",int64(status.Coverage.Synonyms)},
 {"metadata_genres","Selected observations with genres.",int64(status.Coverage.Genres)},
 {"metadata_tags","Selected observations with tags.",int64(status.Coverage.Tags)},
 {"metadata_themes","Selected observations with themes.",int64(status.Coverage.Themes)},
 {"metadata_demographics","Selected observations with demographics.",int64(status.Coverage.Demographics)},
 {"metadata_studios","Selected observations with studios.",int64(status.Coverage.Studios)},
 {"metadata_relations","Selected observations with relation claims.",int64(status.Coverage.Relations)},
 }
 for _,g:=range gauges{if g.value<0{return "",ErrStatus}}
 if status.ObservedTitles+status.UnobservedTitles!=status.Titles||status.StaleTitles>status.ObservedTitles||status.FutureTitles>status.StaleTitles||status.ConfirmedEpisodes>status.EpisodeSchedules{return "",ErrStatus}
 // Deficits include every canonical title without a selected value, including
 // completely unobserved titles. Presence does not establish factual accuracy.
 for _,g:=range gauges{
  if strings.HasPrefix(g.name,"metadata_"){
   if g.value>int64(status.ObservedTitles){return "",ErrStatus}
  }
 }
 presence:=append([]struct{name,help string;value int64}(nil),gauges...)
 for _,g:=range presence{
  if strings.HasPrefix(g.name,"metadata_"){
   gauges=append(gauges,struct{name,help string;value int64}{
    "missing_"+g.name,"Canonical titles without this selected metadata field.",int64(status.Titles)-g.value,
   })
  }
 }
 var out strings.Builder
 for _,g:=range gauges{fmt.Fprintf(&out,"# HELP anime_catalog_%s %s\n# TYPE anime_catalog_%s gauge\nanime_catalog_%s %d\n",g.name,g.help,g.name,g.name,g.value)}
 return out.String(),nil
}
