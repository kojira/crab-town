// Command town-next runs the next-generation town (internal/next) side by side
// with the current one. It never reads tokens.json or CRAB_* settings and
// listens on its own port (default 127.0.0.1:8788), so the current town on
// 8787 is unaffected whether this runs or not.
//
//	town-next export -owner <npub|hex> [-holder id=<npub|hex>]... > town.json
//	    write the current hard-coded town (internal/world NewDefault) as data
//	town-next [serve]
//	    TOWN_NEXT_DATA=town.json (required), TOWN_NEXT_ADDR (127.0.0.1:8788),
//	    TOWN_NEXT_BASE_URL (http://<addr>; the NIP-98 "u" prefix clients sign),
//	    TOWN_NEXT_TICK (250ms), TOWN_NEXT_AUTH_WINDOW (60s)
//	    serves the town-next viewer (internal/nextweb) at / -- not the current town's web/
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kojira/crab-town/internal/next"
	"github.com/kojira/crab-town/internal/nextweb"
	"github.com/kojira/crab-town/internal/nostr"
	"github.com/kojira/crab-town/internal/world"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type holders map[string]string

func (h holders) String() string { return fmt.Sprint(map[string]string(h)) }
func (h holders) Set(s string) error {
	id, pk, ok := strings.Cut(s, "=")
	if !ok {
		return errors.New("want id=<npub|hex>")
	}
	hex, err := nostr.ParsePubKey(pk)
	if err != nil {
		return err
	}
	h[id] = hex
	return nil
}

func export(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	owner := fs.String("owner", "", "town owner (npub or hex)")
	hs := holders{}
	fs.Var(hs, "holder", "current owner id = new holder pubkey (repeatable); unmapped houses become vacant plots")
	fs.Parse(args)
	pk, err := nostr.ParsePubKey(*owner)
	if err != nil {
		log.Fatalf("-owner: %v", err)
	}
	d := next.FromWorld(world.NewDefault(), pk, next.Spawn{Room: world.TownID, Pos: world.Pos{X: world.GardenX + 2, Y: 5}}, hs)
	if err := d.Validate(); err != nil {
		log.Fatal(err)
	}
	tmp, err := os.CreateTemp("", "town-next-export-*.json")
	if err != nil {
		log.Fatal(err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if err := d.Save(tmp.Name()); err != nil {
		log.Fatal(err)
	}
	b, _ := os.ReadFile(tmp.Name())
	os.Stdout.Write(b)
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "export" {
		export(os.Args[2:])
		return
	}
	path := os.Getenv("TOWN_NEXT_DATA")
	if path == "" {
		log.Fatal("TOWN_NEXT_DATA: path to the town data file is required (see `town-next export`)")
	}
	d, err := next.Load(path)
	if err != nil {
		log.Fatal(err)
	}
	t, err := next.New(d, path)
	if err != nil {
		log.Fatal(err)
	}
	addr := env("TOWN_NEXT_ADDR", "127.0.0.1:8788")
	tick, err := time.ParseDuration(env("TOWN_NEXT_TICK", "250ms"))
	if err != nil || tick <= 0 {
		log.Fatal("invalid TOWN_NEXT_TICK")
	}
	win, err := time.ParseDuration(env("TOWN_NEXT_AUTH_WINDOW", "60s"))
	if err != nil || win <= 0 {
		log.Fatal("invalid TOWN_NEXT_AUTH_WINDOW")
	}
	base := env("TOWN_NEXT_BASE_URL", "http://"+addr)
	srv := &next.Server{Town: t, Auth: &next.Verifier{BaseURL: base, Window: win}, Static: nextweb.FS()}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		tk := time.NewTicker(tick)
		defer tk.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
				t.World.Step()
			}
		}
	}()
	hs := &http.Server{Addr: addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		hs.Shutdown(sctx)
	}()
	log.Printf("town-next listening on %s (data %s, NIP-98 base %s)", addr, path, base)
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
