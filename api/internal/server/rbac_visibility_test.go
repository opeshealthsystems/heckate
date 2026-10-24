package server

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// rbacVisibility creates one client with an invoice, a quote, a ticket, a project and an issued document, then
// asks every role to list each of them. It reports how many rows each role can see, which is what row-level
// security and the role gates really let through.
func rbacVisibility(t *testing.T) (lists []string, seen map[string]map[string]int, probe func(role, path string) int) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	mgr := token("vis-mgr", "", "manager")
	fin := token("vis-fin", "", "finance")
	sales := token("vis-sales", "", "sales")
	tid := e.newClient(mgr, "Visibility Co "+sfx)
	owner := token("vis-owner", tid, "client_admin")
	_, tk, _ := e.do("POST", "/v1/tickets", owner, map[string]any{"subject": "Visibility ticket " + sfx, "description": "x", "severity": "P3", "channel": "portal"})
	_, inv, _ := e.do("POST", "/v1/invoices", fin, map[string]any{"tenant_id": tid, "title": "Visibility invoice " + sfx, "lines": lines(100_000)})
	e.do("POST", "/v1/invoices/"+inv["id"].(string)+"/issue", fin, nil)
	_, q, _ := e.do("POST", "/v1/quotes", sales, map[string]any{"tenant_id": tid, "title": "Visibility quote " + sfx, "lines": lines(50_000)})
	e.do("POST", "/v1/quotes/"+q["id"].(string)+"/send", sales, nil)
	e.do("POST", "/v1/projects", mgr, map[string]any{"tenant_id": tid, "name": "Visibility project " + sfx})
	_, n, _ := e.do("POST", "/v1/narratives", sales, map[string]any{"kind": "proposal", "tenant_id": tid, "title": "Visibility proposal " + sfx, "sections": []map[string]any{{"body": "text"}}})
	e.do("POST", "/v1/narratives/"+n["id"].(string)+"/issue", sales, nil)
	lists = []string{"/v1/invoices", "/v1/quotes", "/v1/tickets", "/v1/projects", "/v1/narratives", "/v1/interventions", "/v1/attachments", "/v1/leads", "/v1/clients", "/v1/employees", "/v1/leave", "/v1/expenses", "/v1/timesheets", "/v1/suppliers", "/v1/purchase-orders", "/v1/stock", "/v1/assets", "/v1/accounting/entries", "/v1/payroll/runs", "/v1/audit", "/v1/api-keys", "/v1/documents", "/v1/reports/sales"}
	tok := func(role string) string {
		switch role {
		case "client_admin", "client_user":
			return token("vis-c-"+role, tid, role)
		case "supplier":
			return token("vis-supplier", "", "supplier")
		}
		return token("vis-"+role, "", role)
	}
	seen = map[string]map[string]int{}
	for _, role := range rbacRoles {
		if role == "anonymous" {
			continue
		}
		seen[role] = map[string]int{}
		for _, l := range lists {
			code, body, _ := e.do("GET", l, tok(role), nil)
			switch {
			case code == 403 || code == 401:
				seen[role][l] = -1
			case code != 200:
				seen[role][l] = -2
			default:
				n := 0
				for _, k := range []string{"data", "items", "rows", "clients"} {
					if arr, ok := body[k].([]any); ok {
						n = len(arr)
					}
				}
				seen[role][l] = n
			}
		}
	}
	ids := map[string]string{"narrative": n["id"].(string), "ticket": tk["id"].(string), "invoice": inv["id"].(string), "quote": q["id"].(string)}
	probe = func(role, path string) int {
		for k, v := range ids {
			path = strings.ReplaceAll(path, "{"+k+"}", v)
		}
		code, _, _ := e.do("GET", path, tok(role), nil)
		return code
	}
	return lists, seen, probe
}

func TestRBACVisibility(t *testing.T) {
	out := os.Getenv("HEKATE_RBAC_VIS_OUT")
	if out == "" {
		t.Skip("set HEKATE_RBAC_VIS_OUT to generate the visibility table")
	}
	lists, seen, _ := rbacVisibility(t)
	var b strings.Builder
	b.WriteString("list|" + strings.Join(rbacRoles[:len(rbacRoles)-1], "|") + "\n")
	for _, l := range lists {
		cells := []string{}
		for _, r := range rbacRoles[:len(rbacRoles)-1] {
			cells = append(cells, fmt.Sprint(seen[r][l]))
		}
		b.WriteString(l + "|" + strings.Join(cells, "|") + "\n")
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// What each role must and must not see. Counts are of rows in lists that hold other clients' data, so "0 or
// blocked" means the role gate or row-level security kept the data away.
func TestLeastPrivilegeVisibility(t *testing.T) {
	_, seen, probe := rbacVisibility(t)
	none := func(list string, roles ...string) {
		t.Helper()
		for _, r := range roles {
			if seen[r][list] > 0 {
				t.Errorf("%s must not see rows in %s, but sees %d", r, list, seen[r][list])
			}
		}
	}
	some := func(list string, roles ...string) {
		t.Helper()
		for _, r := range roles {
			if seen[r][list] <= 0 {
				t.Errorf("%s must see rows in %s, got %d", r, list, seen[r][list])
			}
		}
	}
	others := []string{"support", "engineer", "hr", "procurement", "logistics", "employee", "auditor", "supplier"}
	some("/v1/invoices", "ceo", "manager", "finance", "sales", "client_admin", "client_user")
	none("/v1/invoices", others...)
	some("/v1/quotes", "ceo", "manager", "finance", "sales", "client_admin", "client_user")
	none("/v1/quotes", others...)
	// A specific proposal and a specific client ticket, looked up by id (lists can legitimately hold other rows
	// that a role is allowed to see, such as an HR form for HR).
	for _, r := range []string{"ceo", "manager", "sales", "client_admin", "client_user"} {
		if c := probe(r, "/v1/narratives/{narrative}"); c != 200 {
			t.Errorf("%s must read the proposal, got %d", r, c)
		}
	}
	for _, r := range []string{"finance", "support", "engineer", "hr", "procurement", "logistics", "employee", "auditor", "supplier"} {
		if c := probe(r, "/v1/narratives/{narrative}"); c == 200 {
			t.Errorf("%s must not read the proposal", r)
		}
	}
	for _, r := range []string{"ceo", "manager", "support", "engineer", "client_admin", "client_user"} {
		if c := probe(r, "/v1/tickets/{ticket}"); c != 200 {
			t.Errorf("%s must read the client's ticket, got %d", r, c)
		}
	}
	for _, r := range []string{"sales", "finance", "hr", "procurement", "logistics", "employee", "auditor", "supplier"} {
		if c := probe(r, "/v1/tickets/{ticket}"); c == 200 {
			t.Errorf("%s must not read the client's ticket", r)
		}
	}
	none("/v1/api-keys", "sales", "finance", "hr", "procurement", "logistics", "employee", "engineer", "auditor", "supplier")
	none("/v1/projects", "auditor", "supplier")
	none("/v1/clients", "hr", "procurement", "logistics", "employee", "client_admin", "client_user", "auditor", "supplier")
}

func TestNarrativeKindsFollowRoles(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	mgr := token("nk-mgr", "", "manager")
	tid := e.newClient(mgr, "Kinds Co "+sfx)
	create := func(tok string, kind, form string) (int, string) {
		body := map[string]any{"kind": kind, "tenant_id": tid, "title": kind + form + sfx, "sections": []map[string]any{{"body": "text"}}}
		if form != "" {
			body["form"] = form
		}
		c, b, _ := e.do("POST", "/v1/narratives", tok, body)
		id, _ := b["id"].(string)
		return c, id
	}
	roles := map[string]string{}
	for _, r := range []string{"sales", "hr", "support", "engineer", "procurement", "finance", "employee", "logistics"} {
		roles[r] = token("nk-"+r, "", r)
	}
	type want struct {
		role, kind, form string
		code             int
	}
	for _, w := range []want{
		{"sales", "proposal", "", 201}, {"sales", "form", "FRM-23", 201}, {"sales", "form", "FRM-12", 403}, {"sales", "form", "FRM-13", 403}, {"sales", "pentest_report", "", 403},
		{"hr", "form", "FRM-32", 201}, {"hr", "form", "FRM-33", 201}, {"hr", "proposal", "", 403}, {"hr", "form", "FRM-13", 403},
		{"support", "form", "FRM-14", 201}, {"support", "form", "FRM-32", 403}, {"support", "proposal", "", 403},
		{"engineer", "pentest_report", "", 201}, {"engineer", "form", "FRM-14", 201}, {"engineer", "form", "FRM-33", 403},
		{"procurement", "form", "FRM-39", 201}, {"procurement", "form", "FRM-13", 403},
		{"finance", "proposal", "", 403}, {"employee", "proposal", "", 403}, {"logistics", "form", "FRM-39", 403},
	} {
		if c, _ := create(roles[w.role], w.kind, w.form); c != w.code {
			t.Errorf("%s creating %s %s: want %d, got %d", w.role, w.kind, w.form, w.code, c)
		}
	}
	// Restricted forms stay with management: sales and HR cannot even read them.
	for _, form := range []string{"FRM-12", "FRM-13"} {
		c, id := create(mgr, "form", form)
		if c != 201 {
			t.Fatalf("manager creating %s: %d", form, c)
		}
		for _, r := range []string{"sales", "hr", "support", "finance", "employee"} {
			if code, _, _ := e.do("GET", "/v1/narratives/"+id, roles[r], nil); code != 404 {
				t.Errorf("%s reading restricted %s: want 404, got %d", r, form, code)
			}
		}
		if code, _, _ := e.do("GET", "/v1/narratives/"+id, mgr, nil); code != 200 {
			t.Errorf("manager reading %s: %d", form, code)
		}
	}
}
