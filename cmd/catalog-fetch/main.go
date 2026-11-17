// catalog-fetch normalizes explicit reviewed provider items without user accounts.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/khordestan/anime-intelligence/internal/providers/wikidata"
 "github.com/khordestan/anime-intelligence/internal/providers/tvmaze"
 "github.com/khordestan/anime-intelligence/internal/modules/catalog/feed"
)
func run() error {
	ids:=flag.String("ids","","Provider IDs: up to 20 Wikidata Q IDs, or one reviewed TVmaze show-N-season-N pair")
 provider:=flag.String("provider","wikidata","Provider: wikidata or tvmaze")
 reviewed:=flag.Bool("reviewed-anime-seasons",false,"Explicitly reviewed anime classification and season identities for TVmaze")
 flag.Parse()
 if flag.NArg()!=0{return feed.ErrInvalid}
	items:=strings.Split(*ids,",")
	for i:=range items {items[i]=strings.TrimSpace(items[i])}
	ctx,cancel:=context.WithTimeout(context.Background(),time.Minute)
	defer cancel()
	var snapshot feed.Snapshot
 var err error
 switch *provider{
 case "wikidata":snapshot,err=wikidata.New().Fetch(ctx,items)
 case "tvmaze":snapshot,err=tvmaze.New(*reviewed).Fetch(ctx,items)
 default:return feed.ErrInvalid
 }
	if err!=nil {return err}
	return json.NewEncoder(os.Stdout).Encode(snapshot)
}
func main() {
	if err:=run();err!=nil {
		var limited *wikidata.RateLimitError
  var tvLimited *tvmaze.RateLimitError
		if errors.As(err,&limited) {fmt.Fprintln(os.Stderr,"catalog fetch rate limited; wait at least",int(limited.Wait.Seconds()),"seconds")
		} else if errors.As(err,&tvLimited){fmt.Fprintln(os.Stderr,"catalog fetch rate limited; wait at least",int(tvLimited.Wait.Seconds()),"seconds")} else {fmt.Fprintln(os.Stderr,"catalog fetch failed; check provider review/configuration or retry later if busy")}
		os.Exit(1)
	}
}
