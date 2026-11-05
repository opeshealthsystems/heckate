package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func mkItem(t *testing.T, e *env, tok, sku string, serialised bool) string {
	code, b, _ := e.do("POST", "/v1/items", tok, map[string]any{"sku": sku, "name": "Item " + sku, "serialised": serialised, "min_stock": 2})
	if code != 201 {
		t.Fatalf("item: %d %v", code, b)
	}
	return b["id"].(string)
}

func mkWarehouse(t *testing.T, e *env, tok, name string) string {
	code, b, _ := e.do("POST", "/v1/warehouses", tok, map[string]any{"name": name})
	if code != 201 {
		t.Fatalf("warehouse: %d %v", code, b)
	}
	return b["id"].(string)
}

func stockOf(e *env, tok, item string) float64 {
	_, l, _ := e.do("GET", "/v1/stock", tok, nil)
	var n float64
	for _, r := range l["data"].([]any) {
		m := r.(map[string]any)
		if m["item_id"] == item {
			n += m["qty"].(float64)
		}
	}
	return n
}

func TestStockLedger(t *testing.T) {
	e := setup(t)
	log := token("log-1", "", "logistics")
	mgr := token("mgr-i", "", "manager")
	sfx := fmt.Sprint(time.Now().UnixNano())
	item := mkItem(t, e, log, "CBL-"+sfx, false)
	wh1 := mkWarehouse(t, e, log, "Main "+sfx)
	wh2 := mkWarehouse(t, e, log, "Van "+sfx)

	if c, _, _ := e.do("POST", "/v1/items", token("eng", "", "engineer"), map[string]any{"sku": "x", "name": "x"}); c != 403 {
		t.Errorf("engineer cannot create items, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/stock/receipts", log, map[string]any{"item_id": item, "warehouse_id": wh1, "qty": 0}); c != 422 {
		t.Errorf("zero receipt: want 422, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/stock/receipts", log, map[string]any{"item_id": item, "warehouse_id": wh1, "qty": 10, "serials": []string{"a"}}); c != 422 {
		t.Errorf("serials on a non-serialised item: want 422, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/stock/receipts", log, map[string]any{"item_id": item, "warehouse_id": wh1, "qty": 10, "ref": "PO-1"}); c != 201 {
		t.Fatalf("receipt: want 201, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/stock/issues", log, map[string]any{"item_id": item, "warehouse_id": wh1, "qty": 11}); c != 409 {
		t.Errorf("issuing more than on hand: want 409, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/stock/issues", log, map[string]any{"item_id": item, "warehouse_id": wh1, "qty": 4}); c != 201 {
		t.Errorf("issue: want 201, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/stock/transfers", log, map[string]any{"item_id": item, "from_warehouse_id": wh1, "to_warehouse_id": wh2, "qty": 5}); c != 201 {
		t.Errorf("transfer: want 201, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/stock/transfers", log, map[string]any{"item_id": item, "from_warehouse_id": wh1, "to_warehouse_id": wh2, "qty": 5}); c != 409 {
		t.Errorf("transfer beyond stock: want 409, got %d", c)
	}
	if got := stockOf(e, log, item); got != 6 {
		t.Errorf("on hand = %v, want 6", got)
	}
	// Corrections are for managers and need a reason.
	if c, _, _ := e.do("POST", "/v1/stock/adjustments", log, map[string]any{"item_id": item, "warehouse_id": wh2, "qty": -1, "reason": "damaged"}); c != 403 {
		t.Errorf("logistics adjustment: want 403, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/stock/adjustments", mgr, map[string]any{"item_id": item, "warehouse_id": wh2, "qty": -1}); c != 422 {
		t.Errorf("adjustment without reason: want 422, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/stock/adjustments", mgr, map[string]any{"item_id": item, "warehouse_id": wh2, "qty": -1, "reason": "damaged"}); c != 201 {
		t.Errorf("adjustment: want 201, got %d", c)
	}
	_, items, _ := e.do("GET", "/v1/items", log, nil)
	for _, x := range items["data"].([]any) {
		m := x.(map[string]any)
		if m["id"] == item && (m["on_hand"] != float64(5) || m["low"] != false) {
			t.Errorf("item view wrong: %v", m)
		}
	}
	// Movements are append-only, and a concurrent race cannot drive stock negative.
	conn, err := pgx.Connect(context.Background(), adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(context.Background(), `DELETE FROM inventory.moves`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Errorf("moves must be append-only, got %v", err)
	}
	var wg sync.WaitGroup
	codes := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, _, _ := e.do("POST", "/v1/stock/issues", log, map[string]any{"item_id": item, "warehouse_id": wh2, "qty": 1})
			codes <- c
		}()
	}
	wg.Wait()
	close(codes)
	ok := 0
	for c := range codes {
		if c == 201 {
			ok++
		}
	}
	if ok != 4 { // wh2 holds 5-1 = 4
		t.Errorf("exactly 4 of 8 racing issues may succeed, got %d", ok)
	}
	// wh2 is drained; wh1 still holds the 1 unit left after the transfer.
	if got := stockOf(e, log, item); got != 1 {
		t.Errorf("on hand must end at 1 (all of the van, none negative), got %v", got)
	}
}

func TestAssetsAndClientEquipment(t *testing.T) {
	e := setup(t)
	log := token("log-a", "", "logistics")
	mgr := token("mgr-a", "", "manager")
	sfx := fmt.Sprint(time.Now().UnixNano())
	item := mkItem(t, e, log, "FW-"+sfx, true)
	wh := mkWarehouse(t, e, log, "Cage "+sfx)
	tid := e.newClient(mgr, "AssetCo "+t.Name())
	other := e.newClient(mgr, "OtherAssetCo "+t.Name())

	if c, _, _ := e.do("POST", "/v1/stock/receipts", log, map[string]any{"item_id": item, "warehouse_id": wh, "qty": 2, "serials": []string{"S1-" + sfx}}); c != 422 {
		t.Errorf("serial count mismatch: want 422, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/stock/receipts", log, map[string]any{"item_id": item, "warehouse_id": wh, "qty": 2, "serials": []string{"S1-" + sfx, "S2-" + sfx}}); c != 201 {
		t.Fatalf("serialised receipt: want 201, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/stock/receipts", log, map[string]any{"item_id": item, "warehouse_id": wh, "qty": 1, "serials": []string{"S1-" + sfx}}); c != 409 {
		t.Errorf("duplicate serial: want 409, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/stock/issues", log, map[string]any{"item_id": item, "warehouse_id": wh, "qty": 1}); c != 422 {
		t.Errorf("bulk issue of serialised item: want 422, got %d", c)
	}
	_, assets, _ := e.do("GET", "/v1/assets", log, nil)
	ids := map[string]string{}
	for _, a := range assets["data"].([]any) {
		m := a.(map[string]any)
		ids[m["serial"].(string)] = m["id"].(string)
	}
	s1, s2 := ids["S1-"+sfx], ids["S2-"+sfx]

	code, iss, _ := e.do("POST", "/v1/assets/issue", log, map[string]any{"serial_id": s1, "assignee": "eng-7"})
	if code != 200 || !strings.HasPrefix(iss["document_ref"].(string), "HS-FRM-38-") {
		t.Fatalf("issue asset: %d %v", code, iss)
	}
	if c, _, _ := e.do("POST", "/v1/assets/issue", log, map[string]any{"serial_id": s1, "assignee": "eng-8"}); c != 409 {
		t.Errorf("issuing an issued asset: want 409, got %d", c)
	}
	if got := stockOf(e, log, item); got != 1 {
		t.Errorf("one unit left in stock, got %v", got)
	}
	if c, _, _ := e.do("POST", "/v1/assets/return", log, map[string]any{"serial_id": s1, "warehouse_id": wh}); c != 200 {
		t.Errorf("return: want 200, got %d", c)
	}
	if got := stockOf(e, log, item); got != 2 {
		t.Errorf("two units back in stock, got %v", got)
	}
	if c, _, _ := e.do("POST", "/v1/assets/install", log, map[string]any{"serial_id": s2, "tenant_id": tid}); c != 200 {
		t.Fatalf("install at client: want 200, got %d", c)
	}
	// The client sees only their own installed equipment.
	_, mine, _ := e.do("GET", "/v1/assets", token("ca", tid, "client_user"), nil)
	if n := len(mine["data"].([]any)); n != 1 || mine["data"].([]any)[0].(map[string]any)["serial"] != "S2-"+sfx {
		t.Errorf("client should see exactly their asset, got %v", mine["data"])
	}
	_, theirs, _ := e.do("GET", "/v1/assets", token("cb", other, "client_user"), nil)
	if n := len(theirs["data"].([]any)); n != 0 {
		t.Errorf("another client must see no assets, got %d", n)
	}
	if c, _, _ := e.do("GET", "/v1/stock", token("ca", tid, "client_user"), nil); c != 403 {
		t.Errorf("client cannot see stock levels, got %d", c)
	}
}

func TestInterventionPartsComeOutOfStock(t *testing.T) {
	e := setup(t)
	log := token("log-p", "", "logistics")
	sfx := fmt.Sprint(time.Now().UnixNano())
	item := mkItem(t, e, log, "PSU-"+sfx, false)
	wh := mkWarehouse(t, e, log, "Van "+sfx)
	e.do("POST", "/v1/stock/receipts", log, map[string]any{"item_id": item, "warehouse_id": wh, "qty": 3})
	tid := e.newClient(token("m", "", "manager"), "PartsCo "+t.Name())
	eng := token("eng-parts", "", "engineer")

	_, iv, _ := e.do("POST", "/v1/interventions", token("d", "", "support"), map[string]any{"tenant_id": tid, "engineer": "eng-parts"})
	id := iv["id"].(string)
	e.do("POST", "/v1/interventions/"+id+"/check-in", eng, map[string]any{"lat": 4.0, "lng": 9.7})
	out := func(qty int) int {
		c, _, _ := e.do("POST", "/v1/interventions/"+id+"/check-out", eng, map[string]any{"lat": 4.0, "lng": 9.7, "report": "Replaced PSU and verified",
			"parts": []map[string]any{{"item_id": item, "warehouse_id": wh, "qty": qty}}})
		return c
	}
	if c := out(5); c != 409 {
		t.Errorf("using more parts than the van holds: want 409, got %d", c)
	}
	// The failed attempt must not have completed the intervention or touched stock.
	if got := stockOf(e, log, item); got != 3 {
		t.Errorf("failed check-out must not consume stock, got %v", got)
	}
	if c := out(2); c != 200 {
		t.Fatalf("check-out with parts: want 200, got %d", c)
	}
	if got := stockOf(e, log, item); got != 1 {
		t.Errorf("2 of 3 parts consumed, 1 left, got %v", got)
	}
	_, done, _ := e.do("GET", "/v1/interventions/"+id, token("m", "", "manager"), nil)
	if p, _ := done["parts"].([]any); len(p) != 1 {
		t.Errorf("parts used must be recorded on the report: %v", done["parts"])
	}
}

func TestStockCountsAndReorder(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	log := token("cnt-log", "", "logistics")
	mgr1 := token("cnt-mgr1", "", "manager")
	mgr2 := token("cnt-mgr2", "", "manager")
	item := mkItem(t, e, log, "CNT-"+sfx, false) // min_stock 2
	wh := mkWarehouse(t, e, log, "Count store "+sfx)
	if c, _, _ := e.do("POST", "/v1/stock/receipts", log, map[string]any{"item_id": item, "warehouse_id": wh, "qty": 10}); c != 201 {
		t.Fatalf("receipt: %d", c)
	}
	onHand := func() float64 {
		_, lv, _ := e.do("GET", "/v1/stock", log, nil)
		for _, r := range lv["data"].([]any) {
			m := r.(map[string]any)
			if m["item_id"] == item && m["warehouse_id"] == wh {
				return m["qty"].(float64)
			}
		}
		return 0
	}
	if c, _, _ := e.do("POST", "/v1/stock/counts", token("cnt-sales", "", "sales"), map[string]any{"warehouse_id": wh, "lines": []map[string]any{{"item_id": item, "counted": 1}}}); c != 403 {
		t.Errorf("sales counting: want 403, got %d", c)
	}
	for _, bad := range []map[string]any{
		{"warehouse_id": wh, "lines": []map[string]any{}},
		{"warehouse_id": wh, "lines": []map[string]any{{"item_id": item, "counted": -1}}},
		{"warehouse_id": wh, "lines": []map[string]any{{"item_id": item, "counted": 1}, {"item_id": item, "counted": 2}}},
	} {
		if c, _, _ := e.do("POST", "/v1/stock/counts", mgr1, bad); c != 422 {
			t.Errorf("invalid count %v: want 422, got %d", bad, c)
		}
	}
	code, cnt, _ := e.do("POST", "/v1/stock/counts", mgr1, map[string]any{"warehouse_id": wh, "lines": []map[string]any{{"item_id": item, "counted": 1}}})
	if code != 201 {
		t.Fatalf("count: %d %v", code, cnt)
	}
	id := cnt["id"].(string)
	line := cnt["lines"].([]any)[0].(map[string]any)
	if line["expected"] != float64(10) || line["counted"] != float64(1) || line["variance"] != float64(-9) {
		t.Errorf("the count must record expected 10, counted 1: %v", line)
	}
	if onHand() != 10 {
		t.Error("a draft count must not change stock")
	}
	// A shortfall against the minimum is reported before the count is approved or not: 10 on hand is fine.
	_, ro, _ := e.do("GET", "/v1/stock/reorder", log, nil)
	for _, r := range ro["data"].([]any) {
		if r.(map[string]any)["item_id"] == item {
			t.Error("10 on hand with a minimum of 2 must not be on the reorder list")
		}
	}
	if c, _, _ := e.do("POST", "/v1/stock/counts/"+id+"/approve", mgr1, nil); c != 403 {
		t.Errorf("the counter approving their own count: want 403, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/stock/counts/"+id+"/approve", log, nil); c != 403 {
		t.Errorf("logistics approving: want 403, got %d", c)
	}
	if c, b, _ := e.do("POST", "/v1/stock/counts/"+id+"/approve", mgr2, nil); c != 200 || b["status"] != "approved" {
		t.Fatalf("approve: %d %v", c, b)
	}
	if onHand() != 1 {
		t.Errorf("approval must apply the difference: want 1 on hand, got %v", onHand())
	}
	if c, _, _ := e.do("POST", "/v1/stock/counts/"+id+"/approve", mgr2, nil); c != 409 {
		t.Errorf("approve twice: want 409, got %d", c)
	}
	_, ro, _ = e.do("GET", "/v1/stock/reorder", log, nil)
	found := false
	for _, r := range ro["data"].([]any) {
		if m := r.(map[string]any); m["item_id"] == item {
			found = m["shortfall"] == float64(1) && m["on_hand"] == float64(1)
		}
	}
	if !found {
		t.Errorf("1 on hand against a minimum of 2 must be listed with a shortfall of 1: %v", ro)
	}
	if c, _, _ := e.do("GET", "/v1/stock/counts/"+id, log, nil); c != 200 {
		t.Errorf("get count: %d", c)
	}

	// The rules also hold in the database.
	conn, err := pgx.Connect(context.Background(), adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(context.Background(), `UPDATE inventory.counts SET counted_by = 'x' WHERE id = $1`, id); err == nil {
		t.Error("an approved count must not change, even with raw SQL")
	}
	if _, err := conn.Exec(context.Background(), `DELETE FROM inventory.count_lines WHERE count_id = $1`, id); err == nil {
		t.Error("the lines of an approved count must not be deleted")
	}
}
