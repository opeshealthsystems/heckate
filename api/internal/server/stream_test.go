package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// setupStream builds a server with the live-event hub and a relay that feeds it.
func setupStream(t *testing.T) (*env, func(), string) {
	dsn := os.Getenv("HEKATE_TEST_DB")
	if dsn == "" {
		t.Skip("HEKATE_TEST_DB not set")
	}
	ctx := context.Background()
	pool, err := platform.NewPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	handler, hub := NewWithHub(platform.Config{DevSecret: secret, CORSOrigins: "http://localhost:3000"}, pool)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	relay := func() {
		for i := 0; i < 50; i++ {
			if err := platform.RelayOnce(ctx, pool, hub); err != nil {
				t.Fatal(err)
			}
			var left int
			_ = pool.QueryRow(ctx, `SELECT count(*) FROM platform.outbox WHERE published_at IS NULL`).Scan(&left)
			if left == 0 {
				return
			}
		}
	}
	return &env{t, srv}, relay, strings.Replace(srv.URL, "http://", "ws://", 1)
}

func dial(ctx context.Context, url, tok string) (*websocket.Conn, error) {
	c, _, err := websocket.Dial(ctx, url+"/v1/stream", &websocket.DialOptions{Subprotocols: []string{"hekate.v1", "bearer." + tok}})
	return c, err
}

// listen reads a connection in the background. A Read whose context times out closes the whole
// connection in this library, so tests must not poll the socket with short deadlines.
func listen(c *websocket.Conn) <-chan map[string]any {
	ch := make(chan map[string]any, 64)
	go func() {
		defer close(ch)
		for {
			_, b, err := c.Read(context.Background())
			if err != nil {
				return
			}
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			ch <- m
		}
	}()
	return ch
}

// next returns one event, or nil if none arrives within d (or the connection closed).
func next(ch <-chan map[string]any, d time.Duration) map[string]any {
	select {
	case m := <-ch:
		return m
	case <-time.After(d):
		return nil
	}
}

func TestLiveEventsAreFilteredByTenant(t *testing.T) {
	e, relay, wsURL := setupStream(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	staffTok := token("ws-agent-"+sfx, "", "support")
	ta := e.newClient(staffTok, "WS A "+sfx)
	tb := e.newClient(staffTok, "WS B "+sfx)
	alice := token("ws-alice-"+sfx, ta, "client_user")
	bob := token("ws-bob-"+sfx, tb, "client_user")
	supplier := token("ws-sup-"+sfx, ta, "supplier")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := dial(ctx, wsURL, ""); err == nil {
		t.Error("a connection without a token must be refused")
	}
	if _, err := dial(ctx, wsURL, "garbage"); err == nil {
		t.Error("a connection with a bad token must be refused")
	}
	conns := map[string]<-chan map[string]any{}
	for name, tok := range map[string]string{"staff": staffTok, "alice": alice, "bob": bob, "supplier": supplier} {
		c, err := dial(ctx, wsURL, tok)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		defer c.CloseNow()
		if c.Subprotocol() != "hekate.v1" {
			t.Errorf("%s: negotiated %q, want hekate.v1", name, c.Subprotocol())
		}
		conns[name] = listen(c)
	}
	relay() // flush older events so only this test's events follow
	for _, ch := range conns {
		for next(ch, 100*time.Millisecond) != nil {
		}
	}

	_, tk, _ := e.do("POST", "/v1/tickets", alice, map[string]any{"subject": "live " + sfx, "severity": "P3", "channel": "app"})
	id := tk["id"].(string)
	e.do("POST", "/v1/tickets/"+id+"/comments", staffTok, map[string]any{"body": "hello alice"})
	e.do("POST", "/v1/tickets/"+id+"/comments", staffTok, map[string]any{"body": "private", "internal": true})
	e.do("POST", "/v1/invoices", token("f", "", "finance"), map[string]any{"tenant_id": ta, "title": "not streamed", "lines": lines(1000)})
	relay()

	// Staff and the ticket's own client hear created + the public comment (and never the internal note).
	for _, who := range []string{"staff", "alice"} {
		var subjects []string
		for m := next(conns[who], 800*time.Millisecond); m != nil; m = next(conns[who], 300*time.Millisecond) {
			subjects = append(subjects, m["subject"].(string))
			if d, _ := m["data"].(map[string]any); d != nil && d["id"] != id {
				t.Errorf("%s got an event for another ticket: %v", who, d)
			}
		}
		if len(subjects) != 2 || subjects[0] != "ticket.created" || subjects[1] != "ticket.commented" {
			t.Errorf("%s should get ticket.created then ticket.commented, got %v", who, subjects)
		}
	}
	for _, who := range []string{"bob", "supplier"} {
		if m := next(conns[who], 500*time.Millisecond); m != nil {
			t.Errorf("%s must hear nothing about another tenant's ticket, got %v", who, m)
		}
	}
}

func TestStreamEndsWithTheToken(t *testing.T) {
	_, _, wsURL := setupStream(t)
	short := platform.SignDevToken(secret, map[string]any{"sub": "ws-short", "roles": []string{"support"}, "exp": time.Now().Add(2 * time.Second).Unix()})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c, err := dial(ctx, wsURL, short)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	start := time.Now()
	ch := listen(c)
	select {
	case _, open := <-ch:
		if open {
			t.Fatal("no event was expected")
		}
	case <-time.After(8 * time.Second):
		t.Fatal("the connection must close when the token expires")
	}
	if d := time.Since(start); d > 6*time.Second {
		t.Errorf("closed after %v, expected about 2s", d)
	}
}

func TestStreamLimitsConnectionsPerUser(t *testing.T) {
	_, _, wsURL := setupStream(t)
	tok := token("ws-many-"+fmt.Sprint(time.Now().UnixNano()), "", "support")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for i := 0; i < 5; i++ {
		c, err := dial(ctx, wsURL, tok)
		if err != nil {
			t.Fatalf("connection %d: %v", i+1, err)
		}
		defer c.CloseNow()
	}
	if c, err := dial(ctx, wsURL, tok); err == nil {
		c.CloseNow()
		t.Error("a sixth connection for the same user must be refused")
	}
}
