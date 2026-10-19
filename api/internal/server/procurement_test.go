package server

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func mkSupplier(t *testing.T, e *env, buyer, name string) string {
	code, b, _ := e.do("POST", "/v1/suppliers", buyer, map[string]any{"legal_name": name})
	if code != 201 {
		t.Fatalf("supplier: %d %v", code, b)
	}
	return b["tenant_id"].(string)
}

func TestRFQToPaidInvoice(t *testing.T) {
	e := setup(t)
	buyer := token("buyer-1", "", "procurement")
	mgr := token("proc-mgr", "", "manager")
	mgr2 := token("proc-mgr2", "", "ceo")
	fin := token("fin-pay", "", "finance")
	fin2 := token("fin-pay2", "", "finance")
	sfx := fmt.Sprint(time.Now().UnixNano())
	sa := mkSupplier(t, e, buyer, "Supplier A "+sfx)
	sb := mkSupplier(t, e, buyer, "Supplier B "+sfx)
	sc := mkSupplier(t, e, buyer, "Supplier C "+sfx)
	supA := token("sup-a", sa, "supplier")
	supB := token("sup-b", sb, "supplier")
	supC := token("sup-c", sc, "supplier")
	before := balances(t, e, fin)

	// RFQ: two lines, A and B invited, C not.
	code, rfq, _ := e.do("POST", "/v1/rfqs", buyer, map[string]any{"title": "Switches " + sfx, "suppliers": []string{sa, sb},
		"lines": []map[string]any{{"description": "24-port switch", "quantity": 4}, {"description": "Patch cables", "quantity": 20}}})
	if code != 201 {
		t.Fatalf("rfq: %d %v", code, rfq)
	}
	rid := rfq["id"].(string)
	if c, _, _ := e.do("GET", "/v1/rfqs/"+rid, supA, nil); c != 404 {
		t.Errorf("a draft RFQ must be invisible to suppliers, got %d", c)
	}
	if c, o, _ := e.do("POST", "/v1/rfqs/"+rid+"/open", buyer, nil); c != 200 || !strings.HasPrefix(o["ref"].(string), "HS-RFQ-") {
		t.Fatalf("open: %d %v", c, o)
	}
	if c, _, _ := e.do("GET", "/v1/rfqs/"+rid, supC, nil); c != 404 {
		t.Errorf("an uninvited supplier must not see the RFQ, got %d", c)
	}
	respond := func(tok string, p1, p2 int64) int {
		c, _, _ := e.do("POST", "/v1/rfqs/"+rid+"/respond", tok, map[string]any{"unit_prices": []map[string]any{{"position": 1, "unit_price": p1}, {"position": 2, "unit_price": p2}}})
		return c
	}
	if c := respond(supC, 1, 1); c != 404 {
		t.Errorf("uninvited response: want 404, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/rfqs/"+rid+"/respond", supA, map[string]any{"unit_prices": []map[string]any{{"position": 1, "unit_price": 100}}}); c != 422 {
		t.Errorf("missing a line price: want 422, got %d", c)
	}
	if c := respond(supA, 1_000_000, 5_000); c != 201 { // 4,000,000 + 100,000
		t.Fatalf("A responds: got %d", c)
	}
	if c := respond(supB, 1_200_000, 4_000); c != 201 {
		t.Fatalf("B responds: got %d", c)
	}
	// A sees only their own bid; the buyer sees both.
	_, asA, _ := e.do("GET", "/v1/rfqs/"+rid, supA, nil)
	if n := len(asA["responses"].([]any)); n != 1 {
		t.Errorf("supplier A must see only their own response, saw %d", n)
	}
	_, asBuyer, _ := e.do("GET", "/v1/rfqs/"+rid, buyer, nil)
	if n := len(asBuyer["responses"].([]any)); n != 2 {
		t.Errorf("buyer sees both responses, saw %d", n)
	}
	if c, _, _ := e.do("POST", "/v1/rfqs/"+rid+"/award", buyer, map[string]any{"supplier": sa}); c != 409 {
		t.Errorf("award before close: want 409, got %d", c)
	}
	e.do("POST", "/v1/rfqs/"+rid+"/close", buyer, nil)
	if c := respond(supA, 1, 1); c != 404 && c != 409 {
		t.Errorf("response after close: want 404/409, got %d", c)
	}
	code, aw, _ := e.do("POST", "/v1/rfqs/"+rid+"/award", buyer, map[string]any{"supplier": sa})
	if code != 201 {
		t.Fatalf("award: %d %v", code, aw)
	}
	po := aw["purchase_order_id"].(string)
	_, pod, _ := e.do("GET", "/v1/purchase-orders/"+po, buyer, nil)
	if pod["net"] != float64(4_100_000) || pod["status"] != "draft" || pod["supplier"] != sa {
		t.Fatalf("PO from award: %v", pod)
	}
	if c, _, _ := e.do("GET", "/v1/purchase-orders/"+po, supA, nil); c != 404 {
		t.Errorf("draft PO must be invisible to the supplier, got %d", c)
	}
	// Under the threshold: issued directly.
	if c, o, _ := e.do("POST", "/v1/purchase-orders/"+po+"/issue", buyer, nil); c != 200 || o["status"] != "issued" || !strings.HasPrefix(o["ref"].(string), "HS-PO-") {
		t.Fatalf("issue: %d %v", c, o)
	}
	if c, _, _ := e.do("GET", "/v1/purchase-orders/"+po, supA, nil); c != 200 {
		t.Errorf("supplier sees their issued PO, got %d", c)
	}
	if c, _, _ := e.do("GET", "/v1/purchase-orders/"+po, supB, nil); c != 404 {
		t.Errorf("another supplier must not see it, got %d", c)
	}

	// Receipt: stock-linked line goes into inventory.
	log := token("log-po", "", "logistics")
	item := mkItem(t, e, log, "SW-"+sfx, false)
	wh := mkWarehouse(t, e, log, "PO store "+sfx)
	_, pol, _ := e.do("POST", "/v1/purchase-orders", buyer, map[string]any{"supplier_id": sa, "title": "Switches direct " + sfx,
		"lines": []map[string]any{{"description": "switch", "quantity": 4, "unit_price": 1_000_000, "item_id": item}, {"description": "service", "quantity": 1, "unit_price": 100_000}}})
	po2 := pol["id"].(string)
	e.do("POST", "/v1/purchase-orders/"+po2+"/issue", buyer, nil)
	if c, _, _ := e.do("POST", "/v1/purchase-orders/"+po2+"/receive", buyer, map[string]any{"lines": []map[string]any{{"position": 1, "qty": 5, "warehouse_id": wh}}}); c != 422 {
		t.Errorf("over-receipt: want 422, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/purchase-orders/"+po2+"/receive", buyer, map[string]any{"lines": []map[string]any{{"position": 1, "qty": 2}}}); c != 422 {
		t.Errorf("stock line without a warehouse: want 422, got %d", c)
	}
	// Invoice before goods are received cannot pass the match.
	_, early, _ := e.do("POST", "/v1/purchase-orders/"+po2+"/invoices", supA, map[string]any{"number": "A-EARLY-" + sfx, "net": 4_100_000, "vat": 0})
	if c, _, _ := e.do("POST", "/v1/supplier-invoices/"+early["id"].(string)+"/approve", fin, nil); c != 409 {
		t.Errorf("approve before receipt: want 409, got %d", c)
	}
	e.do("POST", "/v1/purchase-orders/"+po2+"/receive", buyer, map[string]any{"lines": []map[string]any{{"position": 1, "qty": 4, "warehouse_id": wh}, {"position": 2, "qty": 1}}})
	if got := stockOf(e, log, item); got != 4 {
		t.Errorf("received goods must reach stock, got %v", got)
	}
	_, got, _ := e.do("GET", "/v1/purchase-orders/"+po2, buyer, nil)
	if got["status"] != "received" {
		t.Fatalf("fully received PO should be 'received': %v", got["status"])
	}

	// Supplier invoice and the three-way match.
	if c, _, _ := e.do("POST", "/v1/purchase-orders/"+po2+"/invoices", supB, map[string]any{"number": "B-1", "net": 1, "vat": 0}); c != 404 {
		t.Errorf("another supplier cannot invoice this PO, got %d", c)
	}
	inv := map[string]any{"number": "A-" + sfx, "net": 4_100_000, "vat": 789_250}
	code, si, _ := e.do("POST", "/v1/purchase-orders/"+po2+"/invoices", supA, inv)
	if code != 201 {
		t.Fatalf("submit invoice: %d %v", code, si)
	}
	if c, _, _ := e.do("POST", "/v1/purchase-orders/"+po2+"/invoices", supA, map[string]any{"number": "A-" + sfx, "net": 1, "vat": 0}); c != 409 {
		t.Errorf("duplicate invoice number: want 409, got %d", c)
	}
	sid := si["id"].(string)
	// Over-invoicing against the PO is refused.
	_, big, _ := e.do("POST", "/v1/purchase-orders/"+po2+"/invoices", supA, map[string]any{"number": "A-BIG-" + sfx, "net": 9_000_000, "vat": 0})
	if c, _, _ := e.do("POST", "/v1/supplier-invoices/"+big["id"].(string)+"/approve", fin, nil); c != 422 {
		t.Errorf("invoice above the PO: want 422, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/supplier-invoices/"+sid+"/approve", supA, nil); c != 403 {
		t.Errorf("supplier cannot approve their own invoice, got %d", c)
	}
	if c, o, _ := e.do("POST", "/v1/supplier-invoices/"+sid+"/approve", fin, nil); c != 200 || o["status"] != "approved" {
		t.Fatalf("approve: %d %v", c, o)
	}
	mid := balances(t, e, fin)
	if mid["601"]-before["601"] != 4_100_000 || mid["4452"]-before["4452"] != 789_250 || mid["401"]-before["401"] != -4_889_250 {
		t.Errorf("approval must book purchases, VAT and payable: 601=%v 4452=%v 401=%v", mid["601"]-before["601"], mid["4452"]-before["4452"], mid["401"]-before["401"])
	}

	// Payment: needs verified bank details and a second person.
	if c, _, _ := e.do("POST", "/v1/supplier-invoices/"+sid+"/pay", fin2, map[string]any{"method": "bank"}); c != 409 {
		t.Errorf("pay with no bank details: want 409, got %d", c)
	}
	if c, _, _ := e.do("PUT", "/v1/suppliers/me/bank", supA, map[string]any{"account": "123"}); c != 422 {
		t.Errorf("short account: want 422, got %d", c)
	}
	if c, b, _ := e.do("PUT", "/v1/suppliers/me/bank", supA, map[string]any{"account": "CM21 1000 2000 3000 4000"}); c != 200 || b["bank_status"] != "pending_callback" || b["bank_account_last4"] != "4000" {
		t.Fatalf("bank details: %d %v", c, b)
	}
	if c, _, _ := e.do("POST", "/v1/supplier-invoices/"+sid+"/pay", fin2, map[string]any{"method": "bank"}); c != 409 {
		t.Errorf("pay before call-back verification: want 409, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/suppliers/"+sa+"/bank/verify", supA, map[string]any{"note": "I verified myself"}); c != 403 {
		t.Errorf("a supplier cannot verify their own bank details, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/suppliers/"+sa+"/bank/verify", fin, map[string]any{"note": "x"}); c != 422 {
		t.Errorf("verification needs a call-back note, got %d", c)
	}
	if c, b, _ := e.do("POST", "/v1/suppliers/"+sa+"/bank/verify", fin, map[string]any{"note": "Called finance office on +237 600 000 000, spoke to J. Doe"}); c != 200 || b["bank_status"] != "verified" {
		t.Fatalf("verify: %d %v", c, b)
	}
	if c, _, _ := e.do("POST", "/v1/supplier-invoices/"+sid+"/pay", fin, map[string]any{"method": "bank"}); c != 403 {
		t.Errorf("approver cannot pay: want 403, got %d", c)
	}
	if c, o, _ := e.do("POST", "/v1/supplier-invoices/"+sid+"/pay", fin2, map[string]any{"method": "bank"}); c != 200 || o["status"] != "paid" {
		t.Fatalf("pay: %d %v", c, o)
	}
	end := balances(t, e, fin)
	if end["401"]-before["401"] != 0 || end["521"]-before["521"] != -4_889_250 {
		t.Errorf("payment must clear the payable and credit the bank: 401=%v 521=%v", end["401"]-before["401"], end["521"]-before["521"])
	}
	// Changing the account resets verification.
	if _, b, _ := e.do("PUT", "/v1/suppliers/me/bank", supA, map[string]any{"account": "CM21 9999 8888 7777 6666"}); b["bank_status"] != "pending_callback" {
		t.Errorf("changed bank details must need re-verification: %v", b)
	}

	// Dual control on large purchase orders.
	_, bigpo, _ := e.do("POST", "/v1/purchase-orders", buyer, map[string]any{"supplier_id": sa, "title": "Big " + sfx, "lines": []map[string]any{{"description": "x", "quantity": 1, "unit_price": 6_000_000}}})
	bp := bigpo["id"].(string)
	if c, o, _ := e.do("POST", "/v1/purchase-orders/"+bp+"/issue", buyer, nil); c != 202 || o["status"] != "pending_approval" {
		t.Fatalf("large PO should wait for approval: %d %v", c, o)
	}
	if c, _, _ := e.do("POST", "/v1/purchase-orders/"+bp+"/approve", buyer, nil); c != 403 {
		t.Errorf("a buyer cannot approve (not an approver role), got %d", c)
	}
	_, ownpo, _ := e.do("POST", "/v1/purchase-orders", mgr, map[string]any{"supplier_id": sa, "title": "Own " + sfx, "lines": []map[string]any{{"description": "x", "quantity": 1, "unit_price": 6_000_000}}})
	e.do("POST", "/v1/purchase-orders/"+ownpo["id"].(string)+"/issue", mgr, nil)
	if c, _, _ := e.do("POST", "/v1/purchase-orders/"+ownpo["id"].(string)+"/approve", mgr, nil); c != 403 {
		t.Errorf("self approval of a PO: want 403, got %d", c)
	}
	if c, o, _ := e.do("POST", "/v1/purchase-orders/"+bp+"/approve", mgr2, nil); c != 200 || o["status"] != "issued" || o["approved_by"] != "proc-mgr2" {
		t.Errorf("second approver: %d %v", c, o)
	}
}

func TestBankDetailsCannotBeSelfVerifiedInTheDatabase(t *testing.T) {
	e := setup(t)
	buyer := token("buyer-db", "", "procurement")
	sup := mkSupplier(t, e, buyer, "Fraud Co "+fmt.Sprint(time.Now().UnixNano()))
	e.do("PUT", "/v1/suppliers/me/bank", token("fraud", sup, "supplier"), map[string]any{"account": "CM00 0000 0000 0000"})

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("HEKATE_TEST_DB"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, _ := conn.Begin(ctx)
	defer tx.Rollback(ctx)
	// Act exactly as the app does for a supplier user, but go around the API.
	_, _ = tx.Exec(ctx, `SET LOCAL ROLE hekate_app`)
	_, _ = tx.Exec(ctx, `SELECT set_config('app.role','other',true), set_config('app.roles','supplier',true), set_config('app.tenant_id',$1,true), set_config('app.actor','fraud',true)`, sup)
	_, err = tx.Exec(ctx, `UPDATE procurement.suppliers SET bank_status='verified' WHERE tenant_id=$1`, sup)
	if err == nil || !strings.Contains(err.Error(), "only finance or management") {
		t.Errorf("a supplier must not be able to verify their own bank details, got %v", err)
	}
}
