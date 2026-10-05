package nostr

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// relayConn keeps one relay connected: it subscribes with filter, passes every
// received EVENT to onEvent, and lets the bridge publish through it.
type relayConn struct {
	url     string
	filter  func() map[string]any
	onEvent func(*Event)

	mu   sync.Mutex
	conn *websocket.Conn
}

func (rc *relayConn) run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		start := time.Now()
		err := rc.session(ctx)
		if ctx.Err() != nil {
			return
		}
		if time.Since(start) > time.Minute {
			backoff = time.Second
		}
		log.Printf("nostr: relay %s: %v (reconnect in %s)", rc.url, err, backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < time.Minute {
			backoff *= 2
		}
	}
}

func (rc *relayConn) session(ctx context.Context) error {
	dctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	c, _, err := websocket.Dial(dctx, rc.url, nil)
	cancel()
	if err != nil {
		return err
	}
	c.SetReadLimit(1 << 20)
	defer c.CloseNow()
	rc.mu.Lock()
	rc.conn = c
	rc.mu.Unlock()
	defer func() {
		rc.mu.Lock()
		rc.conn = nil
		rc.mu.Unlock()
	}()
	if err := rc.send(ctx, []any{"REQ", "crab-town", rc.filter()}); err != nil {
		return err
	}
	log.Printf("nostr: relay %s: subscribed", rc.url)
	for {
		_, b, err := c.Read(ctx)
		if err != nil {
			return err
		}
		var msg []json.RawMessage
		if json.Unmarshal(b, &msg) != nil || len(msg) < 2 {
			continue
		}
		var typ string
		json.Unmarshal(msg[0], &typ)
		switch typ {
		case "EVENT":
			if len(msg) < 3 {
				continue
			}
			var ev Event
			if json.Unmarshal(msg[2], &ev) == nil {
				rc.onEvent(&ev)
			}
		case "OK":
			if len(msg) >= 4 {
				var ok bool
				var why string
				json.Unmarshal(msg[2], &ok)
				json.Unmarshal(msg[3], &why)
				if !ok {
					log.Printf("nostr: relay %s rejected an event: %s", rc.url, why)
				}
			}
		case "NOTICE", "CLOSED":
			log.Printf("nostr: relay %s: %s", rc.url, b)
		}
	}
}

// send writes one message; it is a no-op error when disconnected.
func (rc *relayConn) send(ctx context.Context, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.conn == nil {
		return errNotConnected
	}
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return rc.conn.Write(wctx, websocket.MessageText, b)
}

type connErr string

func (e connErr) Error() string { return string(e) }

const errNotConnected = connErr("not connected")
