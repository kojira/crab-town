// Command crab-town runs the 2D space gateway MVP.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	crabtown "github.com/kojira/crab-town"
	"github.com/kojira/crab-town/internal/nostr"
	"github.com/kojira/crab-town/internal/server"
	"github.com/kojira/crab-town/internal/world"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	// "crab-town nostr-pubkey": create the town key file if needed and print
	// only its public key (for web/config.js). The secret is never printed.
	if len(os.Args) == 2 && os.Args[1] == "nostr-pubkey" {
		key, _, err := nostr.LoadOrCreateKey(os.Getenv("CRAB_NOSTR_KEY_FILE"))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(nostr.PubHex(key))
		return
	}
	addr := env("CRAB_ADDR", "127.0.0.1:8787")
	webhook := os.Getenv("CRAB_WEBHOOK_URL")
	tick, err := time.ParseDuration(env("CRAB_TICK", "250ms"))
	if err != nil || tick <= 0 {
		log.Fatalf("invalid CRAB_TICK")
	}

	tokens, err := server.LoadTokens(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}

	w := world.NewDefault()
	invited, noToken, err := server.ApplyInvited(w, tokens, os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	if len(noToken) > 0 {
		log.Printf("warning: invited ids without a token (cannot view as invited): %v", noToken)
	}
	srv := server.New(w, tokens, webhook, crabtown.WebFS())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	nostrCfg, err := nostr.LoadConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	if nostrCfg != nil {
		bridge, town, err := nostr.NewBridge(w, *nostrCfg)
		if err != nil {
			log.Fatal(err)
		}
		go bridge.Run(ctx)
		log.Printf("nostr: enabled (town pubkey %s, owner set: %v, relays %v)", town, nostrCfg.Owner != "", nostrCfg.Relays)
	} else {
		log.Printf("nostr: disabled (no CRAB_NOSTR_KEY_FILE)")
	}

	go func() {
		t := time.NewTicker(tick)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				w.Step()
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
	log.Printf("crab-town listening on %s (webhook: %v, tokens: %d, invited: %v)", addr, webhook != "", len(tokens), invited)
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
