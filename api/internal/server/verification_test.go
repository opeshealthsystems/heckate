package server

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
)

func TestPublicVerificationShowsWhatItShouldAndLogsEveryCheck(t *testing.T) {
	e := setup(t)
	fin := token("ver-fin", "", "finance")
	mgr := token("ver-mgr", "", "manager")
	_, d, _ := e.do("POST", "/v1/documents", fin, map[string]any{"code": "INV", "title": "Verification test " + fmt.Sprint(time.Now().UnixNano())})
	ref := d["ref"].(string)

	c, v, _ := e.do("GET", "/v1/public/verify/"+ref, "", nil)
	if c != 200 || v["valid"] != true || v["result"] != "authentic" || v["type"] != "Invoice" || v["version"] != "1.0" ||
		v["classification"] != "confidential" || v["issued_by"] != "HEKATE SYSTEMS SARL" || v["verified_count"] != float64(1) {
		t.Fatalf("a genuine document: %d %v", c, v)
	}
	if _, has := v["tenant_id"]; has {
		t.Error("verification must never say who a document was issued to")
	}
	_, v, _ = e.do("GET", "/v1/public/verify/"+ref, "", nil)
	if v["verified_count"] != float64(2) || v["last_verified"] == nil {
		t.Errorf("each check is counted: %v", v)
	}
	// Made-up references answer "not found" and write nothing.
	_, nf, _ := e.do("GET", "/v1/public/verify/HS-INV-2026-999999", "", nil)
	if nf["valid"] != false || nf["result"] != "not_found" {
		t.Errorf("unknown reference: %v", nf)
	}
	_, mal, _ := e.do("GET", "/v1/public/verify/rubbish", "", nil)
	if mal["valid"] != false || mal["result"] != "malformed" {
		t.Errorf("malformed reference: %v", mal)
	}
	// Staff who issue documents see the history; others do not.
	_, h, _ := e.do("GET", "/v1/documents/"+ref+"/verifications", fin, nil)
	if h["total"] != float64(2) || len(h["recent"].([]any)) != 2 {
		t.Errorf("verification history: %v", h)
	}
	if c, _, _ := e.do("GET", "/v1/documents/"+ref+"/verifications", token("ver-log", "", "logistics"), nil); c != 403 {
		t.Errorf("logistics reading the verification history: want 403, got %d", c)
	}
	// Revocation shows.
	if c, _, _ := e.do("POST", "/v1/documents/"+ref+"/revoke", mgr, map[string]any{}); c != 200 {
		t.Fatalf("revoke: %d", c)
	}
	_, rv, _ := e.do("GET", "/v1/public/verify/"+ref, "", nil)
	if rv["valid"] != false || rv["result"] != "revoked" {
		t.Errorf("a revoked document: %v", rv)
	}
}

func TestSignedQRCodesAreCheckedAgainstTheRegister(t *testing.T) {
	e := setup(t)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	defer docs.UseQRPublicKeyForTesting(pub)()
	fin := token("qr-fin", "", "finance")
	_, d, _ := e.do("POST", "/v1/documents", fin, map[string]any{"code": "QT", "title": "QR test " + fmt.Sprint(time.Now().UnixNano())})
	ref := d["ref"].(string)
	issued, _ := time.Parse(time.RFC3339, d["issued_at"].(string))
	o := docs.Options{PortalURL: "https://x.test", QRSigner: priv}
	good := o.SignedQRURL(ref, issued)
	tok := good[strings.Index(good, "/v/")+3:]

	c, v, _ := e.do("GET", "/v1/public/verify-qr/"+tok, "", nil)
	sig, _ := v["signature"].(map[string]any)
	if c != 200 || v["valid"] != true || v["ref"] != ref || sig["checked"] != true || sig["valid"] != true || sig["scheme"] != "ed25519" {
		t.Fatalf("a genuine signed code: %d %v", c, v)
	}
	// A correct signature over data that does not match the register is not accepted either.
	wrongDate := o.SignedQRURL(ref, issued.AddDate(0, 0, -3))
	if _, w, _ := e.do("GET", "/v1/public/verify-qr/"+wrongDate[strings.Index(wrongDate, "/v/")+3:], "", nil); w["valid"] != false || w["result"] != "signature_invalid" {
		t.Errorf("a signed code whose date differs from the record: %v", w)
	}
	// Forged: right shape, someone else's key; and plain garbage.
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	forged := docs.Options{PortalURL: "https://x.test", QRSigner: other}.SignedQRURL(ref, issued)
	if _, f, _ := e.do("GET", "/v1/public/verify-qr/"+forged[strings.Index(forged, "/v/")+3:], "", nil); f["valid"] != false || f["result"] != "signature_invalid" {
		t.Errorf("a forged code: %v", f)
	}
	if _, g, _ := e.do("GET", "/v1/public/verify-qr/"+base64.RawURLEncoding.EncodeToString([]byte("x")), "", nil); g["valid"] != false {
		t.Errorf("garbage: %v", g)
	}
	_, h, _ := e.do("GET", "/v1/documents/"+ref+"/verifications", fin, nil)
	if h["total"] != float64(3) {
		t.Errorf("the good check and the two refused ones for a real reference are logged: %v", h)
	}
}

func TestAccessRequestsAreDecidedByManagementAndGrantTimeLimitedLinks(t *testing.T) {
	e := setup(t)
	conn, err := pgx.Connect(context.Background(), adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	sfx := fmt.Sprint(time.Now().UnixNano())
	sales := token("acc-sales", "", "sales")
	mgr := token("acc-mgr", "", "manager")
	ta := e.newClient(token("m", "", "manager"), "Access Co "+sfx)
	// An issued proposal whose PDF has been produced (the stored original is what a link serves).
	_, n, _ := e.do("POST", "/v1/narratives", sales, map[string]any{"kind": "proposal", "tenant_id": ta, "title": "Access test " + sfx, "sections": []map[string]any{{"body": "Confidential scope " + sfx}}})
	id := n["id"].(string)
	_, iss, _ := e.do("POST", "/v1/narratives/"+id+"/issue", sales, nil)
	ref := iss["ref"].(string)

	ask := func(body map[string]any) (int, map[string]any) {
		c, b, _ := e.do("POST", "/v1/public/access-requests", "", body)
		return c, b
	}
	base := map[string]any{"ref": ref, "name": "Auditor One", "email": "auditor@bank.example", "organisation": "A Bank", "reason": "Our due-diligence file for the 2026 supplier review needs this proposal."}
	for name, mut := range map[string]func(map[string]any){
		"short reason": func(b map[string]any) { b["reason"] = "too short" },
		"bad email":    func(b map[string]any) { b["email"] = "not-an-email" },
		"no name":      func(b map[string]any) { b["name"] = "" },
		"bad ref":      func(b map[string]any) { b["ref"] = "nope" },
	} {
		b := map[string]any{}
		for k, v := range base {
			b[k] = v
		}
		mut(b)
		if c, _ := ask(b); c != 422 {
			t.Errorf("%s: want 422, got %d", name, c)
		}
	}
	unknown := map[string]any{}
	for k, v := range base {
		unknown[k] = v
	}
	unknown["ref"] = "HS-PR-2026-999999"
	if c, _ := ask(unknown); c != 404 {
		t.Errorf("unknown document: want 404, got %d", c)
	}
	c, req := ask(base)
	if c != 201 || req["status"] != "pending" {
		t.Fatalf("request: %d %v", c, req)
	}
	rid := req["id"].(string)
	ask(base)
	ask(base)
	if c, _ := ask(base); c != 429 {
		t.Errorf("a fourth open request from the same person: want 429, got %d", c)
	}

	// Only management sees and decides.
	if c, _, _ := e.do("GET", "/v1/access-requests", sales, nil); c != 403 {
		t.Errorf("sales listing access requests: want 403, got %d", c)
	}
	if c, _, _ := e.do("GET", "/v1/access-requests", "", nil); c != 401 {
		t.Errorf("anonymous listing: want 401, got %d", c)
	}
	_, list, _ := e.do("GET", "/v1/access-requests?status=pending", mgr, nil)
	found := false
	for _, x := range list["data"].([]any) {
		if m := x.(map[string]any); m["id"] == rid {
			found = m["reason"] == base["reason"] && m["email"] == "auditor@bank.example" && m["ref"] == ref
		}
	}
	if !found {
		t.Fatalf("the manager must see the request with its reason: %v", list)
	}
	// No stored original yet: approval says so rather than issuing a dead link.
	if c, b, _ := e.do("POST", "/v1/access-requests/"+rid+"/approve", mgr, map[string]any{}); c != 409 {
		t.Errorf("approving before the PDF exists: want 409, got %d %v", c, b)
	}
	if r, _ := e.pdf(sales, "/v1/narratives/"+id+"/pdf"); r.StatusCode != 200 {
		t.Fatalf("pdf: %d", r.StatusCode)
	}
	if c, _, _ := e.do("POST", "/v1/access-requests/"+rid+"/approve", mgr, map[string]any{"days": 99}); c != 422 {
		t.Errorf("a 99-day link: want 422, got %d", c)
	}
	c, ok, _ := e.do("POST", "/v1/access-requests/"+rid+"/approve", mgr, map[string]any{"days": 2, "max_downloads": 2, "note": "Approved for the audit file"})
	if c != 200 || ok["max_downloads"] != float64(2) {
		t.Fatalf("approve: %d %v", c, ok)
	}
	link := ok["link"].(string)
	tok := link[strings.LastIndex(link, "/")+1:]
	if !strings.Contains(link, "/access/") || len(tok) < 40 {
		t.Fatalf("a long unguessable link is expected: %s", link)
	}
	if c, _, _ := e.do("POST", "/v1/access-requests/"+rid+"/approve", mgr, map[string]any{}); c != 409 {
		t.Errorf("a second decision: want 409, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/access-requests/"+rid+"/deny", mgr, map[string]any{}); c != 409 {
		t.Errorf("denying a decided request: want 409, got %d", c)
	}
	// The link serves the stored original, twice, then is used up.
	for i := 1; i <= 2; i++ {
		r, b := e.pdf("", "/v1/public/access/"+tok)
		if r.StatusCode != 200 || !bytes.HasPrefix(b, []byte("%PDF")) || r.Header.Get("X-Document-SHA256") == "" {
			t.Fatalf("download %d: %d", i, r.StatusCode)
		}
	}
	if r, _ := e.pdf("", "/v1/public/access/"+tok); r.StatusCode != 404 {
		t.Errorf("a used-up link: want 404, got %d", r.StatusCode)
	}
	if r, _ := e.pdf("", "/v1/public/access/not-a-real-token"); r.StatusCode != 404 {
		t.Errorf("an unknown link: want 404, got %d", r.StatusCode)
	}
	// Expiry and revocation stop a link that still has downloads left.
	_, req2, _ := e.do("POST", "/v1/public/access-requests", "", base)
	_, ok2, _ := e.do("POST", "/v1/access-requests/"+req2["id"].(string)+"/approve", mgr, map[string]any{})
	link2 := ok2["link"].(string)
	tok2 := link2[strings.LastIndex(link2, "/")+1:]
	if _, err := conn.Exec(context.Background(), `UPDATE documents.access_grants SET expires_at = now() - interval '1 minute' WHERE request_id::text = $1`, req2["id"]); err != nil {
		t.Fatal(err)
	}
	if r, _ := e.pdf("", "/v1/public/access/"+tok2); r.StatusCode != 404 {
		t.Errorf("an expired link: want 404, got %d", r.StatusCode)
	}
	_, req3, _ := e.do("POST", "/v1/public/access-requests", "", base)
	if c, _, _ := e.do("POST", "/v1/access-requests/"+req3["id"].(string)+"/deny", mgr, map[string]any{"note": "Not for external release"}); c != 200 {
		t.Errorf("deny: %d", c)
	}
	_, all, _ := e.do("GET", "/v1/access-requests", mgr, nil)
	states := map[string]int{}
	for _, x := range all["data"].([]any) {
		if m := x.(map[string]any); m["ref"] == ref {
			states[m["status"].(string)]++
		}
	}
	if states["approved"] != 2 || states["denied"] != 1 || states["pending"] != 2 {
		t.Errorf("request states: %v", states)
	}
	// Request rows cannot be edited behind the API's back, and the log cannot be altered.
	if _, err := conn.Exec(context.Background(), `UPDATE documents.access_requests SET reason = 'changed to something else entirely' WHERE id::text = $1`, rid); err == nil {
		t.Error("an access request must not be editable")
	}
	if _, err := conn.Exec(context.Background(), `DELETE FROM documents.verifications`); err == nil {
		t.Error("the verification log must be append-only")
	}
}
