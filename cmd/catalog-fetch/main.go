// catalog-fetch reads explicit CC0 Wikidata anime items without user accounts.
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
)
func run() error {
	ids:=flag.String("ids","","Comma-separated supported Wikidata anime work IDs (TV/season/movie/OVA/ONA; maximum 20)")
	flag.Parse()
	items:=strings.Split(*ids,",")
	for i:=range items {items[i]=strings.TrimSpace(items[i])}
	ctx,cancel:=context.WithTimeout(context.Background(),time.Minute)
	defer cancel()
	snapshot,err:=wikidata.New().Fetch(ctx,items)
	if err!=nil {return err}
	return json.NewEncoder(os.Stdout).Encode(snapshot)
}
func main() {
	if err:=run();err!=nil {
		var limited *wikidata.RateLimitError
		if errors.As(err,&limited) {fmt.Fprintln(os.Stderr,"catalog fetch rate limited; wait at least",int(limited.Wait.Seconds()),"seconds")
		} else {fmt.Fprintln(os.Stderr,"catalog fetch failed; retry later if the source is busy")}
		os.Exit(1)
	}
}
