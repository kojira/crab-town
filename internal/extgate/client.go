package extgate

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Handler receives core→gateway traffic. Calls are serialised per connection.
type Handler interface {
	// OnSay shows an utterance. nil = accepted (ok); an error = external_rejected.
	OnSay(bindingID, address, text string) error
	// OnActivity reports a turn starting / ending ("started" / "ended").
	OnActivity(bindingID, address, activityID, state string)
	// OnDisconnect is called when a live connection closes (activities end with it).
	OnDisconnect()
	// Operations is what crab-town declares in hello (and so what core shows
	// the agent as tools). invoke of anything else is rejected.
	Operations() []Operation
}

// Timings. Variables so tests can shorten them.
var (
	BackoffMin  = 200 * time.Millisecond
	BackoffMax  = 8 * time.Second
	HelloWait   = 10 * time.Second
	SaidTimeout = 10 * time.Second
)

// ErrNotConnected: no live, hello'd connection or no acknowledged binding.
var ErrNotConnected = errors.New("not_connected")

// SaidError is a core err response to said.
type SaidError struct{ Code, Detail string }

func (e *SaidError) Error() string { return "said: " + e.Code }

type response struct {
	ok     bool
	hasSeq bool
	seq    *int64
	code   string
	detail string
}

// Client is one gateway instance. Run keeps it connected.
type Client struct {
	cfg     Config
	handler Handler
	dial    func(ctx context.Context) (net.Conn, error)

	mu       sync.Mutex
	conn     *conn // live (hello acknowledged) connection, or nil
	reqSeq   atomic.Uint64
	Connects atomic.Int64 // successful hellos (observability / tests)
}

// New returns a client for cfg. It does not connect until Run.
func New(cfg Config, h Handler) *Client {
	c := &Client{cfg: cfg, handler: h}
	c.dial = func(ctx context.Context) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "unix", cfg.Socket)
	}
	return c
}

type conn struct {
	nc       net.Conn
	wmu      sync.Mutex
	mu       sync.Mutex
	pending  map[string]chan response
	bindings map[string]string // binding_id -> address (acknowledged)
	closed   chan struct{}
	once     sync.Once
}

func (k *conn) write(v any) error {
	b, err := encodeFrame(v)
	if err != nil {
		return err
	}
	k.wmu.Lock()
	defer k.wmu.Unlock()
	_, err = k.nc.Write(b)
	return err
}

func (k *conn) close(reason string) {
	k.once.Do(func() {
		log.Printf("extgate: close: %s", reason)
		k.nc.Close()
		close(k.closed)
	})
}

// Run connects, says hello, and serves until ctx ends, reconnecting with
// exponential backoff (BackoffMin doubling up to BackoffMax; reset after a
// successful hello).
func (c *Client) Run(ctx context.Context) {
	backoff := BackoffMin
	for ctx.Err() == nil {
		if c.session(ctx) {
			backoff = BackoffMin
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, BackoffMax)
	}
}

// session runs one connection. It reports whether hello was accepted.
func (c *Client) session(ctx context.Context) bool {
	nc, err := c.dial(ctx)
	if err != nil {
		log.Printf("extgate: connect: %v", err)
		return false
	}
	k := &conn{nc: nc, pending: map[string]chan response{}, bindings: map[string]string{}, closed: make(chan struct{})}
	stop := context.AfterFunc(ctx, func() { k.close("shutdown") })
	defer stop()
	defer k.close("session end")

	helloID := "hello:" + c.cfg.InstanceID
	helloCh := make(chan response, 1)
	k.pending[helloID] = helloCh
	if err := k.write(map[string]any{
		"id": helloID, "m": "hello",
		"protocol": Protocol, "operation_protocol": OperationProtocol,
		"final_delivery": "automatic", "operations": Declarations(c.handler.Operations()),
		"instance_id": c.cfg.InstanceID, "revision": c.cfg.Revision,
		"config_digest": c.cfg.ConfigDigest,
	}); err != nil {
		log.Printf("extgate: hello write: %v", err)
		return false
	}
	r := bufio.NewReaderSize(nc, 64*1024)
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.readLoop(k, r)
	}()

	select {
	case resp := <-helloCh:
		if !resp.ok || resp.hasSeq {
			log.Printf("extgate: hello rejected: %s %s", resp.code, resp.detail)
			k.close("hello rejected")
			<-done
			return false
		}
	case <-k.closed:
		<-done
		return false
	case <-time.After(HelloWait):
		k.close("hello timeout")
		<-done
		return false
	}
	log.Printf("extgate: hello ok instance=%s", c.cfg.InstanceID)
	c.Connects.Add(1)
	c.mu.Lock()
	c.conn = k
	c.mu.Unlock()
	<-done
	c.mu.Lock()
	if c.conn == k {
		c.conn = nil
	}
	c.mu.Unlock()
	c.handler.OnDisconnect()
	return true
}

func (c *Client) readLoop(k *conn, r *bufio.Reader) {
	defer func() {
		k.mu.Lock()
		for id, ch := range k.pending {
			close(ch) // closed channel = disconnected
			delete(k.pending, id)
		}
		k.mu.Unlock()
	}()
	for {
		b, err := readFrame(r)
		if err != nil {
			k.close("read: " + err.Error())
			return
		}
		obj, err := decodeObject(b)
		if err != nil {
			k.close("frame: " + err.Error())
			return
		}
		if fatal := c.handle(k, obj); fatal != "" {
			k.close(fatal)
			return
		}
	}
}

func errFrame(id, code string) map[string]any {
	return map[string]any{"id": id, "m": "err", "code": code, "detail": nil}
}

// handle dispatches one message. A non-empty return closes the connection.
func (c *Client) handle(k *conn, obj map[string]any) string {
	m, _ := str(obj, "m")
	id, hasID := reqID(obj)
	switch m {
	case "say": // final delivery of the agent's turn text (protocol, not a declared operation)
		return c.handleSay(k, obj, id, hasID)
	case "invoke":
		return c.handleInvoke(k, obj, id, hasID)
	case "bind":
		bid, _ := str(obj, "binding_id")
		addr, okAddr := nonempty(obj, "address")
		if !hasID || !validUUID(bid) || !okAddr {
			return c.reply(k, hasID, errFrame(id, "bad_request"))
		}
		k.mu.Lock()
		for b, a := range k.bindings {
			if a == addr && b != bid {
				k.mu.Unlock()
				return "binding_conflict"
			}
		}
		k.bindings[bid] = addr
		k.mu.Unlock()
		return c.reply(k, true, map[string]any{"id": id, "m": "ok"})
	case "activity":
		bid, _ := str(obj, "binding_id")
		aid, _ := str(obj, "activity_id")
		state, _ := str(obj, "state")
		if !validUUID(bid) || !validUUID(aid) || (state != "started" && state != "ended") {
			return c.reply(k, hasID, errFrame(id, "bad_request"))
		}
		k.mu.Lock()
		addr, bound := k.bindings[bid]
		k.mu.Unlock()
		if bound {
			c.handler.OnActivity(bid, addr, aid, state)
		}
		return ""
	case "ok", "err":
		return c.resolve(k, obj, m)
	default: // hello/said from core, unknown m (turn_failed, ...)
		return c.reply(k, hasID, errFrame(id, "unknown_message"))
	}
}

func (c *Client) handleSay(k *conn, obj map[string]any, id string, hasID bool) string {
	bid, _ := str(obj, "binding_id")
	payload, okP := obj["payload"].(map[string]any)
	if !hasID || !validUUID(bid) || !okP {
		return c.reply(k, hasID, errFrame(id, "bad_request"))
	}
	text, okT := nonempty(payload, "text")
	k.mu.Lock()
	addr, bound := k.bindings[bid]
	k.mu.Unlock()
	if !okT || !bound {
		return c.reply(k, true, errFrame(id, "external_rejected"))
	}
	if err := c.handler.OnSay(bid, addr, text); err != nil {
		return c.reply(k, true, errFrame(id, "external_rejected"))
	}
	return c.reply(k, true, map[string]any{"id": id, "m": "ok"})
}

func (c *Client) reply(k *conn, send bool, v map[string]any) string {
	if !send {
		return ""
	}
	if err := k.write(v); err != nil {
		return "write: " + err.Error()
	}
	return ""
}

func (c *Client) resolve(k *conn, obj map[string]any, m string) string {
	id, ok := reqID(obj)
	if !ok {
		return "response_invalid"
	}
	var resp response
	if m == "ok" {
		resp.ok = true
		if raw, present := obj["seq"]; present {
			resp.hasSeq = true
			switch v := raw.(type) {
			case nil:
			case json.Number:
				n, err := strconv.ParseInt(v.String(), 10, 64)
				if err != nil || n <= 0 {
					return "response_invalid"
				}
				resp.seq = &n
			default:
				return "response_invalid"
			}
		}
	} else {
		code, okC := nonempty(obj, "code")
		detail, present := obj["detail"]
		if !okC || !present {
			return "response_invalid"
		}
		if detail != nil {
			s, isStr := detail.(string)
			if !isStr {
				return "response_invalid"
			}
			resp.detail = s
		}
		resp.code = code
	}
	k.mu.Lock()
	ch, found := k.pending[id]
	delete(k.pending, id)
	k.mu.Unlock()
	if !found {
		return "response_invalid"
	}
	ch <- resp
	return ""
}

// Bindings returns the acknowledged binding addresses (sorted).
func (c *Client) Bindings() []string {
	k := c.live()
	if k == nil {
		return nil
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []string
	for _, a := range k.bindings {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

func (c *Client) live() *conn {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn
}

// Said is one inbound utterance for core.
type Said struct {
	Origin      string // unique per event; core dedups (binding, origin)
	Text        string
	AuthorLabel string // display only; may be empty
}

// Said sends said to the configured (or first acknowledged) binding and waits
// for core's answer. seq == nil with err == nil means core did not admit it.
func (c *Client) Said(ctx context.Context, s Said) (*int64, error) {
	k := c.live()
	if k == nil {
		return nil, ErrNotConnected
	}
	k.mu.Lock()
	bid := ""
	addrs := make([]string, 0, len(k.bindings))
	for b, a := range k.bindings {
		if c.cfg.Address == "" || a == c.cfg.Address {
			addrs = append(addrs, a+"\x00"+b)
		}
	}
	if len(addrs) > 0 {
		sort.Strings(addrs)
		bid = addrs[0][len(addrs[0])-36:]
	}
	id := fmt.Sprintf("said:%d", c.reqSeq.Add(1))
	ch := make(chan response, 1)
	if bid != "" {
		k.pending[id] = ch
	}
	k.mu.Unlock()
	if bid == "" {
		return nil, ErrNotConnected
	}
	frame := map[string]any{
		"id": id, "m": "said", "binding_id": bid, "origin": s.Origin,
		"author_id": c.cfg.AuthorID, "text": s.Text, "attachments": []any{},
	}
	if s.AuthorLabel != "" {
		frame["author_label"] = s.AuthorLabel
	}
	if err := k.write(frame); err != nil {
		k.close("said write")
		return nil, ErrNotConnected
	}
	select {
	case resp, open := <-ch:
		switch {
		case !open:
			return nil, ErrNotConnected
		case !resp.ok:
			return nil, &SaidError{Code: resp.code, Detail: resp.detail}
		case !resp.hasSeq:
			k.close("response_invalid: said ok without seq")
			return nil, ErrNotConnected
		}
		return resp.seq, nil
	case <-time.After(SaidTimeout):
		k.close("said timeout")
		return nil, ErrNotConnected
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// handleInvoke runs one declared operation the agent chose. Every refusal --
// unknown operation, unbound binding, bad payload, the world saying no -- is
// operation_rejected with the reason in detail: core treats any other code on
// an invoke response as a protocol violation and closes the connection.
func (c *Client) handleInvoke(k *conn, obj map[string]any, id string, hasID bool) string {
	if !hasID {
		return "" // nothing to answer to; core always sends an id
	}
	reject := func(detail string) string {
		f := errFrame(id, "operation_rejected")
		f["detail"] = detail
		return c.reply(k, true, f)
	}
	bid, _ := str(obj, "binding_id")
	k.mu.Lock()
	_, bound := k.bindings[bid]
	k.mu.Unlock()
	if !bound {
		return reject("binding が確認されていない")
	}
	name, _ := str(obj, "operation")
	var op *Operation
	for _, o := range c.handler.Operations() {
		if o.Name == name {
			op = &o
			break
		}
	}
	if op == nil {
		return reject("crab-town に " + strconv.Quote(name) + " という操作はない")
	}
	payload, ok := obj["payload"].(map[string]any)
	if !ok {
		return reject("payload はオブジェクトで渡す")
	}
	result, err := op.Run(payload)
	if err != nil {
		return reject(strings.TrimPrefix(err.Error(), ErrRejected.Error()+": "))
	}
	return c.reply(k, true, map[string]any{"id": id, "m": "ok", "result": result})
}
