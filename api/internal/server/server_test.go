package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/migrations"
)

const secret = "test-secret"

type env struct {
	t   *testing.T
	srv *httptest.Server
}

func setup(t *testing.T) *env {
	dsn := os.Getenv("HEKATE_TEST_DB")
	if dsn == "" {
		t.Skip("HEKATE_TEST_DB not set")
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	if err := platform.Migrate(ctx, dsn, migrations.FS); err != nil {
		t.Fatal(err)
	}
	pool, err := platform.NewPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	srv := httptest.NewServer(New(platform.Config{DevSecret: secret}, pool))
	t.Cleanup(srv.Close)
	return &env{t, srv}
}

func token(sub string, tenant string, roles ...string) string {
	return platform.SignDevToken(secret, map[string]any{"sub": sub, "tenant_id": tenant, "roles": roles, "exp": time.Now().Add(time.Hour).Unix()})
}

var idem atomic.Int64

func jsonDecode(r io.Reader, v any) error { return json.NewDecoder(r).Decode(v) }

func (e *env) do(method, path, tok string, body any, hdr ...string) (int, map[string]any, http.Header) {
	e.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, rd)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	if method == "POST" {
		req.Header.Set("Idempotency-Key", fmt.Sprintf("%s-%d-%d", e.t.Name(), time.Now().UnixNano(), idem.Add(1)))
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out, resp.Header
}

func (e *env) newClient(staff, name string) string {
	code, c, _ := e.do("POST", "/v1/clients", staff, map[string]any{"legal_name": name})
	if code != 201 {
		e.t.Fatalf("create client: %d %v", code, c)
	}
	return c["tenant_id"].(string)
}

func TestAuth(t *testing.T) {
	e := setup(t)
	if code, _, _ := e.do("GET", "/v1/tickets", "", nil); code != 401 {
		t.Errorf("no token: want 401, got %d", code)
	}
	if code, _, _ := e.do("GET", "/v1/tickets", "garbage", nil); code != 401 {
		t.Errorf("bad token: want 401, got %d", code)
	}
	expired := platform.SignDevToken(secret, map[string]any{"sub": "x", "roles": []string{"support"}, "exp": time.Now().Add(-time.Hour).Unix()})
	if code, _, _ := e.do("GET", "/v1/tickets", expired, nil); code != 401 {
		t.Errorf("expired: want 401, got %d", code)
	}
	wrong := platform.SignDevToken("other-secret", map[string]any{"sub": "x", "roles": []string{"support"}, "exp": time.Now().Add(time.Hour).Unix()})
	if code, _, _ := e.do("GET", "/v1/tickets", wrong, nil); code != 401 {
		t.Errorf("wrong signature: want 401, got %d", code)
	}
	if code, _, _ := e.do("GET", "/v1/leads", token("c", "", "client_user"), nil); code != 403 {
		t.Errorf("client on leads: want 403, got %d", code)
	}
}

func TestTenantIsolationAndTicketLifecycle(t *testing.T) {
	e := setup(t)
	staff := token("agent-1", "", "support", "manager")
	ta := e.newClient(staff, "Acme "+t.Name())
	tb := e.newClient(staff, "Globex "+t.Name())
	alice := token("alice", ta, "client_user")
	bob := token("bob", tb, "client_user")

	code, tk, hdr := e.do("POST", "/v1/tickets", alice, map[string]any{"subject": "Server down", "description": "prod", "severity": "P1", "channel": "app"})
	if code != 201 || tk["sla_due_at"] == nil || tk["status"] != "open" || hdr.Get("ETag") == "" {
		t.Fatalf("create: %d %v", code, tk)
	}
	id := tk["id"].(string)

	// Bob (other tenant) cannot see or touch Alice's ticket.
	if code, _, _ := e.do("GET", "/v1/tickets/"+id, bob, nil); code != 404 {
		t.Errorf("cross-tenant get: want 404, got %d", code)
	}
	_, bl, _ := e.do("GET", "/v1/tickets", bob, nil)
	if len(bl["data"].([]any)) != 0 {
		t.Errorf("bob must see no tickets, got %v", bl["data"])
	}
	if code, _, _ := e.do("POST", "/v1/tickets/"+id+"/comments", bob, map[string]any{"body": "hi"}); code != 404 {
		t.Errorf("cross-tenant comment: want 404, got %d", code)
	}
	// Client cannot spoof another tenant.
	_, spoof, _ := e.do("POST", "/v1/tickets", alice, map[string]any{"tenant_id": tb, "subject": "x", "severity": "P3", "channel": "portal"})
	if spoof["tenant_id"] != ta {
		t.Errorf("client tenant must come from token, got %v", spoof["tenant_id"])
	}

	// Internal notes are invisible to clients.
	if code, _, _ := e.do("POST", "/v1/tickets/"+id+"/comments", alice, map[string]any{"body": "x", "internal": true}); code != 403 {
		t.Errorf("client internal note: want 403, got %d", code)
	}
	e.do("POST", "/v1/tickets/"+id+"/comments", staff, map[string]any{"body": "secret note", "internal": true})
	e.do("POST", "/v1/tickets/"+id+"/comments", staff, map[string]any{"body": "We are on it"})
	_, cl, _ := e.do("GET", "/v1/tickets/"+id+"/comments", alice, nil)
	if n := len(cl["data"].([]any)); n != 1 {
		t.Errorf("client should see 1 comment, got %d", n)
	}
	_, sl, _ := e.do("GET", "/v1/tickets/"+id+"/comments", staff, nil)
	if n := len(sl["data"].([]any)); n != 2 {
		t.Errorf("staff should see 2 comments, got %d", n)
	}
	_, got, _ := e.do("GET", "/v1/tickets/"+id, staff, nil)
	if got["first_response_at"] == nil {
		t.Error("first staff reply must set first_response_at")
	}

	// Workflow: client cannot resolve; staff can; client confirms and rates.
	if code, _, _ := e.do("PATCH", "/v1/tickets/"+id, alice, map[string]any{"status": "resolved"}); code != 403 {
		t.Errorf("client resolve: want 403, got %d", code)
	}
	if code, _, _ := e.do("PATCH", "/v1/tickets/"+id, staff, map[string]any{"status": "closed"}); code != 422 {
		t.Errorf("open->closed: want 422, got %d", code)
	}
	if code, _, _ := e.do("PATCH", "/v1/tickets/"+id, staff, map[string]any{"status": "in_progress", "assignee": "eng-1"}, "If-Match", `W/"999"`); code != 412 {
		t.Errorf("stale ETag: want 412, got %d", code)
	}
	if code, b, _ := e.do("PATCH", "/v1/tickets/"+id, staff, map[string]any{"status": "in_progress", "assignee": "eng-1"}); code != 200 || b["assignee"] != "eng-1" {
		t.Fatalf("in_progress: %d %v", code, b)
	}
	if code, _, _ := e.do("POST", "/v1/tickets/"+id+"/rating", alice, map[string]any{"rating": 5}); code != 422 {
		t.Errorf("rate before resolve: want 422, got %d", code)
	}
	e.do("PATCH", "/v1/tickets/"+id, staff, map[string]any{"status": "resolved"})
	if code, _, _ := e.do("PATCH", "/v1/tickets/"+id, alice, map[string]any{"status": "closed"}); code != 200 {
		t.Errorf("client confirm: want 200, got %d", code)
	}
	if code, b, _ := e.do("POST", "/v1/tickets/"+id+"/rating", alice, map[string]any{"rating": 5, "comment": "fast"}); code != 200 || b["rating"] != float64(5) {
		t.Errorf("rate: %d %v", code, b)
	}
	if code, b, _ := e.do("GET", "/v1/reports/support", staff, nil); code != 200 || b["avg_rating"] == nil {
		t.Errorf("report: %d %v", code, b)
	}
	if code, _, _ := e.do("GET", "/v1/reports/support", alice, nil); code != 403 {
		t.Errorf("client report: want 403, got %d", code)
	}
}

func TestValidationAndIdempotency(t *testing.T) {
	e := setup(t)
	staff := token("agent-2", "", "support")
	tid := e.newClient(staff, "Idem "+t.Name())
	for name, body := range map[string]map[string]any{
		"no subject":     {"tenant_id": tid, "severity": "P2", "channel": "app"},
		"bad severity":   {"tenant_id": tid, "subject": "x", "severity": "P9", "channel": "app"},
		"bad channel":    {"tenant_id": tid, "subject": "x", "severity": "P2", "channel": "fax"},
		"no tenant":      {"subject": "x", "severity": "P2", "channel": "app"},
		"unknown tenant": {"tenant_id": "00000000-0000-0000-0000-000000000000", "subject": "x", "severity": "P2", "channel": "app"},
	} {
		if code, _, _ := e.do("POST", "/v1/tickets", staff, body); code != 422 {
			t.Errorf("%s: want 422, got %d", name, code)
		}
	}
	// missing Idempotency-Key
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/tickets", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer "+staff)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 400 {
		t.Errorf("missing key: want 400, got %d", resp.StatusCode)
	}
	// same key + same body replays; same key + different body conflicts
	body := map[string]any{"tenant_id": tid, "subject": "once", "severity": "P4", "channel": "email"}
	c1, r1, _ := e.do("POST", "/v1/tickets", staff, body, "Idempotency-Key", "fixed-"+tid)
	c2, r2, h2 := e.do("POST", "/v1/tickets", staff, body, "Idempotency-Key", "fixed-"+tid)
	if c1 != 201 || c2 != 201 || r1["id"] != r2["id"] || h2.Get("Idempotent-Replay") != "true" {
		t.Errorf("replay: %d %d %v %v", c1, c2, r1["id"], r2["id"])
	}
	body["subject"] = "different"
	if code, _, _ := e.do("POST", "/v1/tickets", staff, body, "Idempotency-Key", "fixed-"+tid); code != 422 {
		t.Errorf("key reuse: want 422, got %d", code)
	}
	// pagination
	for i := 0; i < 3; i++ {
		e.do("POST", "/v1/tickets", staff, map[string]any{"tenant_id": tid, "subject": fmt.Sprint("p", i), "severity": "P3", "channel": "app"})
	}
	_, p1, _ := e.do("GET", "/v1/tickets?limit=2", staff, nil)
	if len(p1["data"].([]any)) != 2 || p1["next_cursor"] == nil {
		t.Fatalf("page 1: %v", p1)
	}
	_, p2, _ := e.do("GET", "/v1/tickets?limit=2&cursor="+p1["next_cursor"].(string), staff, nil)
	first := p1["data"].([]any)[0].(map[string]any)["id"]
	for _, x := range p2["data"].([]any) {
		if x.(map[string]any)["id"] == first {
			t.Error("page 2 must not repeat page 1")
		}
	}
	if code, _, _ := e.do("GET", "/v1/tickets?cursor=bogus", staff, nil); code != 400 {
		t.Errorf("bad cursor: want 400, got %d", code)
	}
}

func TestEnquiryAndLeads(t *testing.T) {
	e := setup(t)
	if code, _, _ := e.do("POST", "/v1/public/enquiries", "", map[string]any{"name": "Jo", "email": "not-an-email"}); code != 422 {
		t.Errorf("bad email: want 422, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/public/enquiries", "", map[string]any{"name": "Jo", "email": "jo@example.com", "message": "Pen test quote please"}); code != 202 {
		t.Fatalf("enquiry: want 202, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/public/enquiries", "", map[string]any{"name": "Bot", "email": "b@example.com", "website": "http://spam"}); code != 202 {
		t.Errorf("honeypot should look successful, got %d", code)
	}
	_, leads, _ := e.do("GET", "/v1/leads", token("s", "", "sales"), nil)
	found := false
	for _, l := range leads["data"].([]any) {
		m := l.(map[string]any)
		if m["email"] == "jo@example.com" {
			found = true
		}
		if m["email"] == "b@example.com" {
			t.Error("honeypot lead must not be stored")
		}
	}
	if !found {
		t.Error("enquiry should appear in CRM")
	}
}

func TestDocumentsAndVerification(t *testing.T) {
	e := setup(t)
	staff := token("fin", "", "finance")
	code, d, _ := e.do("POST", "/v1/documents", staff, map[string]any{"code": "HS-CRT-A", "title": "Certificate of Assessment"})
	if code != 201 {
		t.Fatalf("issue: %d %v", code, d)
	}
	ref := d["ref"].(string)
	_, v, _ := e.do("GET", "/v1/public/verify/"+ref, "", nil)
	if v["valid"] != true || v["title"] != "Certificate of Assessment" {
		t.Errorf("verify: %v", v)
	}
	if _, has := v["tenant_id"]; has {
		t.Error("public verification must not expose tenant_id")
	}
	forged := ref[:len(ref)-2] + "9" + ref[len(ref)-1:]
	if _, v, _ := e.do("GET", "/v1/public/verify/"+forged, "", nil); v["valid"] != false {
		t.Errorf("forged ref must be invalid: %v", v)
	}
	if code, _, _ := e.do("POST", "/v1/documents/"+ref+"/revoke", staff, nil); code != 403 {
		t.Errorf("finance revoke: want 403, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/documents/"+ref+"/revoke", token("m", "", "manager"), nil); code != 200 {
		t.Errorf("manager revoke: want 200, got %d", code)
	}
	if _, v, _ := e.do("GET", "/v1/public/verify/"+ref, "", nil); v["valid"] != false || v["status"] != "revoked" {
		t.Errorf("revoked: %v", v)
	}
	_, d2, _ := e.do("POST", "/v1/documents", staff, map[string]any{"code": "INV", "title": "Invoice"})
	_, d3, _ := e.do("POST", "/v1/documents", staff, map[string]any{"code": "INV", "title": "Invoice"})
	if d2["ref"] == d3["ref"] {
		t.Error("sequence must increment")
	}
}

func TestRBACOnClients(t *testing.T) {
	e := setup(t)
	staff := token("sales-1", "", "sales")
	id := e.newClient(staff, "Rbac "+t.Name())
	other := e.newClient(staff, "Other "+t.Name())
	c := token("cu", id, "client_user")
	if code, _, _ := e.do("GET", "/v1/clients", c, nil); code != 403 {
		t.Errorf("client list: want 403, got %d", code)
	}
	if code, _, _ := e.do("GET", "/v1/clients/"+id, c, nil); code != 200 {
		t.Errorf("own account: want 200, got %d", code)
	}
	if code, _, _ := e.do("GET", "/v1/clients/"+other, c, nil); code != 404 {
		t.Errorf("other account: want 404, got %d", code)
	}
	if code, _, _ := e.do("PATCH", "/v1/clients/"+id, staff, map[string]any{"kyc_status": "verified"}); code != 403 {
		t.Errorf("sales KYC change: want 403, got %d", code)
	}
	if code, b, _ := e.do("PATCH", "/v1/clients/"+id, token("m", "", "manager"), map[string]any{"kyc_status": "verified"}); code != 200 || b["kyc_status"] != "verified" {
		t.Errorf("manager KYC: %d %v", code, b)
	}
}

func TestOutboxEvents(t *testing.T) {
	e := setup(t)
	staff := token("a", "", "support")
	tid := e.newClient(staff, "Outbox "+t.Name())
	_, tk, _ := e.do("POST", "/v1/tickets", staff, map[string]any{"tenant_id": tid, "subject": "p1", "severity": "P1", "channel": "phone"})
	pool, _ := platform.NewPool(context.Background(), os.Getenv("HEKATE_TEST_DB"))
	defer pool.Close()
	var n int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM platform.outbox WHERE payload->>'id' = $1 AND subject IN ('ticket.created','ticket.p1.page')`, tk["id"]).Scan(&n)
	if n != 2 {
		t.Errorf("P1 ticket must emit created + page events, got %d", n)
	}
}

func TestDeviceRegistrationIsPerUser(t *testing.T) {
	e := setup(t)
	a, b := token("dev-a", "", "support"), token("dev-b", "", "support")
	tok := fmt.Sprintf("ExponentPushToken[%d]", time.Now().UnixNano())
	put := func(tk, device string) int {
		body, _ := json.Marshal(map[string]string{"token": device, "platform": "expo"})
		req, _ := http.NewRequest("PUT", e.srv.URL+"/v1/devices", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tk)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if c := put(a, tok); c != 204 {
		t.Fatalf("register: want 204, got %d", c)
	}
	if c := put(a, tok); c != 204 {
		t.Fatalf("re-register: want 204, got %d", c)
	}
	if c := put(b, tok); c != 204 { // handed to another user
		t.Fatalf("reassign: want 204, got %d", c)
	}
	owner, err := pgx.Connect(context.Background(), os.Getenv("HEKATE_TEST_DB"))
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close(context.Background())
	var actor string
	if err := owner.QueryRow(context.Background(), `SELECT actor FROM notify.devices WHERE token=$1`, tok).Scan(&actor); err != nil || actor != "dev-b" {
		t.Errorf("token must belong to last registrant, got %q %v", actor, err)
	}
	body, _ := json.Marshal(map[string]string{"token": "x", "platform": "pager"})
	req, _ := http.NewRequest("PUT", e.srv.URL+"/v1/devices", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+a)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 422 {
		t.Errorf("bad platform: want 422, got %d", resp.StatusCode)
	}
}
