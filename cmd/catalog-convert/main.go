// catalog-convert normalizes lawfully obtained offline provider responses.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
	"github.com/khordestan/anime-intelligence/internal/providers/jikan"
)
func run() error {
	input:=flag.String("input","","Offline Jikan v4 full anime response")
	episodes:=flag.String("episodes","","Optional ordered comma-separated episode-page JSON paths")
	observed:=flag.String("observed-at","","Actual source observation timestamp in RFC3339 (required)")
	flag.Parse()
	at,err:=time.Parse(time.RFC3339Nano,*observed)
	if err!=nil {return feed.ErrInvalid}
	file,err:=os.Open(*input)
	if err!=nil {return feed.ErrInvalid}
	defer file.Close()
	now:=time.Now().UTC()
	snapshot,err:=jikan.DecodeFull(file,at,now)
	if err!=nil {return err}
	if *episodes!="" {
		paths:=strings.Split(*episodes,",")
		if len(paths)>100 {return feed.ErrInvalid}
		for i,path:=range paths {
			f,err:=os.Open(path)
			if err!=nil {return feed.ErrInvalid}
			page,hasNext,err:=jikan.EpisodePage(f)
			f.Close()
			if err!=nil || hasNext!=(i<len(paths)-1) {return feed.ErrInvalid}
			snapshot.Titles[0].Episodes=append(snapshot.Titles[0].Episodes,page...)
		}
	}
	if snapshot.Validate(now)!=nil {return feed.ErrInvalid}
	return json.NewEncoder(os.Stdout).Encode(snapshot)
}
func main() {
	if run()!=nil {fmt.Fprintln(os.Stderr,"provider conversion failed");os.Exit(1)}
}
