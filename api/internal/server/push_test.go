package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/modules/notify"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

type fakeExpo struct {
	mu   sync.Mutex
	sent []notify.PushMessage
	dead map[string]bool
	srv  *httptest.Server
}

func newFakeExpo() *fakeExpo {
	f := &fakeExpo{dead: map[string]bool{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msgs []notify.PushMessage
		_ = json.NewDecoder(r.Body).Decode(&msgs)
		f.mu.Lock()
		f.sent = append(f.sent, msgs...)
		type result struct {
			Status  string         `json:"status"`
			Details map[string]any `json:"details,omitempty"`
		}
		out := struct {
			Data []result `json:"data"`
		}{}
		for _, m := range msgs {
			if f.dead[m.To] {
				out.Data = append(out.Data, result{"error", map[string]any{"error": "DeviceNotRegistered"}})
			} else {
				out.Data = append(out.Data, result{Status: "ok"})
			}
		}
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(out)
	}))
	return f
}

func (f *fakeExpo) got(token string) []notify.PushMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []notify.PushMessage
	for _, m := range f.sent {
		if m.To == token {
			out = append(out, m)
		}
	}
	return out
}

func (e *env) register(tok, device string) {
	e.t.Helper()
	body, _ := json.Marshal(map[string]string{"token": device, "platform": "expo"})
	req, _ := http.NewRequest("PUT", e.srv.URL+"/v1/devices", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 204 {
		e.t.Fatalf("register %s: %v %v", device, err, resp)
	}
	resp.Body.Close()
}

func TestPushReachesTheRightDevicesOnly(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	staffTok := token("push-agent-"+sfx, "", "support")
	mgrTok := token("push-mgr-"+sfx, "", "manager")
	engTok := token("push-eng-"+sfx, "", "engineer")
	salesTok := token("push-sales-"+sfx, "", "sales")
	ta := e.newClient(staffTok, "PushA "+sfx)
	tb := e.newClient(staffTok, "PushB "+sfx)
	alice := token("push-alice-"+sfx, ta, "client_user")
	bob := token("push-bob-"+sfx, tb, "client_user")

	dev := func(n string) string { return fmt.Sprintf("ExponentPushToken[%s-%s]", n, sfx) }
	e.register(staffTok, dev("agent"))
	e.register(mgrTok, dev("mgr"))
	e.register(engTok, dev("eng"))
	e.register(salesTok, dev("sales"))
	e.register(alice, dev("alice"))
	e.register(bob, dev("bob"))

	fx := newFakeExpo()
	defer fx.srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := platform.NewPool(ctx, os.Getenv("HEKATE_TEST_DB"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	notifier := notify.NewNotifier(pool, &notify.ExpoSender{URL: fx.srv.URL, Client: fx.srv.Client()})
	relay := func() {
		for i := 0; i < 50; i++ {
			if err := platform.RelayOnce(ctx, pool, notifier); err != nil {
				t.Fatal(err)
			}
			var left int
			_ = pool.QueryRow(ctx, `SELECT count(*) FROM platform.outbox WHERE published_at IS NULL`).Scan(&left)
			if left == 0 {
				return
			}
		}
	}

	// Alice opens a P1; support, manager and engineer devices are paged, clients and sales are not.
	_, tk, _ := e.do("POST", "/v1/tickets", alice, map[string]any{"subject": "Core router down " + sfx, "severity": "P1", "channel": "app"})
	id := tk["id"].(string)
	relay()
	for _, who := range []string{"agent", "mgr", "eng"} {
		if n := len(fx.got(dev(who))); n != 1 || fx.got(dev(who))[0].Title != "P1 incident" {
			t.Errorf("%s should be paged once for the P1, got %v", who, fx.got(dev(who)))
		}
	}
	for _, who := range []string{"alice", "bob", "sales"} {
		if n := len(fx.got(dev(who))); n != 0 {
			t.Errorf("%s must not get the P1 page, got %d", who, n)
		}
	}

	// Staff resolve the ticket: Alice is told, Bob (another tenant) and staff are not.
	e.do("PATCH", "/v1/tickets/"+id, staffTok, map[string]any{"status": "in_progress"})
	e.do("PATCH", "/v1/tickets/"+id, staffTok, map[string]any{"status": "resolved"})
	relay()
	if n := len(fx.got(dev("alice"))); n != 2 {
		t.Errorf("alice should get 2 status updates, got %d", n)
	}
	if got := fx.got(dev("alice")); len(got) > 0 && got[len(got)-1].Body != "Core router down "+sfx+" is now resolved" {
		t.Errorf("message wrong: %q", got[len(got)-1].Body)
	}
	if n := len(fx.got(dev("bob"))); n != 0 {
		t.Errorf("another tenant must receive nothing, got %d", n)
	}

	// A staff reply reaches the client but not the author; a client reply reaches support but not the client.
	e.do("POST", "/v1/tickets/"+id+"/comments", staffTok, map[string]any{"body": "Engineer is on the way"})
	e.do("POST", "/v1/tickets/"+id+"/comments", staffTok, map[string]any{"body": "internal only", "internal": true})
	relay()
	if n := len(fx.got(dev("alice"))); n != 3 {
		t.Errorf("alice gets the status updates plus the reply (not the internal note): want 3, got %d", n)
	}
	before := len(fx.got(dev("agent")))
	e.do("POST", "/v1/tickets/"+id+"/comments", alice, map[string]any{"body": "Thank you"})
	relay()
	if n := len(fx.got(dev("agent"))) - before; n != 1 {
		t.Errorf("support should hear about the client reply once, got %d", n)
	}
	if n := len(fx.got(dev("alice"))); n != 3 {
		t.Errorf("a client must not be notified of their own reply, got %d", n)
	}
	if n := len(fx.got(dev("sales"))); n != 0 {
		t.Errorf("sales are not part of ticket traffic, got %d", n)
	}

	// An uninstalled app is forgotten after the push service says so.
	fx.mu.Lock()
	fx.dead[dev("alice")] = true
	fx.mu.Unlock()
	e.do("PATCH", "/v1/tickets/"+id, staffTok, map[string]any{"status": "closed"})
	relay()
	owner, _ := pgx.Connect(ctx, os.Getenv("HEKATE_TEST_DB"))
	defer owner.Close(ctx)
	var n int
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM notify.devices WHERE token=$1`, dev("alice")).Scan(&n)
	if n != 0 {
		t.Error("a DeviceNotRegistered token must be removed")
	}
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM notify.devices WHERE token=$1`, dev("bob")).Scan(&n)
	if n != 1 {
		t.Error("other devices must be kept")
	}
}
