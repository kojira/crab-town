package extgate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kojira/crab-town/internal/world"
)

const (
	testInstance = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	testBinding  = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	testDigest   = "3b354e0c676677f4ab5735bc9f8eb2fbb2c74df34e5d54d3b4fd88165b542672"
	testActivity = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
)

func init() {
	BackoffMin = 10 * time.Millisecond
	BackoffMax = 40 * time.Millisecond
	HelloWait = 2 * time.Second
	SaidTimeout = 2 * time.Second
}

// fakeCore listens on a UDS like opencrab core and hands out accepted conns.
type fakeCore struct {
	t     *testing.T
	ln    net.Listener
	path  string
	conns chan net.Conn
}

func newFakeCore(t *testing.T) *fakeCore {
	t.Helper()
	// short dir: UDS paths are limited to ~104 bytes on macOS
	dir, err := os.MkdirTemp("", "eg")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "core.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakeCore{t: t, ln: ln, path: path, conns: make(chan net.Conn, 8)}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			fc.conns <- c
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return fc
}

func (fc *fakeCore) config() Config {
	return Config{Socket: fc.path, InstanceID: testInstance, Revision: 1,
		ConfigDigest: testDigest, AuthorID: "crab-town", Actor: "nostarou"}
}

type coreConn struct {
	t  *testing.T
	c  net.Conn
	rd *bufio.Reader
}

func (fc *fakeCore) accept() *coreConn {
	fc.t.Helper()
	select {
	case c := <-fc.conns:
		fc.t.Cleanup(func() { c.Close() })
		return &coreConn{t: fc.t, c: c, rd: bufio.NewReaderSize(c, 2*MaxFrame)}
	case <-time.After(3 * time.Second):
		fc.t.Fatal("no connection from gateway")
		return nil
	}
}

func (cc *coreConn) recv() map[string]any {
	cc.t.Helper()
	cc.c.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := cc.rd.ReadBytes('\n')
	if err != nil {
		cc.t.Fatalf("core recv: %v", err)
	}
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		cc.t.Fatalf("core recv decode: %v (%q)", err, line)
	}
	return m
}

// expectClosed asserts the gateway closed the connection (EOF) without
// sending anything else.
func (cc *coreConn) expectClosed() {
	cc.t.Helper()
	cc.c.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := cc.rd.ReadBytes('\n')
	if err == nil {
		cc.t.Fatalf("expected close, got frame %q", line)
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		cc.t.Fatalf("expected close, connection still open")
	}
}

// expectQuiet asserts nothing arrives for d.
func (cc *coreConn) expectQuiet(d time.Duration) {
	cc.t.Helper()
	cc.c.SetReadDeadline(time.Now().Add(d))
	line, err := cc.rd.ReadBytes('\n')
	if err == nil {
		cc.t.Fatalf("expected no frame, got %q", line)
	}
	if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
		cc.t.Fatalf("expected open connection, got %v", err)
	}
}

func (cc *coreConn) send(v any) {
	cc.t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		cc.t.Fatal(err)
	}
	cc.sendRaw(append(b, '\n'))
}

func (cc *coreConn) sendRaw(b []byte) {
	cc.t.Helper()
	if _, err := cc.c.Write(b); err != nil {
		cc.t.Fatalf("core send: %v", err)
	}
}

// helloBind does the core side of hello → ok → bind → ok.
func (cc *coreConn) helloBind() {
	cc.t.Helper()
	h := cc.recv()
	want := map[string]any{
		"m": "hello", "protocol": json.Number("3"), "operation_protocol": json.Number("1"),
		"final_delivery": "automatic", "instance_id": testInstance,
		"revision": json.Number("1"), "config_digest": testDigest,
	}
	for k, v := range want {
		if h[k] != v {
			cc.t.Fatalf("hello[%s] = %#v, want %#v (hello=%v)", k, h[k], v, h)
		}
	}
	if ops, ok := h["operations"].([]any); !ok || len(ops) != 0 {
		cc.t.Fatalf("hello operations = %#v, want []", h["operations"])
	}
	cc.send(map[string]any{"id": h["id"], "m": "ok"})
	cc.send(map[string]any{"id": "bind:" + testBinding, "m": "bind", "binding_id": testBinding, "address": "crab-town-house"})
	ok := cc.recv()
	if ok["m"] != "ok" || ok["id"] != "bind:"+testBinding {
		cc.t.Fatalf("bind response = %v", ok)
	}
}

// startBridge runs a bridge on the default world against fc.
func startBridge(t *testing.T, fc *fakeCore) (*world.World, *Bridge) {
	t.Helper()
	w := world.NewDefault()
	b := NewBridge(w, fc.config())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go b.Run(ctx)
	return w, b
}

// waitFor polls cond, stepping the world each time.
func waitFor(t *testing.T, w *world.World, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		w.Step()
		time.Sleep(2 * time.Millisecond)
	}
}

// waitBound waits until the client has the binding acknowledged.
func waitBound(t *testing.T, b *Bridge) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for len(b.Client.Bindings()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("binding not acknowledged")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// waitConnects waits until exactly n hellos have been accepted.
func waitConnects(t *testing.T, b *Bridge, n int64) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for b.Client.Connects.Load() < n {
		if time.Now().After(deadline) {
			t.Fatalf("hellos = %d, want %d", b.Client.Connects.Load(), n)
		}
		time.Sleep(2 * time.Millisecond)
	}
	if got := b.Client.Connects.Load(); got != n {
		t.Fatalf("hellos = %d, want %d", got, n)
	}
}
