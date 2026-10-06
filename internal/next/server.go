package next

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/kojira/crab-town/internal/world"
)

// Server is the HTTP face of the next-generation town. Every POST is one
// operation by the pubkey that signed it (NIP-98). The same endpoints serve
// a browser (NIP-07) and an agent (its own key): nothing asks which it is.
type Server struct {
	Town   *Town
	Auth   *Verifier
	Static fs.FS // optional viewer files
}

// Request bodies. The caller is never named in the body: it is the signer.
type moveReq struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type interactReq struct {
	Furniture string `json:"furniture"`
}
type sayReq struct {
	Text string `json:"text"`
}
type knockReq struct {
	House   string `json:"house"`
	Message string `json:"message"`
}
type plotReq struct {
	House  string   `json:"house"`
	Pubkey string   `json:"pubkey,omitempty"`  // approve: the applicant
	Invite []string `json:"invited,omitempty"` // invite: the full list
}

// op is one operation: the verified caller and the raw body.
type op func(pubkey string, body []byte) error

func jsonOp[T any](fn func(pubkey string, req T) error) op {
	return func(pubkey string, body []byte) error {
		var req T
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return world.ErrBadRequest
			}
		}
		return fn(pubkey, req)
	}
}

func (s *Server) Handler() http.Handler {
	t := s.Town
	ops := map[string]op{
		"/join":  jsonOp(func(pk string, _ struct{}) error { return t.Join(pk) }),
		"/leave": jsonOp(func(pk string, _ struct{}) error { return t.Leave(pk) }),
		"/move":  jsonOp(func(pk string, r moveReq) error { return t.Move(pk, world.Pos{X: r.X, Y: r.Y}) }),
		"/interact": jsonOp(func(pk string, r interactReq) error {
			return t.Interact(pk, r.Furniture)
		}),
		"/say":           jsonOp(func(pk string, r sayReq) error { return t.Say(pk, r.Text) }),
		"/knock":         jsonOp(func(pk string, r knockReq) error { return t.Knock(pk, r.House, r.Message) }),
		"/plots/apply":   jsonOp(func(pk string, r plotReq) error { return t.Apply(pk, r.House) }),
		"/plots/approve": jsonOp(func(pk string, r plotReq) error { return t.Approve(pk, r.House, r.Pubkey) }),
		"/plots/release": jsonOp(func(pk string, r plotReq) error { return t.Release(pk, r.House) }),
		"/houses/invite": jsonOp(func(pk string, r plotReq) error { return t.SetInvited(pk, r.House, r.Invite) }),
	}
	mux := http.NewServeMux()
	if s.Static != nil {
		mux.Handle("GET /", http.FileServerFS(s.Static))
	}
	mux.HandleFunc("GET /world", s.handleWorld)
	for path, fn := range ops {
		mux.HandleFunc("POST "+path, s.wrap(fn))
	}
	return mux
}

// wrap reads the body, verifies the signature over it, and runs fn as the signer.
func (s *Server) wrap(fn op) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<16))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "body too large")
			return
		}
		pk, err := s.Auth.CheckRequest(r, body)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, err.Error())
			return
		}
		if err := fn(pk, body); err != nil {
			writeErr(w, statusOf(err), err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "you": pk, "rights": s.Town.RightsOf(pk)})
	}
}

// handleWorld: read-only WebSocket. No auth = anonymous viewer; ?auth= (a
// NIP-98 event for GET /world) = that pubkey's view; a bad one = 401.
func (s *Server) handleWorld(w http.ResponseWriter, r *http.Request) {
	viewer := ""
	if r.Header.Get("Authorization") != "" || r.URL.Query().Get("auth") != "" {
		pk, err := s.Auth.CheckRequest(r, nil)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, err.Error())
			return
		}
		viewer = pk
	}
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		defer cancel()
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
		}
	}()
	events, unsub := s.Town.World.Subscribe()
	defer unsub()
	v := s.Town.ViewFor(viewer)
	if err := writeJSON(ctx, c, v); err != nil {
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
			if ev, ok = s.Town.World.FilterEvent(viewer, ev); !ok {
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

func statusOf(err error) int {
	switch {
	case errors.Is(err, world.ErrForbidden), errors.Is(err, ErrNotOwner), errors.Is(err, ErrNotHolder):
		return http.StatusForbidden
	case errors.Is(err, world.ErrNoActor), errors.Is(err, world.ErrNoRoom),
		errors.Is(err, world.ErrNoFurniture), errors.Is(err, world.ErrNoHouse):
		return http.StatusNotFound
	case errors.Is(err, world.ErrInUse), errors.Is(err, world.ErrTileTaken), errors.Is(err, ErrNotVacant),
		errors.Is(err, ErrNoApplication), errors.Is(err, ErrFull), errors.Is(err, ErrNotJoined):
		return http.StatusConflict
	}
	return http.StatusBadRequest
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": msg})
}
