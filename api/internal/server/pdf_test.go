package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

func (e *env) pdf(tok, path string) (*http.Response, []byte) {
	req, _ := http.NewRequest("GET", e.srv.URL+path, nil)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return resp, buf.Bytes()
}

func hexSum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func TestInvoicePDFIsOneStableVerifiableFile(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	fin := token("pdf-fin", "", "finance")
	ta := e.newClient(token("m", "", "manager"), "Société PDF "+sfx)
	tb := e.newClient(token("m", "", "manager"), "Other PDF "+sfx)
	alice := token("pdf-alice", ta, "client_user")
	bob := token("pdf-bob", tb, "client_user")

	_, inv, _ := e.do("POST", "/v1/invoices", fin, map[string]any{"tenant_id": ta, "title": "Pen test " + sfx, "lines": lines(1_000_000)})
	id := inv["id"].(string)
	if resp, _ := e.pdf(fin, "/v1/invoices/"+id+"/pdf"); resp.StatusCode != 409 {
		t.Errorf("a draft invoice has no PDF: want 409, got %d", resp.StatusCode)
	}
	_, issued, _ := e.do("POST", "/v1/invoices/"+id+"/issue", fin, nil)
	ref := issued["ref"].(string)

	resp, first := e.pdf(fin, "/v1/invoices/"+id+"/pdf")
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/pdf" || !bytes.HasPrefix(first, []byte("%PDF-")) {
		t.Fatalf("pdf: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	for _, want := range []string{ref, "1 192 500 XAF", "Pen test " + sfx, "https://app.example.com/verify?ref=" + ref} {
		if !bytes.Contains(first, []byte(want)) {
			t.Errorf("the invoice PDF must contain %q", want)
		}
	}
	if resp.Header.Get("X-Document-SHA256") != hexSum(first) {
		t.Error("the hash header must be the SHA-256 of the body")
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment;") || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("unsafe headers: %v", resp.Header)
	}

	// The public register now carries the hash, so anyone can check a file they hold.
	_, v, _ := e.do("GET", "/v1/public/verify/"+ref, "", nil)
	if v["valid"] != true || v["sha256"] != hexSum(first) {
		t.Errorf("public verification must expose the file hash: %v", v)
	}

	// One reference is one file: payments, other callers and even a different letterhead change nothing.
	e.do("POST", "/v1/invoices/"+id+"/payments", fin, map[string]any{"amount": 100000, "method": "bank"})
	_, again := e.pdf(fin, "/v1/invoices/"+id+"/pdf")
	_, asClient := e.pdf(alice, "/v1/invoices/"+id+"/pdf")
	if !bytes.Equal(first, again) || !bytes.Equal(first, asClient) {
		t.Error("every download of an issued invoice must be byte-identical")
	}
	srv2 := httptest.NewServer(New(platform.Config{DevSecret: secret, PortalURL: "https://app.example.com", Letterhead: []string{"New letterhead line"}}, testPool(t)))
	defer srv2.Close()
	_, elsewhere := (&env{t, srv2}).pdf(fin, "/v1/invoices/"+id+"/pdf")
	if !bytes.Equal(first, elsewhere) {
		t.Error("a changed letterhead must not change an already-issued document")
	}
	if bytes.Contains(first, []byte("100 000 XAF")) {
		t.Error("the issued invoice must not print payments")
	}

	// Access follows the invoice's row-level security.
	if resp, _ := e.pdf(bob, "/v1/invoices/"+id+"/pdf"); resp.StatusCode != 404 {
		t.Errorf("another client: want 404, got %d", resp.StatusCode)
	}
	if resp, _ := e.pdf("", "/v1/invoices/"+id+"/pdf"); resp.StatusCode != 401 {
		t.Errorf("anonymous: want 401, got %d", resp.StatusCode)
	}

	// The stored file and the registered hash cannot be altered, even with raw SQL.
	conn, err := pgx.Connect(context.Background(), adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	for name, sql := range map[string]string{
		"edit artifact":   `UPDATE documents.artifacts SET content = '\x00' WHERE ref = $1`,
		"delete artifact": `DELETE FROM documents.artifacts WHERE ref = $1`,
		"change hash":     `UPDATE documents.register SET sha256 = 'deadbeef' WHERE ref = $1`,
	} {
		if _, err := conn.Exec(context.Background(), sql, ref); err == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
}

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := platform.NewPool(context.Background(), setupDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestQuotePDF(t *testing.T) {
	e := setup(t)
	sales := token("pdf-sales", "", "sales")
	ta := e.newClient(token("m", "", "manager"), "Quote PDF "+fmt.Sprint(time.Now().UnixNano()))
	admin := token("pdf-admin", ta, "client_admin")
	_, q, _ := e.do("POST", "/v1/quotes", sales, map[string]any{"tenant_id": ta, "title": "SOC subscription", "notes": "Payable in 30 days.", "lines": lines(2_000_000)})
	id := q["id"].(string)
	if resp, _ := e.pdf(sales, "/v1/quotes/"+id+"/pdf"); resp.StatusCode != 409 {
		t.Errorf("a draft quote has no PDF: want 409, got %d", resp.StatusCode)
	}
	if resp, _ := e.pdf(admin, "/v1/quotes/"+id+"/pdf"); resp.StatusCode != 404 {
		t.Errorf("the client cannot see a draft quote's PDF: want 404, got %d", resp.StatusCode)
	}
	_, sent, _ := e.do("POST", "/v1/quotes/"+id+"/send", sales, nil)
	ref := sent["ref"].(string)
	resp, b := e.pdf(admin, "/v1/quotes/"+id+"/pdf")
	if resp.StatusCode != 200 || !bytes.Contains(b, []byte(ref)) || !bytes.Contains(b, []byte("QUOTATION")) || !bytes.Contains(b, []byte("2 385 000 XAF")) {
		t.Errorf("quote PDF: %d (ref=%s)", resp.StatusCode, ref)
	}
}

func TestInterventionPDFCarriesTheSignature(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	tid := e.newClient(token("m", "", "manager"), "Visit PDF "+sfx)
	other := e.newClient(token("m", "", "manager"), "Other Visit PDF "+sfx)
	eng := token("pdf-eng-"+sfx, "", "engineer")
	_, iv, _ := e.do("POST", "/v1/interventions", token("d", "", "support"), map[string]any{"tenant_id": tid, "engineer": "pdf-eng-" + sfx, "summary": "Replace switch"})
	id := iv["id"].(string)
	if resp, _ := e.pdf(eng, "/v1/interventions/"+id+"/pdf"); resp.StatusCode != 409 {
		t.Errorf("an unfinished visit has no PDF: want 409, got %d", resp.StatusCode)
	}
	e.do("POST", "/v1/interventions/"+id+"/check-in", eng, map[string]any{"lat": 4.05, "lng": 9.7})
	img := image.NewRGBA(image.Rect(0, 0, 60, 30))
	for x := 5; x < 55; x++ {
		img.Set(x, 15, color.Black)
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	e.do("POST", "/v1/interventions/"+id+"/check-out", eng, map[string]any{"lat": 4.06, "lng": 9.71, "report": "Replaced the faulty switch and verified the uplinks",
		"signed_by": "Jean Dupont", "signature": "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())})

	resp, b := e.pdf(token("c", tid, "client_user"), "/v1/interventions/"+id+"/pdf")
	if resp.StatusCode != 200 {
		t.Fatalf("client's visit PDF: %d", resp.StatusCode)
	}
	for _, want := range []string{"HS-FRM-27-", "Replaced the faulty switch", "Jean Dupont", "4.05000, 9.70000", "/Subtype /Image"} {
		if !bytes.Contains(b, []byte(want)) {
			t.Errorf("the visit PDF must contain %q", want)
		}
	}
	if resp, _ := e.pdf(token("c2", other, "client_user"), "/v1/interventions/"+id+"/pdf"); resp.StatusCode != 404 {
		t.Errorf("another client: want 404, got %d", resp.StatusCode)
	}
}

func TestReceiptAndCreditNotePDFs(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	fin := token("rc-fin", "", "finance")
	ta := e.newClient(token("m", "", "manager"), "Receipt Co "+sfx)
	tb := e.newClient(token("m", "", "manager"), "Other Receipt Co "+sfx)
	alice := token("rc-alice", ta, "client_user")
	bob := token("rc-bob", tb, "client_user")

	_, inv, _ := e.do("POST", "/v1/invoices", fin, map[string]any{"tenant_id": ta, "title": "Support " + sfx, "lines": lines(1_000_000)})
	id := inv["id"].(string)
	e.do("POST", "/v1/invoices/"+id+"/issue", fin, nil)
	e.do("POST", "/v1/invoices/"+id+"/payments", fin, map[string]any{"amount": 300000, "method": "momo", "reference": "TX" + sfx})
	_, cn, _ := e.do("POST", "/v1/invoices/"+id+"/credit-notes", fin, map[string]any{"net": 100000, "reason": "Goodwill"})
	cnRef := cn["credit_note_ref"].(string)

	_, list, _ := e.do("GET", "/v1/invoices/"+id+"/payments", fin, nil)
	items := list["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("want 1 payment, got %v", list)
	}
	p := items[0].(map[string]any)
	rct := p["receipt_ref"].(string)
	if !strings.Contains(rct, "-RCT-") {
		t.Fatalf("receipt ref: %v", rct)
	}

	resp, a := e.pdf(fin, "/v1/payments/"+p["id"].(string)+"/receipt")
	if resp.StatusCode != 200 || !bytes.Contains(a, []byte(rct)) || !bytes.Contains(a, []byte("300 000 XAF")) || !bytes.Contains(a, []byte("TX"+sfx)) {
		t.Fatalf("receipt: %d", resp.StatusCode)
	}
	_, b := e.pdf(alice, "/v1/payments/"+p["id"].(string)+"/receipt")
	if !bytes.Equal(a, b) {
		t.Error("receipt must be one stable file")
	}
	if r, _ := e.pdf(bob, "/v1/payments/"+p["id"].(string)+"/receipt"); r.StatusCode != 404 {
		t.Errorf("another client: want 404, got %d", r.StatusCode)
	}

	resp, c := e.pdf(fin, "/v1/credit-notes/"+cnRef+"/pdf")
	if resp.StatusCode != 200 || !bytes.Contains(c, []byte(cnRef)) || !bytes.Contains(c, []byte("Goodwill")) {
		t.Fatalf("credit note pdf: %d", resp.StatusCode)
	}
	if r, _ := e.pdf(bob, "/v1/credit-notes/"+cnRef+"/pdf"); r.StatusCode != 404 {
		t.Errorf("another client: want 404, got %d", r.StatusCode)
	}
}

func TestPurchasingAndAssetPDFs(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	buyer := token("pdf-buyer", "", "procurement")
	log := token("pdf-log", "", "logistics")
	sa := mkSupplier(t, e, buyer, "PDF Supplier A "+sfx)
	sb := mkSupplier(t, e, buyer, "PDF Supplier B "+sfx)
	supA := token("pdf-sup-a", sa, "supplier")
	supB := token("pdf-sup-b", sb, "supplier")

	// RFQ
	_, rfq, _ := e.do("POST", "/v1/rfqs", buyer, map[string]any{"title": "Routers " + sfx, "suppliers": []string{sa},
		"lines": []map[string]any{{"description": "Edge router " + sfx, "quantity": 3}}})
	rid := rfq["id"].(string)
	if r, _ := e.pdf(buyer, "/v1/rfqs/"+rid+"/pdf"); r.StatusCode != 409 {
		t.Errorf("draft RFQ: want 409, got %d", r.StatusCode)
	}
	_, opened, _ := e.do("POST", "/v1/rfqs/"+rid+"/open", buyer, nil)
	resp, p := e.pdf(buyer, "/v1/rfqs/"+rid+"/pdf")
	if resp.StatusCode != 200 || !bytes.Contains(p, []byte(opened["ref"].(string))) || !bytes.Contains(p, []byte("Edge router "+sfx)) {
		t.Fatalf("rfq pdf: %d", resp.StatusCode)
	}
	if r, b := e.pdf(supA, "/v1/rfqs/"+rid+"/pdf"); r.StatusCode != 200 || !bytes.Equal(p, b) {
		t.Errorf("invited supplier must get the same file, got %d", r.StatusCode)
	}
	if r, _ := e.pdf(supB, "/v1/rfqs/"+rid+"/pdf"); r.StatusCode != 404 {
		t.Errorf("uninvited supplier: want 404, got %d", r.StatusCode)
	}

	// Purchase order
	_, po, _ := e.do("POST", "/v1/purchase-orders", buyer, map[string]any{"supplier_id": sa, "title": "Cables " + sfx,
		"lines": []map[string]any{{"description": "Cat6 cable", "quantity": 10, "unit_price": 5000}}})
	pid := po["id"].(string)
	if r, _ := e.pdf(buyer, "/v1/purchase-orders/"+pid+"/pdf"); r.StatusCode != 409 {
		t.Errorf("draft PO: want 409, got %d", r.StatusCode)
	}
	_, issued, _ := e.do("POST", "/v1/purchase-orders/"+pid+"/issue", buyer, nil)
	resp, p = e.pdf(buyer, "/v1/purchase-orders/"+pid+"/pdf")
	if resp.StatusCode != 200 || !bytes.Contains(p, []byte(issued["ref"].(string))) || !bytes.Contains(p, []byte("50 000 XAF")) || !bytes.Contains(p, []byte("PDF Supplier A "+sfx)) {
		t.Fatalf("po pdf: %d", resp.StatusCode)
	}
	if r, b := e.pdf(supA, "/v1/purchase-orders/"+pid+"/pdf"); r.StatusCode != 200 || !bytes.Equal(p, b) {
		t.Errorf("the supplier must get the same file, got %d", r.StatusCode)
	}
	if r, _ := e.pdf(supB, "/v1/purchase-orders/"+pid+"/pdf"); r.StatusCode != 404 {
		t.Errorf("another supplier: want 404, got %d", r.StatusCode)
	}

	// Asset issue form
	item := mkItem(t, e, log, "LAP-"+sfx, true)
	wh := mkWarehouse(t, e, log, "PDF store "+sfx)
	e.do("POST", "/v1/stock/receipts", log, map[string]any{"item_id": item, "warehouse_id": wh, "qty": 1, "serials": []string{"SN-" + sfx}})
	_, assets, _ := e.do("GET", "/v1/assets", log, nil)
	var sid string
	for _, a := range assets["data"].([]any) {
		if m := a.(map[string]any); m["serial"] == "SN-"+sfx {
			sid = m["id"].(string)
		}
	}
	_, iss, _ := e.do("POST", "/v1/assets/issue", log, map[string]any{"serial_id": sid, "assignee": "Eng " + sfx})
	fref := iss["document_ref"].(string)
	resp, p = e.pdf(log, "/v1/assets/forms/"+fref+"/pdf")
	if resp.StatusCode != 200 || !bytes.Contains(p, []byte(fref)) || !bytes.Contains(p, []byte("SN-"+sfx)) || !bytes.Contains(p, []byte("Eng "+sfx)) {
		t.Fatalf("asset form pdf: %d", resp.StatusCode)
	}
	if r, _ := e.pdf(log, "/v1/assets/forms/HS-FRM-38-2000-000001/pdf"); r.StatusCode != 404 {
		t.Errorf("unknown form: want 404, got %d", r.StatusCode)
	}
}

func TestNarrativeDocuments(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	sales := token("nar-sales", "", "sales")
	hr := token("nar-hr", "", "hr")
	ta := e.newClient(token("m", "", "manager"), "Narrative Co "+sfx)
	tb := e.newClient(token("m", "", "manager"), "Other Narrative Co "+sfx)
	alice := token("nar-alice", ta, "client_user")
	bob := token("nar-bob", tb, "client_user")
	body := map[string]any{"kind": "proposal", "tenant_id": ta, "title": "Network upgrade " + sfx, "signed_by": "A. Manager",
		"sections": []map[string]any{{"heading": "Scope", "body": "Replace the core switches " + sfx}, {"heading": "Timeline", "body": "Six weeks."}}}

	if c, _, _ := e.do("POST", "/v1/narratives", hr, body); c != 403 {
		t.Errorf("hr creating: want 403, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/narratives", sales, map[string]any{"kind": "memo", "tenant_id": ta, "title": "x", "sections": []map[string]any{{"body": "y"}}}); c != 422 {
		t.Errorf("unknown kind: want 422, got %d", c)
	}
	c, n, _ := e.do("POST", "/v1/narratives", sales, body)
	if c != 201 {
		t.Fatalf("create: %d %v", c, n)
	}
	id := n["id"].(string)
	if r, _ := e.pdf(sales, "/v1/narratives/"+id+"/pdf"); r.StatusCode != 409 {
		t.Errorf("draft pdf: want 409, got %d", r.StatusCode)
	}
	if c, _, _ := e.do("GET", "/v1/narratives/"+id, alice, nil); c != 404 {
		t.Errorf("client must not see a draft, got %d", c)
	}
	body["title"] = "Network upgrade v2 " + sfx
	if c, _, _ := e.do("PUT", "/v1/narratives/"+id, sales, body); c != 200 {
		t.Fatalf("edit draft: %d", c)
	}
	c, iss, _ := e.do("POST", "/v1/narratives/"+id+"/issue", sales, nil)
	if c != 200 || !strings.Contains(iss["ref"].(string), "-PRP-") {
		t.Fatalf("issue: %d %v", c, iss)
	}
	if c, _, _ := e.do("PUT", "/v1/narratives/"+id, sales, body); c != 409 {
		t.Errorf("edit issued: want 409, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/narratives/"+id+"/issue", sales, nil); c != 409 {
		t.Errorf("issue twice: want 409, got %d", c)
	}
	resp, a := e.pdf(sales, "/v1/narratives/"+id+"/pdf")
	for _, w := range []string{iss["ref"].(string), "PROPOSAL", "Network upgrade v2 " + sfx, "Replace the core switches " + sfx, "A. Manager", "Narrative Co " + sfx} {
		if !bytes.Contains(a, []byte(w)) {
			t.Errorf("proposal PDF must contain %q (status %d)", w, resp.StatusCode)
		}
	}
	if _, b := e.pdf(alice, "/v1/narratives/"+id+"/pdf"); !bytes.Equal(a, b) {
		t.Error("the client must get the same file")
	}
	if r, _ := e.pdf(bob, "/v1/narratives/"+id+"/pdf"); r.StatusCode != 404 {
		t.Errorf("another client: want 404, got %d", r.StatusCode)
	}
	_, lst, _ := e.do("GET", "/v1/narratives", bob, nil)
	for _, x := range lst["data"].([]any) {
		if x.(map[string]any)["id"] == id {
			t.Error("another client's list must not include the document")
		}
	}
}
