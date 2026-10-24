package server

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

var rbacRoles = []string{"ceo", "manager", "sales", "support", "engineer", "finance", "hr", "procurement", "logistics", "employee", "auditor", "client_admin", "client_user", "supplier", "anonymous"}

// rbacMatrix calls every route as every role with an empty body and an unknown id, and records whether the
// role gate lets the caller through. 401 and 403 mean blocked; anything else (404, 409, 422, 200) means the
// request got past the role gate and was judged on its content.
func rbacMatrix(t *testing.T) (routes []string, allowed map[string]map[string]bool) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	mgr := token("rbac-mgr", "", "manager")
	client := e.newClient(mgr, "RBAC Co "+sfx)
	supplier := mkSupplier(t, e, token("rbac-buyer", "", "procurement"), "RBAC Supplier "+sfx)
	tok := func(role string) string {
		switch role {
		case "anonymous":
			return ""
		case "client_admin", "client_user":
			return token("rbac-"+role, client, role)
		case "supplier":
			return token("rbac-supplier", supplier, "supplier")
		}
		return token("rbac-"+role, "", role)
	}
	allowed = map[string]map[string]bool{}
	routes = Patterns()
	sort.Strings(routes)
	for _, pat := range routes {
		method, path, _ := strings.Cut(pat, " ")
		if strings.HasPrefix(path, "/v1/stream") || path == "/metrics" {
			continue // upgrade endpoints and the internal listener are checked elsewhere
		}
		path = strings.NewReplacer("{id}", "00000000-0000-0000-0000-000000000000", "{ref}", "HS-INV-2000-000001", "{employee_id}", "00000000-0000-0000-0000-000000000000", "{account}", "521", "{key}", "00000000-0000-0000-0000-000000000000").Replace(path)
		path = strings.NewReplacer("{", "", "}", "").Replace(path)
		allowed[pat] = map[string]bool{}
		for _, role := range rbacRoles {
			var body any
			if method == "POST" || method == "PUT" || method == "PATCH" {
				body = map[string]any{}
			}
			code, _, _ := e.do(method, path, tok(role), body)
			allowed[pat][role] = code != 401 && code != 403
		}
	}
	return routes, allowed
}

func TestRBACMatrix(t *testing.T) {
	out := os.Getenv("HEKATE_RBAC_OUT")
	if out == "" {
		t.Skip("set HEKATE_RBAC_OUT to a file to generate the matrix")
	}
	routes, allowed := rbacMatrix(t)
	var b strings.Builder
	b.WriteString("route|" + strings.Join(rbacRoles, "|") + "\n")
	for _, pat := range routes {
		a, ok := allowed[pat]
		if !ok {
			continue
		}
		cells := make([]string, len(rbacRoles))
		for i, r := range rbacRoles {
			if a[r] {
				cells[i] = "A"
			} else {
				cells[i] = "-"
			}
		}
		b.WriteString(pat + "|" + strings.Join(cells, "|") + "\n")
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The role gates, as a standing rule: who can reach a route at all (row-level security then narrows what
// they see). Anything that moves data outside these lists is a change to access control and must be argued
// for in review, not slip in because a route was registered without a role list.
func TestRouteGatesStayNarrow(t *testing.T) {
	routes, allowed := rbacMatrix(t)
	public := map[string]bool{"GET /healthz": true, "GET /livez": true, "GET /readyz": true, "GET /v1/public/verify/{ref}": true, "POST /v1/public/enquiries": true,
		"GET /v1/public/verify-qr/{token}": true, "POST /v1/public/access-requests": true, "GET /v1/public/access/{token}": true}
	// Writes a role may make besides its own work: its own devices, a ticket it raises, its own leave and so on.
	selfService := map[string]bool{
		"PUT /v1/devices": true, "DELETE /v1/devices/{token}": true, "POST /v1/tickets": true, "PATCH /v1/tickets/{id}": true, "POST /v1/tickets/{id}/comments": true,
		"POST /v1/attachments": true, "POST /v1/expenses": true, "POST /v1/leave": true, "POST /v1/leave/{id}/cancel": true,
		"POST /v1/timesheets": true, "POST /v1/timesheets/{id}/submit": true,
	}
	for _, pat := range routes {
		a, ok := allowed[pat]
		if !ok {
			continue
		}
		mutating := !strings.HasPrefix(pat, "GET ")
		if a["anonymous"] && !public[pat] {
			t.Errorf("%s is open to anonymous callers", pat)
		}
		if a["auditor"] && mutating && !public[pat] {
			t.Errorf("the read-only auditor can write through %s", pat)
		}
		if a["supplier"] && mutating && !public[pat] && !strings.Contains(pat, "/rfqs/") && !strings.Contains(pat, "/purchase-orders/") && pat != "PUT /v1/suppliers/me/bank" {
			t.Errorf("a supplier can write through %s", pat)
		}
		if a["employee"] && mutating && !public[pat] && !selfService[pat] {
			t.Errorf("a plain employee can write through %s", pat)
		}
		for _, role := range []string{"auditor", "supplier"} {
			if a[role] && strings.HasPrefix(pat, "GET /v1/") && (strings.Contains(pat, "/invoices") || strings.Contains(pat, "/quotes") || strings.Contains(pat, "/tickets") || strings.Contains(pat, "/projects") || strings.Contains(pat, "/clients/") || strings.Contains(pat, "/api-keys")) {
				if !(role == "supplier" && strings.Contains(pat, "/supplier-invoices")) {
					t.Errorf("%s can reach the client-facing route %s", role, pat)
				}
			}
		}
	}
}
