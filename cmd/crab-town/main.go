// Command crab-town runs the 2D space gateway MVP.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	crabtown "github.com/kojira/crab-town"
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
	srv := server.New(w, tokens, webhook, crabtown.WebFS())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	log.Printf("crab-town listening on %s (webhook: %v, tokens: %d)", addr, webhook != "", len(tokens))
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
