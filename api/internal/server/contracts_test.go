package server

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/modules/contracts"
)

func day(offset int) string { return time.Now().UTC().AddDate(0, 0, offset).Format("2006-01-02") }

func listOf(t *testing.T, b map[string]any) []any {
	t.Helper()
	l, _ := b["data"].([]any)
	return l
}

func TestContractLifecycleSLAAndClientView(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	mgr := token("ct-mgr", "", "manager")
	sales := token("ct-sales", "", "sales")
	eng := token("ct-eng", "", "engineer")
	fin := token("ct-fin", "", "finance")
	hr := token("ct-hr", "", "hr")
	ta := e.newClient(mgr, "Bank Co "+sfx)
	tb := e.newClient(mgr, "Hospital Co "+sfx)
	clientA := token("ct-ca", ta, "client_admin")
	clientB := token("ct-cb", tb, "client_admin")

	body := map[string]any{"tenant_id": ta, "title": "Five-year maintenance", "kind": "maintenance", "start_date": day(0), "end_date": day(5 * 365),
		"value": 60_000_000, "billing": "monthly", "period_amount": 1_000_000, "auto_renew": true, "renewal_term_months": 12}
	for who, tok := range map[string]string{"engineer": eng, "finance": fin, "hr": hr} {
		if code, _, _ := e.do("POST", "/v1/contracts", tok, body); code != 403 {
			t.Errorf("%s must not create contracts: %d", who, code)
		}
	}
	bad := map[string]any{"tenant_id": ta, "title": "x", "start_date": day(10), "end_date": day(1)}
	if code, _, _ := e.do("POST", "/v1/contracts", sales, bad); code != 422 {
		t.Errorf("end before start: want 422, got %d", code)
	}
	code, c, _ := e.do("POST", "/v1/contracts", sales, body)
	if code != 201 || c["status"] != "draft" || !strings.HasPrefix(c["number"].(string), "CT-") || c["term_months"].(float64) != 60 {
		t.Fatalf("create: %d %v", code, c)
	}
	id := c["id"].(string)

	// A draft is internal: the client cannot see it, and it cannot be activated without clauses.
	if code, _, _ := e.do("GET", "/v1/contracts/"+id, clientA, nil); code != 404 {
		t.Errorf("client must not see a draft, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/contracts/"+id+"/activate", sales, nil); code != 422 {
		t.Errorf("activate without clauses: want 422, got %d", code)
	}
	if code, _, _ := e.do("PUT", "/v1/contracts/"+id+"/clauses", sales, map[string]any{"clauses": []map[string]any{{"title": "Scope", "body": ""}}}); code != 422 {
		t.Errorf("empty clause text: want 422, got %d", code)
	}
	code, c, _ = e.do("PUT", "/v1/contracts/"+id+"/clauses", sales, map[string]any{"clauses": []map[string]any{
		{"title": "Scope", "body": "Preventive and corrective maintenance of the server room."}, {"title": "Payment", "body": "Monthly in advance."}}})
	if code != 200 || len(c["clauses"].([]any)) != 2 {
		t.Fatalf("clauses: %d %v", code, c)
	}
	// P1 within 10 minutes / 60 minutes: tighter than the standard 15 / 240.
	code, c, _ = e.do("PUT", "/v1/contracts/"+id+"/sla", sales, map[string]any{"sla": []map[string]any{{"severity": "P1", "respond_minutes": 10, "resolve_minutes": 60}}})
	if code != 200 || len(c["sla"].([]any)) != 1 {
		t.Fatalf("sla: %d %v", code, c)
	}
	if code, _, _ := e.do("PUT", "/v1/contracts/"+id+"/sla", sales, map[string]any{"sla": []map[string]any{{"severity": "P1", "respond_minutes": 90, "resolve_minutes": 60}}}); code != 422 {
		t.Errorf("resolve before respond: want 422, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/contracts/"+id+"/activate", sales, nil); code != 200 {
		t.Fatalf("activate: %d", code)
	}
	if code, _, _ := e.do("PUT", "/v1/contracts/"+id+"/clauses", sales, map[string]any{"clauses": []map[string]any{{"title": "Changed", "body": "after signing"}}}); code != 409 {
		t.Errorf("agreed clauses must not change: want 409, got %d", code)
	}

	// The client sees its own signed contract, with clauses and promised times, but not the internal history or links.
	code, v, _ := e.do("GET", "/v1/contracts/"+id, clientA, nil)
	if code != 200 || len(v["clauses"].([]any)) != 2 || len(v["history"].([]any)) != 0 {
		t.Errorf("client view: %d %v", code, v)
	}
	if code, _, _ := e.do("GET", "/v1/contracts/"+id, clientB, nil); code != 404 {
		t.Errorf("another client must not see it, got %d", code)
	}
	code, l, _ := e.do("GET", "/v1/contracts", clientB, nil)
	if code != 200 || len(listOf(t, l)) != 0 {
		t.Errorf("other client's list must be empty: %d %v", code, l)
	}

	// A ticket raised by this client now carries the contract's promise.
	code, tk, _ := e.do("POST", "/v1/tickets", clientA, map[string]any{"subject": "Server down", "description": "all down", "severity": "P1", "channel": "portal"})
	if code != 201 {
		t.Fatalf("ticket: %d %v", code, tk)
	}
	due, _ := time.Parse(time.RFC3339, tk["respond_due_at"].(string))
	if d := time.Until(due); d > 11*time.Minute || d < 8*time.Minute {
		t.Errorf("P1 response should be due in 10 minutes under the contract, due in %v", d)
	}
	// Another client keeps the standard 15 minutes.
	_, tk2, _ := e.do("POST", "/v1/tickets", clientB, map[string]any{"subject": "Server down", "description": "all down", "severity": "P1", "channel": "portal"})
	due2, _ := time.Parse(time.RFC3339, tk2["respond_due_at"].(string))
	if d := time.Until(due2); d < 13*time.Minute {
		t.Errorf("a client without a contract keeps the standard target, due in %v", d)
	}

	// Links: only things that belong to this client.
	_, pa, _ := e.do("POST", "/v1/projects", mgr, map[string]any{"tenant_id": ta, "name": "Rack refresh"})
	_, pb, _ := e.do("POST", "/v1/projects", mgr, map[string]any{"tenant_id": tb, "name": "Other"})
	if code, _, _ := e.do("POST", "/v1/contracts/"+id+"/links", sales, map[string]any{"kind": "project", "ref_id": pb["id"]}); code != 422 {
		t.Errorf("link to another client's project: want 422, got %d", code)
	}
	code, c, _ = e.do("POST", "/v1/contracts/"+id+"/links", sales, map[string]any{"kind": "project", "ref_id": pa["id"], "label": "Covered project"})
	if code != 201 || len(c["links"].([]any)) != 1 {
		t.Fatalf("link: %d %v", code, c)
	}
	if code, _, _ := e.do("POST", "/v1/contracts/"+id+"/links", sales, map[string]any{"kind": "robot", "ref_id": pa["id"]}); code != 422 {
		t.Errorf("unknown link kind: want 422, got %d", code)
	}

	// Suspend, resume, renew, terminate, each logged.
	if code, _, _ := e.do("POST", "/v1/contracts/"+id+"/suspend", sales, map[string]any{}); code != 422 {
		t.Errorf("suspend needs a reason: %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/contracts/"+id+"/suspend", sales, map[string]any{"reason": "payment overdue"}); code != 200 {
		t.Errorf("suspend: %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/contracts/"+id+"/renew", sales, map[string]any{"months": 6}); code != 409 {
		t.Errorf("renew while suspended: want 409, got %d", code)
	}
	e.do("POST", "/v1/contracts/"+id+"/resume", sales, map[string]any{})
	code, c, _ = e.do("POST", "/v1/contracts/"+id+"/renew", sales, map[string]any{"months": 12, "value": 72_000_000})
	if code != 200 || c["value"].(float64) != 72_000_000 {
		t.Errorf("renew: %d %v", code, c)
	}
	if code, _, _ := e.do("POST", "/v1/contracts/"+id+"/terminate", sales, map[string]any{}); code != 422 {
		t.Errorf("terminate needs a reason: %d", code)
	}
	code, c, _ = e.do("POST", "/v1/contracts/"+id+"/terminate", sales, map[string]any{"reason": "client moved"})
	if code != 200 || c["status"] != "terminated" {
		t.Errorf("terminate: %d %v", code, c)
	}
	var actions []string
	for _, h := range c["history"].([]any) {
		actions = append(actions, h.(map[string]any)["action"].(string))
	}
	want := "created clauses_set sla_set activated linked suspend resume renewed terminated"
	if strings.Join(actions, " ") != want {
		t.Errorf("history = %v, want %s", actions, want)
	}
	if code, _, _ := e.do("PATCH", "/v1/contracts/"+id, sales, map[string]any{"title": "x"}); code != 409 {
		t.Errorf("an ended contract is closed: want 409, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/contracts/"+id+"/activate", sales, nil); code != 409 {
		t.Errorf("activate twice: want 409, got %d", code)
	}
	resp, pdf := e.pdf(mgr, "/v1/contracts/"+id+"/pdf")
	if resp.StatusCode != 200 || !strings.HasPrefix(string(pdf), "%PDF") {
		t.Errorf("contract pdf: %d", resp.StatusCode)
	}
}

func TestContractRenewalsAreWarnedOnceAndEnded(t *testing.T) {
	e := setup(t)
	pool := testPool(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	mgr := token("cr-mgr", "", "manager")
	ta := e.newClient(mgr, "Renewal Co "+sfx)

	make := func(title string, start, end int, auto bool) string {
		_, c, _ := e.do("POST", "/v1/contracts", mgr, map[string]any{"tenant_id": ta, "title": title, "start_date": day(start), "end_date": day(end), "auto_renew": auto, "renewal_term_months": 12})
		id := c["id"].(string)
		e.do("PUT", "/v1/contracts/"+id+"/clauses", mgr, map[string]any{"clauses": []map[string]any{{"title": "Scope", "body": "x"}}})
		if code, _, _ := e.do("POST", "/v1/contracts/"+id+"/activate", mgr, nil); code != 200 {
			t.Fatalf("activate %s: %d", title, code)
		}
		return id
	}
	soon := make("Soon", -300, 25, false)
	lapsed := make("Lapsed", -400, -3, false)
	auto := make("Auto", -400, -3, true)
	far := make("Far", -10, 400, false)

	if n, err := contracts.RunOnce(context.Background(), pool); err != nil || n < 3 {
		t.Fatalf("run: %d %v", n, err)
	}
	state := func(id string) map[string]any { _, c, _ := e.do("GET", "/v1/contracts/"+id, mgr, nil); return c }
	actions := func(c map[string]any) string {
		var a []string
		for _, h := range c["history"].([]any) {
			a = append(a, h.(map[string]any)["action"].(string))
		}
		return strings.Join(a, " ")
	}
	if c := state(soon); c["status"] != "active" || !strings.Contains(actions(c), "renewal_due") {
		t.Errorf("soon: %v %s", c["status"], actions(c))
	}
	if c := state(lapsed); c["status"] != "expired" {
		t.Errorf("lapsed contract must expire, got %v", c["status"])
	}
	if c := state(auto); c["status"] != "active" || !strings.Contains(actions(c), "auto_renewed") || c["days_left"].(float64) < 300 {
		t.Errorf("auto-renewing contract must be extended: %v %v %s", c["status"], c["days_left"], actions(c))
	}
	if c := state(far); strings.Contains(actions(c), "renewal_due") {
		t.Errorf("a contract 400 days from its end must not be warned about")
	}
	// A second pass changes nothing: the warning is recorded once per term.
	before := actions(state(soon))
	contracts.RunOnce(context.Background(), pool)
	if after := actions(state(soon)); after != before {
		t.Errorf("warned twice: %q then %q", before, after)
	}
	// The expiring list shows the one about to end, not the far one.
	code, l, _ := e.do("GET", "/v1/contracts/expiring?days=60", mgr, nil)
	found := map[string]bool{}
	for _, x := range listOf(t, l) {
		found[x.(map[string]any)["id"].(string)] = true
	}
	if code != 200 || !found[soon] || found[far] {
		t.Errorf("expiring list wrong: %d %v", code, found)
	}
	// An expired contract can be renewed back to life.
	if code, c, _ := e.do("POST", "/v1/contracts/"+lapsed+"/renew", mgr, map[string]any{"months": 24}); code != 200 || c["status"] != "active" {
		t.Errorf("renew expired: %d %v", code, c)
	}
}
