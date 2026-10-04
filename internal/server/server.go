// Package server exposes the world over HTTP / WebSocket and serves the static viewer.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/kojira/crab-town/internal/world"
)

type Server struct {
	World      *world.World
	Tokens     Tokens // actor id -> token; who a request acts as
	WebhookURL string // empty = no webhook
	Static     fs.FS
	Client     *http.Client
}

func New(w *world.World, tokens Tokens, webhookURL string, static fs.FS) *Server {
	s := &Server{World: w, Tokens: tokens, WebhookURL: webhookURL, Static: static, Client: &http.Client{Timeout: 5 * time.Second}}
	if webhookURL != "" {
		w.SetHook(s.forward)
	}
	return s
}

// forward posts furniture interact / knock events to the webhook (async, best effort).
func (s *Server) forward(ev world.Event) {
	if s.WebhookURL == "" || (ev.Type != "interact" && ev.Type != "knock") {
		return
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return
	}
	go func() {
		resp, err := s.Client.Post(s.WebhookURL, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("webhook: %v", err)
			return
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			log.Printf("webhook: status %d", resp.StatusCode)
		}
	}()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	if s.Static != nil {
		mux.Handle("GET /", http.FileServerFS(s.Static))
	}
	mux.HandleFunc("GET /world", s.handleWorld)
	mux.HandleFunc("POST /actor/move", s.handleMove)
	mux.HandleFunc("POST /actor/interact", s.handleInteract)
	mux.HandleFunc("POST /actor/knock", s.handleKnock)
	return mux
}

type moveReq struct {
	Actor string `json:"actor"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
}

type interactReq struct {
	Actor     string `json:"actor"`
	Furniture string `json:"furniture"`
}

type knockReq struct {
	Room    string `json:"room"`
	Message string `json:"message"`
}

// caller returns the authenticated actor id of a mutating request. A missing or
// unknown token is answered with 401 and ok=false.
func (s *Server) caller(w http.ResponseWriter, r *http.Request) (string, bool) {
	who, err := s.Tokens.Who(requestToken(r))
	if err != nil || who == "" {
		writeErr(w, http.StatusUnauthorized, "valid token required")
		return "", false
	}
	return who, true
}

func (s *Server) handleMove(w http.ResponseWriter, r *http.Request) {
	by, ok := s.caller(w, r)
	var req moveReq
	if !ok || !decode(w, r, &req) {
		return
	}
	err := s.World.Move(by, req.Actor, world.Pos{X: req.X, Y: req.Y})
	reply(w, err)
}

func (s *Server) handleInteract(w http.ResponseWriter, r *http.Request) {
	by, ok := s.caller(w, r)
	var req interactReq
	if !ok || !decode(w, r, &req) {
		return
	}
	err := s.World.Interact(by, req.Actor, req.Furniture)
	reply(w, err)
}

func (s *Server) handleKnock(w http.ResponseWriter, r *http.Request) {
	by, ok := s.caller(w, r)
	var req knockReq
	if !ok || !decode(w, r, &req) {
		return
	}
	err := s.World.Knock(by, req.Room, req.Message)
	reply(w, err)
}

// handleWorld: read-only WebSocket. The viewer is whoever owns the token
// (?token= or Authorization: Bearer); no token = public, a bad token = 401.
// Sends a snapshot, then events, both filtered by the viewer's zone visibility.
// Client messages are ignored.
func (s *Server) handleWorld(w http.ResponseWriter, r *http.Request) {
	viewer, err := s.Tokens.Who(requestToken(r))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()
	// read-only: anything the viewer sends is read and discarded; it never touches the world.
	ctx, cancelCtx := context.WithCancel(r.Context())
	defer cancelCtx()
	go func() {
		defer cancelCtx()
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
		}
	}()

	events, cancel := s.World.Subscribe()
	defer cancel()

	if err := writeJSON(ctx, c, s.World.ViewSnapshot(viewer)); err != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			ev, ok = s.World.FilterEvent(viewer, ev)
			if !ok {
				continue
			}
			if err := writeJSON(ctx, c, ev); err != nil {
				return
			}
		}
	}
}

func writeJSON(ctx context.Context, c *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.Write(ctx, websocket.MessageText, b)
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return false
	}
	return true
}

func reply(w http.ResponseWriter, err error) {
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}` + "\n"))
		return
	}
	writeErr(w, statusOf(err), err.Error())
}

func statusOf(err error) int {
	switch {
	case errors.Is(err, world.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, world.ErrNoActor), errors.Is(err, world.ErrNoRoom), errors.Is(err, world.ErrNoFurniture):
		return http.StatusNotFound
	case errors.Is(err, world.ErrOutOfBounds), errors.Is(err, world.ErrBlocked),
		errors.Is(err, world.ErrUnreachable), errors.Is(err, world.ErrBadRequest),
		errors.Is(err, world.ErrNotUsable):
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": msg})
}
