package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlanRules(t *testing.T) {
	// A staff status change goes to the client's devices only, never back to the person who made it.
	m, a := Plan("ticket.status_changed", map[string]any{"id": "t1", "tenant_id": "ten", "subject": "Rack down", "to": "resolved", "actor": "agent", "by": "staff"})
	if m == nil || a.Tenant != "ten" || a.Except != "agent" || m.Body != "Rack down is now resolved" {
		t.Fatalf("status change: %+v %+v", m, a)
	}
	for _, r := range a.Roles {
		if r != "client_user" && r != "client_admin" {
			t.Errorf("status change must target clients only, got role %s", r)
		}
	}
	// A client reply goes to support staff; a staff reply goes to the client.
	_, a = Plan("ticket.commented", map[string]any{"id": "t1", "tenant_id": "ten", "by": "client", "actor": "alice"})
	if a.Tenant != "" || len(a.Roles) == 0 || a.Roles[0] != "support" {
		t.Errorf("client reply must reach support, got %+v", a)
	}
	_, a = Plan("ticket.commented", map[string]any{"id": "t1", "tenant_id": "ten", "by": "staff", "actor": "agent"})
	if a.Tenant != "ten" {
		t.Errorf("staff reply must reach the tenant, got %+v", a)
	}
	// A P1 pages responders everywhere and never a client.
	m, a = Plan("ticket.p1.page", map[string]any{"id": "t1", "subject": "Down"})
	if m.Title != "P1 incident" || a.Tenant != "" {
		t.Errorf("p1: %+v %+v", m, a)
	}
	for _, r := range a.Roles {
		if r == "client_user" || r == "client_admin" {
			t.Error("a P1 page must not go to clients")
		}
	}
	if _, a = Plan("intervention.scheduled", map[string]any{"engineer": "eng-1"}); a.Actor != "eng-1" {
		t.Errorf("a site visit goes to its engineer, got %+v", a)
	}
	if _, a = Plan("leave.rejected", map[string]any{"actor": "emp"}); a.Actor != "emp" {
		t.Errorf("leave outcome goes to the requester, got %+v", a)
	}
	// Events that are not for phones produce nothing.
	for _, s := range []string{"client.created", "stock.received", "ticket.created", "rfq.opened", "payment.recorded", "nonsense"} {
		if m, _ := Plan(s, map[string]any{}); m != nil {
			t.Errorf("%s must not push", s)
		}
	}
}

func TestExpoSenderRequestAndResults(t *testing.T) {
	var got []PushMessage
	var auth, ctype string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, ctype = r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"data":[{"status":"ok","id":"x"},{"status":"error","message":"gone","details":{"error":"DeviceNotRegistered"}}]}`))
	}))
	defer srv.Close()
	s := &ExpoSender{URL: srv.URL, AccessToken: "secret", Client: srv.Client()}
	res, err := s.Send(context.Background(), []PushMessage{{To: "ExponentPushToken[a]", Title: "t", Body: "b"}, {To: "ExponentPushToken[b]", Title: "t", Body: "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].To != "ExponentPushToken[a]" || auth != "Bearer secret" || ctype != "application/json" {
		t.Errorf("request wrong: %+v auth=%q ctype=%q", got, auth, ctype)
	}
	if res[0].Status != "ok" || res[1].Error != "DeviceNotRegistered" {
		t.Errorf("results wrong: %+v", res)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer bad.Close()
	if _, err := (&ExpoSender{URL: bad.URL, Client: bad.Client()}).Send(context.Background(), []PushMessage{{To: "x"}}); err == nil {
		t.Error("a 500 from the push service must surface as an error")
	}
	short := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"data":[]}`)) }))
	defer short.Close()
	if _, err := (&ExpoSender{URL: short.URL, Client: short.Client()}).Send(context.Background(), []PushMessage{{To: "x"}}); err == nil {
		t.Error("a response that does not match the request must be an error")
	}
}
