package server

import (
	"fmt"
	"testing"
	"time"
)

// A client lists only their own company's register, and never Internal or Restricted classes.
func TestClientSeesOnlyOwnConfidentialOrPublicDocuments(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	mgr := token("docs-mgr", "", "manager")
	ta := e.newClient(mgr, "Docs A "+sfx)
	tb := e.newClient(mgr, "Docs B "+sfx)
	issue := func(tid, code, title string) string {
		c, d, _ := e.do("POST", "/v1/documents", mgr, map[string]any{"code": code, "title": title, "tenant_id": tid})
		if c != 201 {
			t.Fatalf("issue %s: %d %v", code, c, d)
		}
		return d["ref"].(string)
	}
	qtA := issue(ta, "QT", "A quote "+sfx)
	crtA := issue(ta, "CRT", "A certificate "+sfx)
	rptA := issue(ta, "RPT", "A restricted report "+sfx)
	frmA := issue(ta, "FRM-15", "A internal form "+sfx)
	qtB := issue(tb, "QT", "B quote "+sfx)
	refs := func(tok string) map[string]bool {
		c, body, _ := e.do("GET", "/v1/documents?limit=200", tok, nil)
		if c != 200 {
			t.Fatalf("list: %d", c)
		}
		out := map[string]bool{}
		for _, x := range body["data"].([]any) {
			out[x.(map[string]any)["ref"].(string)] = true
		}
		return out
	}
	for _, role := range []string{"client_admin", "client_user"} {
		got := refs(token("docs-"+role, ta, role))
		if !got[qtA] || !got[crtA] {
			t.Errorf("%s should see own quote and certificate: %v", role, got)
		}
		if got[rptA] || got[frmA] {
			t.Errorf("%s must not see Restricted or Internal documents: %v", role, got)
		}
		if got[qtB] {
			t.Errorf("%s sees another client's document", role)
		}
	}
	a := token("docs-a", ta, "client_admin")
	if c, _, _ := e.do("GET", "/v1/documents/"+qtB, a, nil); c != 404 {
		t.Errorf("other tenant's document by ref: want 404, got %d", c)
	}
	if c, _, _ := e.do("GET", "/v1/documents/"+rptA, a, nil); c != 404 {
		t.Errorf("restricted document by ref: want 404, got %d", c)
	}
	all := refs(mgr)
	for _, r := range []string{qtA, rptA, frmA, qtB} {
		if !all[r] {
			t.Errorf("staff should see %s", r)
		}
	}
}
