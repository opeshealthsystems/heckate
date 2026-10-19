package server

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func lines(price int64) []map[string]any {
	return []map[string]any{{"description": "Penetration test", "quantity": 1, "unit_price": price}}
}

func TestQuoteToInvoiceWorkflow(t *testing.T) {
	e := setup(t)
	sales := token("sales-1", "", "sales")
	staff := token("mgr-q", "", "manager")
	ta := e.newClient(staff, "QuoteCo "+t.Name())
	tb := e.newClient(staff, "OtherCo "+t.Name())
	adminA := token("adminA", ta, "client_admin")
	userA := token("userA", ta, "client_user")
	adminB := token("adminB", tb, "client_admin")

	code, q, _ := e.do("POST", "/v1/quotes", sales, map[string]any{"tenant_id": ta, "title": "Pentest 2026", "lines": lines(1_000_000)})
	if code != 201 || q["total"] != float64(1_192_500) || q["status"] != "draft" {
		t.Fatalf("create quote: %d %v", code, q)
	}
	id := q["id"].(string)
	if code, _, _ := e.do("GET", "/v1/quotes/"+id, adminA, nil); code != 404 {
		t.Errorf("client must not see a draft quote, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/quotes/"+id+"/approve", adminA, nil); code != 404 && code != 409 {
		t.Errorf("approve a draft: want 404/409, got %d", code)
	}
	code, sent, _ := e.do("POST", "/v1/quotes/"+id+"/send", sales, nil)
	ref, _ := sent["ref"].(string)
	if code != 200 || !strings.HasPrefix(ref, "HS-QT-") {
		t.Fatalf("send: %d %v", code, sent)
	}
	if code, _, _ := e.do("PUT", "/v1/quotes/"+id+"/lines", sales, map[string]any{"lines": lines(1)}); code != 409 {
		t.Errorf("edit sent quote: want 409, got %d", code)
	}
	if code, _, _ := e.do("GET", "/v1/quotes/"+id, adminA, nil); code != 200 {
		t.Errorf("client sees sent quote, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/quotes/"+id+"/approve", userA, nil); code != 403 {
		t.Errorf("client_user approve: want 403, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/quotes/"+id+"/approve", adminB, nil); code != 404 {
		t.Errorf("other tenant admin approve: want 404, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/quotes/"+id+"/approve", sales, nil); code != 403 {
		t.Errorf("staff approve: want 403, got %d", code)
	}
	if code, b, _ := e.do("POST", "/v1/quotes/"+id+"/invoice", sales, nil); code != 409 {
		t.Errorf("invoice before approval: want 409, got %d %v", code, b)
	}
	if code, b, _ := e.do("POST", "/v1/quotes/"+id+"/approve", adminA, nil); code != 200 || b["status"] != "approved" {
		t.Fatalf("approve: %d %v", code, b)
	}
	code, inv, _ := e.do("POST", "/v1/quotes/"+id+"/invoice", sales, nil)
	if code != 201 || inv["total"] != float64(1_192_500) || inv["status"] != "draft" {
		t.Fatalf("invoice from quote: %d %v", code, inv)
	}
	if code, _, _ := e.do("POST", "/v1/quotes/"+id+"/invoice", sales, nil); code != 409 {
		t.Errorf("second invoice for quote: want 409, got %d", code)
	}
}

func TestInvoiceDualControlPaymentsAndCredit(t *testing.T) {
	e := setup(t)
	fin := token("fin-1", "", "finance")
	mgr := token("mgr-1", "", "manager")
	mgr2 := token("mgr-2", "", "ceo")
	ta := e.newClient(mgr, "PayCo "+t.Name())
	tb := e.newClient(mgr, "NotPayCo "+t.Name())
	clientA := token("ca", ta, "client_user")
	clientB := token("cb", tb, "client_user")

	// Below the threshold: issued immediately.
	_, small, _ := e.do("POST", "/v1/invoices", fin, map[string]any{"tenant_id": ta, "title": "Small", "lines": lines(100_000)})
	if code, b, _ := e.do("POST", "/v1/invoices/"+small["id"].(string)+"/issue", fin, nil); code != 200 || b["status"] != "issued" || b["ref"] == nil {
		t.Fatalf("small issue: %d %v", code, b)
	}

	// At or above the threshold: needs a second person.
	_, big, _ := e.do("POST", "/v1/invoices", fin, map[string]any{"tenant_id": ta, "title": "Big", "lines": lines(6_000_000)})
	bid := big["id"].(string)
	if code, _, _ := e.do("GET", "/v1/invoices/"+bid, clientA, nil); code != 404 {
		t.Errorf("client must not see a draft invoice, got %d", code)
	}
	if code, b, _ := e.do("POST", "/v1/invoices/"+bid+"/issue", fin, nil); code != 202 || b["status"] != "pending_approval" {
		t.Fatalf("big issue should wait for approval: %d %v", code, b)
	}
	if code, _, _ := e.do("POST", "/v1/invoices/"+bid+"/approve", fin, nil); code != 403 {
		t.Errorf("finance cannot approve (not an approver role), got %d", code)
	}
	// A manager who prepared the invoice cannot approve their own.
	_, own, _ := e.do("POST", "/v1/invoices", mgr, map[string]any{"tenant_id": ta, "title": "Own", "lines": lines(6_000_000)})
	e.do("POST", "/v1/invoices/"+own["id"].(string)+"/issue", mgr, nil)
	if code, _, _ := e.do("POST", "/v1/invoices/"+own["id"].(string)+"/approve", mgr, nil); code != 403 {
		t.Errorf("self approval: want 403, got %d", code)
	}
	code, issued, _ := e.do("POST", "/v1/invoices/"+bid+"/approve", mgr, nil)
	if code != 200 || issued["status"] != "issued" || issued["approved_by"] != "mgr-1" || !strings.HasPrefix(issued["ref"].(string), "HS-INV-") {
		t.Fatalf("approve: %d %v", code, issued)
	}
	_ = mgr2
	total := int64(issued["total"].(float64))
	if total != 7_155_000 {
		t.Errorf("6,000,000 net must total 7,155,000, got %d", total)
	}

	// Client sees it now, but not the other tenant.
	if code, _, _ := e.do("GET", "/v1/invoices/"+bid, clientA, nil); code != 200 {
		t.Errorf("client sees issued invoice, got %d", code)
	}
	if code, _, _ := e.do("GET", "/v1/invoices/"+bid, clientB, nil); code != 404 {
		t.Errorf("other tenant must not see it, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/invoices/"+bid+"/payments", clientA, map[string]any{"amount": 1, "method": "momo"}); code != 403 {
		t.Errorf("client cannot record payments, got %d", code)
	}

	// Payments.
	if code, _, _ := e.do("POST", "/v1/invoices/"+bid+"/payments", fin, map[string]any{"amount": 1, "method": "crypto"}); code != 422 {
		t.Errorf("bad method: want 422, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/invoices/"+bid+"/payments", fin, map[string]any{"amount": total + 1, "method": "bank"}); code != 422 {
		t.Errorf("overpayment: want 422, got %d", code)
	}
	if code, b, _ := e.do("POST", "/v1/invoices/"+bid+"/payments", fin, map[string]any{"amount": 3_000_000, "method": "momo", "reference": "MP123"}); code != 201 || b["status"] != "issued" || b["balance"] != float64(total-3_000_000) {
		t.Fatalf("partial payment: %d %v", code, b)
	}
	// Credit note for part of it, then the rest is paid.
	if code, _, _ := e.do("POST", "/v1/invoices/"+bid+"/credit-notes", fin, map[string]any{"net": 10_000_000, "reason": "too much"}); code != 422 {
		t.Errorf("excess credit: want 422, got %d", code)
	}
	code, cn, _ := e.do("POST", "/v1/invoices/"+bid+"/credit-notes", fin, map[string]any{"net": 1_000_000, "reason": "Scope reduced"})
	if code != 201 || !strings.HasPrefix(cn["credit_note_ref"].(string), "HS-CN-") || cn["total"] != float64(1_192_500) {
		t.Fatalf("credit note: %d %v", code, cn)
	}
	remaining := total - 3_000_000 - 1_192_500
	if code, b, _ := e.do("POST", "/v1/invoices/"+bid+"/payments", fin, map[string]any{"amount": remaining, "method": "bank"}); code != 201 || b["status"] != "paid" || b["balance"] != float64(0) {
		t.Fatalf("final payment should settle the invoice: %d %v", code, b)
	}
	if code, _, _ := e.do("POST", "/v1/invoices/"+bid+"/payments", fin, map[string]any{"amount": 1, "method": "bank"}); code != 409 {
		t.Errorf("payment on paid invoice: want 409, got %d", code)
	}

	// Report (finance sees it, client does not).
	if code, b, _ := e.do("GET", "/v1/reports/sales", fin, nil); code != 200 || b["collected"].(float64) < 3_000_000 {
		t.Errorf("report: %d %v", code, b)
	}
	if code, _, _ := e.do("GET", "/v1/reports/sales", clientA, nil); code != 403 {
		t.Errorf("client report: want 403, got %d", code)
	}
}

func TestIssuedInvoicesAreImmutableInTheDatabase(t *testing.T) {
	e := setup(t)
	fin := token("fin-2", "", "finance")
	tid := e.newClient(token("m", "", "manager"), "Immutable "+t.Name())
	_, inv, _ := e.do("POST", "/v1/invoices", fin, map[string]any{"tenant_id": tid, "title": "Frozen", "lines": lines(50_000)})
	id := inv["id"].(string)
	e.do("POST", "/v1/invoices/"+id+"/issue", fin, nil)

	// Even the owning database role (no RLS, no SET ROLE) is stopped by the triggers.
	conn, err := pgx.Connect(context.Background(), adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	for name, sql := range map[string]string{
		"change total":  `UPDATE sales.invoices SET total = 1 WHERE id = $1`,
		"change ref":    `UPDATE sales.invoices SET ref = 'HS-INV-2026-999999' WHERE id = $1`,
		"delete":        `DELETE FROM sales.invoices WHERE id = $1`,
		"edit line":     `UPDATE sales.invoice_lines SET unit_price = 1 WHERE invoice_id = $1`,
		"delete line":   `DELETE FROM sales.invoice_lines WHERE invoice_id = $1`,
		"add line":      `INSERT INTO sales.invoice_lines (invoice_id, tenant_id, position, description, quantity, unit_price) SELECT id, tenant_id, 9, 'x', 1, 1 FROM sales.invoices WHERE id = $1`,
		"back to draft": `UPDATE sales.invoices SET status = 'draft' WHERE id = $1`,
	} {
		if _, err := conn.Exec(context.Background(), sql, id); err == nil {
			t.Errorf("%s: must be rejected on an issued invoice", name)
		} else if !strings.Contains(err.Error(), "immutable") && !strings.Contains(err.Error(), "cannot") {
			t.Errorf("%s: unexpected error %v", name, err)
		}
	}
}
