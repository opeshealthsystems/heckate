package server

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
	"testing"
	"time"
)

func TestClientOnlineFormFlow(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	mgr := token("fm-mgr", "", "manager")
	sup := token("fm-sup", "", "support")
	fin := token("fm-fin", "", "finance")
	ta := e.newClient(mgr, "Form Co "+sfx)
	tb := e.newClient(mgr, "Other Form Co "+sfx)
	clientA := token("fm-ca", ta, "client_user")
	clientB := token("fm-cb", tb, "client_user")
	fields := []map[string]any{
		{"key": "site", "label": "Site name", "type": "text", "required": true},
		{"key": "racks", "label": "Number of racks", "type": "number", "required": true},
		{"key": "install", "label": "Installation date", "type": "date"},
		{"key": "power", "label": "Power", "type": "select", "options": []string{"mains", "generator"}, "required": true},
		{"key": "ups", "label": "Has a UPS", "type": "checkbox", "required": true},
	}
	send := map[string]any{"tenant_id": ta, "title": "Site survey", "form_code": "FRM-14", "fields": fields, "due_date": day(7)}
	if code, _, _ := e.do("POST", "/v1/form-requests", fin, send); code != 403 {
		t.Errorf("finance must not send forms: %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/form-requests", clientA, send); code != 403 {
		t.Errorf("a client cannot send forms: %d", code)
	}
	dup := map[string]any{"tenant_id": ta, "title": "x", "fields": []map[string]any{{"key": "a", "label": "A", "type": "text"}, {"key": "a", "label": "B", "type": "text"}}}
	if code, _, _ := e.do("POST", "/v1/form-requests", sup, dup); code != 422 {
		t.Errorf("duplicate question key: want 422, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/form-requests", sup, map[string]any{"tenant_id": ta, "title": "x", "fields": []map[string]any{{"key": "p", "label": "P", "type": "select"}}}); code != 422 {
		t.Errorf("select without options: want 422, got %d", code)
	}
	code, f, _ := e.do("POST", "/v1/form-requests", sup, send)
	if code != 201 || f["status"] != "sent" {
		t.Fatalf("send: %d %v", code, f)
	}
	id := f["id"].(string)

	// Only the addressed client sees and fills it.
	if code, _, _ := e.do("GET", "/v1/form-requests/"+id, clientB, nil); code != 404 {
		t.Errorf("another client must not see it: %d", code)
	}
	_, l, _ := e.do("GET", "/v1/form-requests", clientB, nil)
	if len(listOf(t, l)) != 0 {
		t.Errorf("other client's inbox must be empty: %v", l)
	}
	_, l, _ = e.do("GET", "/v1/form-requests", clientA, nil)
	if len(listOf(t, l)) != 1 {
		t.Errorf("client inbox: %v", l)
	}
	if code, _, _ := e.do("PUT", "/v1/form-requests/"+id+"/answers", clientB, map[string]any{"answers": map[string]any{"site": "hack"}}); code != 404 {
		t.Errorf("another client cannot write answers: %d", code)
	}
	// A draft can be partial, but wrong types and unknown answers are refused.
	if code, _, _ := e.do("PUT", "/v1/form-requests/"+id+"/answers", clientA, map[string]any{"answers": map[string]any{"racks": "many"}}); code != 422 {
		t.Errorf("non-numeric number: want 422, got %d", code)
	}
	if code, _, _ := e.do("PUT", "/v1/form-requests/"+id+"/answers", clientA, map[string]any{"answers": map[string]any{"nope": 1}}); code != 422 {
		t.Errorf("unknown answer: want 422, got %d", code)
	}
	if code, _, _ := e.do("PUT", "/v1/form-requests/"+id+"/answers", clientA, map[string]any{"answers": map[string]any{"power": "solar"}}); code != 422 {
		t.Errorf("option not listed: want 422, got %d", code)
	}
	if code, g, _ := e.do("PUT", "/v1/form-requests/"+id+"/answers", clientA, map[string]any{"answers": map[string]any{"site": "Douala DC"}}); code != 200 || g["answers"].(map[string]any)["site"] != "Douala DC" {
		t.Fatalf("save draft: %d %v", code, g)
	}
	if code, _, _ := e.do("POST", "/v1/form-requests/"+id+"/submit", clientA, map[string]any{"answers": map[string]any{"site": "Douala DC"}}); code != 422 {
		t.Errorf("submit with required answers missing: want 422, got %d", code)
	}
	full := map[string]any{"site": "Douala DC", "racks": 12, "install": "2026-03-01", "power": "generator", "ups": false}
	code, g, _ := e.do("POST", "/v1/form-requests/"+id+"/submit", clientA, map[string]any{"answers": full})
	if code != 200 || g["status"] != "submitted" || g["submitted_by"] != "fm-ca" {
		t.Fatalf("submit: %d %v", code, g)
	}
	if code, _, _ := e.do("PUT", "/v1/form-requests/"+id+"/answers", clientA, map[string]any{"answers": full}); code != 409 && code != 404 {
		t.Errorf("a submitted form is locked for the client: got %d", code)
	}
	// Review: return needs a note, then the client fixes it, then it is accepted.
	if code, _, _ := e.do("POST", "/v1/form-requests/"+id+"/return", sup, map[string]any{}); code != 422 {
		t.Errorf("return without a note: want 422, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/form-requests/"+id+"/accept", clientA, map[string]any{}); code != 403 {
		t.Errorf("a client cannot accept its own form: %d", code)
	}
	if code, g, _ := e.do("POST", "/v1/form-requests/"+id+"/return", sup, map[string]any{"note": "Racks count looks wrong"}); code != 200 || g["status"] != "returned" {
		t.Fatalf("return: %d %v", code, g)
	}
	full["racks"] = 10
	if code, g, _ := e.do("POST", "/v1/form-requests/"+id+"/submit", clientA, map[string]any{"answers": full}); code != 200 || g["status"] != "submitted" {
		t.Fatalf("resubmit: %d %v", code, g)
	}
	code, g, _ = e.do("POST", "/v1/form-requests/"+id+"/accept", sup, map[string]any{})
	if code != 200 || g["status"] != "accepted" || g["reviewed_by"] != "fm-sup" {
		t.Fatalf("accept: %d %v", code, g)
	}
	var acts []string
	for _, h := range g["history"].([]any) {
		acts = append(acts, h.(map[string]any)["action"].(string))
	}
	if strings.Join(acts, " ") != "sent submitted returned submitted accepted" {
		t.Errorf("history = %v", acts)
	}
	if code, _, _ := e.do("POST", "/v1/form-requests/"+id+"/cancel", sup, map[string]any{}); code != 409 {
		t.Errorf("an accepted form is closed: %d", code)
	}

	// A paper form keyed in by staff is marked as such.
	_, f2, _ := e.do("POST", "/v1/form-requests", sup, send)
	id2 := f2["id"].(string)
	code, g, _ = e.do("POST", "/v1/form-requests/"+id2+"/enter", sup, map[string]any{"answers": full})
	if code != 200 || g["on_behalf"] != true || g["submitted_by"] != "fm-sup" {
		t.Errorf("enter from paper: %d %v", code, g)
	}
	if code, _, _ := e.do("POST", "/v1/form-requests/"+id2+"/enter", clientA, map[string]any{"answers": full}); code != 403 {
		t.Errorf("a client cannot key in on behalf: %d", code)
	}
	resp, csv := e.pdf(sup, "/v1/exports/form-requests")
	if resp.StatusCode != 200 || !strings.Contains(string(csv), "Site survey") {
		t.Errorf("export: %d", resp.StatusCode)
	}
}

func TestProjectProgressBudgetAndFromQuote(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	mgr := token("pp-mgr", "", "manager")
	fin := token("pp-fin", "", "finance")
	sales := token("pp-sales", "", "sales")
	eng := token("pp-eng", "", "engineer")
	ta := e.newClient(mgr, "Progress Co "+sfx)
	tb := e.newClient(mgr, "Other Progress Co "+sfx)
	clientA := token("pp-ca", ta, "client_admin")
	clientB := token("pp-cb", tb, "client_admin")

	// From an approved quote: the budget is the quote total and each line becomes a task.
	_, q, _ := e.do("POST", "/v1/quotes", sales, map[string]any{"tenant_id": ta, "title": "Rack refresh " + sfx, "lines": []map[string]any{
		{"description": "Supply racks", "quantity": 2, "unit_price": 500_000}, {"description": "Install and test", "quantity": 1, "unit_price": 250_000}}})
	qid := q["id"].(string)
	body := map[string]any{"quote_id": qid, "manager": "pp-mgr", "start_date": day(-10), "end_date": day(30)}
	if code, _, _ := e.do("POST", "/v1/projects/from-quote", mgr, body); code != 409 {
		t.Errorf("a draft quote cannot start a project: %d", code)
	}
	e.do("POST", "/v1/quotes/"+qid+"/send", sales, nil)
	e.do("POST", "/v1/quotes/"+qid+"/approve", token("pp-ca2", ta, "client_admin"), nil)
	if code, _, _ := e.do("POST", "/v1/projects/from-quote", eng, body); code != 403 {
		t.Errorf("engineer cannot start projects: %d", code)
	}
	code, p, _ := e.do("POST", "/v1/projects/from-quote", mgr, body)
	if code != 201 || p["budget"].(float64) != 1_250_000 {
		t.Fatalf("from quote: %d %v", code, p)
	}
	if code, _, _ := e.do("POST", "/v1/projects/from-quote", mgr, body); code != 409 {
		t.Errorf("second project for the same quote: want 409, got %d", code)
	}
	pid := p["id"].(string)
	_, d, _ := e.do("GET", "/v1/projects/"+pid, mgr, nil)
	tasks := d["tasks"].([]any)
	if len(tasks) != 2 {
		t.Fatalf("one task per quote line: %v", tasks)
	}

	// Milestones and tasks drive the percentage.
	_, m1, _ := e.do("POST", "/v1/projects/"+pid+"/milestones", mgr, map[string]any{"title": "Racks delivered", "due_date": day(-2)})
	_, m2, _ := e.do("POST", "/v1/projects/"+pid+"/milestones", mgr, map[string]any{"title": "Handover", "due_date": day(25)})
	_, pr, _ := e.do("GET", "/v1/projects/"+pid+"/progress", mgr, nil)
	if pr["percent"].(float64) != 0 || pr["late_milestones"].(float64) != 1 || pr["behind_schedule"] != true {
		t.Errorf("a late milestone means behind schedule: %v", pr)
	}
	if code, _, _ := e.do("POST", "/v1/milestones/"+m1["id"].(string)+"/done", eng, nil); code != 200 {
		t.Errorf("engineer completes a milestone: %d", code)
	}
	e.do("PATCH", "/v1/tasks/"+tasks[0].(map[string]any)["id"].(string), eng, map[string]any{"status": "done"})
	_, pr, _ = e.do("GET", "/v1/projects/"+pid+"/progress", mgr, nil)
	if pr["percent"].(float64) != 50 || pr["milestones_done"].(float64) != 1 || pr["tasks_done"].(float64) != 1 || pr["late_milestones"].(float64) != 0 {
		t.Errorf("2 of 4 done = 50%%: %v", pr)
	}
	_ = m2

	// Budget against actual cost: finance and management record costs; the client never sees money.
	if code, _, _ := e.do("POST", "/v1/projects/"+pid+"/costs", eng, map[string]any{"description": "x", "amount": 1}); code != 403 {
		t.Errorf("engineer cannot record costs: %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/projects/"+pid+"/costs", fin, map[string]any{"description": "Zero", "amount": 0}); code != 422 {
		t.Errorf("zero cost: want 422, got %d", code)
	}
	code, cst, _ := e.do("POST", "/v1/projects/"+pid+"/costs", fin, map[string]any{"description": "Racks", "category": "materials", "amount": 900_000})
	if code != 201 {
		t.Fatalf("cost: %d %v", code, cst)
	}
	_, pr, _ = e.do("GET", "/v1/projects/"+pid+"/progress", fin, nil)
	if pr["actual"].(float64) != 900_000 || pr["remaining"].(float64) != 350_000 || pr["over_budget"] != false {
		t.Errorf("budget vs actual: %v", pr)
	}
	e.do("POST", "/v1/projects/"+pid+"/costs", mgr, map[string]any{"description": "Overtime", "category": "labour", "amount": 500_000})
	_, pr, _ = e.do("GET", "/v1/projects/"+pid+"/progress", mgr, nil)
	if pr["over_budget"] != true || pr["remaining"].(float64) != -150_000 {
		t.Errorf("over budget: %v", pr)
	}
	_, pr, _ = e.do("GET", "/v1/projects/"+pid+"/progress", clientA, nil)
	if pr["percent"].(float64) != 50 {
		t.Errorf("the client sees progress: %v", pr)
	}
	for _, k := range []string{"budget", "actual", "remaining", "costs", "over_budget"} {
		if _, has := pr[k]; has {
			t.Errorf("the client must not see %s", k)
		}
	}
	if code, _, _ := e.do("GET", "/v1/projects/"+pid+"/progress", clientB, nil); code != 404 {
		t.Errorf("another client: want 404, got %d", code)
	}
	_, pf, _ := e.do("GET", "/v1/projects/portfolio", clientA, nil)
	if len(listOf(t, pf)) != 1 || listOf(t, pf)[0].(map[string]any)["actual"] != nil {
		t.Errorf("client portfolio: %v", pf)
	}
	_, pf, _ = e.do("GET", "/v1/projects/portfolio", mgr, nil)
	found := false
	for _, x := range listOf(t, pf) {
		if m := x.(map[string]any); m["project_id"] == pid {
			found = m["percent"].(float64) == 50 && m["over_budget"] == true
		}
	}
	if !found {
		t.Errorf("portfolio line missing or wrong")
	}
	if code, _, _ := e.do("DELETE", "/v1/costs/"+cst["id"].(string), fin, nil); code != 204 {
		t.Errorf("delete cost: %d", code)
	}
}

func TestQRLabelsForAssetsAndItems(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	log := token("lb-log", "", "logistics")
	mgr := token("lb-mgr", "", "manager")
	item := mkItem(t, e, log, "LB-"+sfx, true)
	wh := mkWarehouse(t, e, log, "Labels "+sfx)
	e.do("POST", "/v1/stock/receipts", log, map[string]any{"item_id": item, "warehouse_id": wh, "qty": 2, "serials": []string{"L1-" + sfx, "L2-" + sfx}})
	_, assets, _ := e.do("GET", "/v1/assets", log, nil)
	var ids []string
	for _, a := range listOf(t, assets) {
		m := a.(map[string]any)
		if strings.HasSuffix(m["serial"].(string), sfx) {
			ids = append(ids, m["id"].(string))
		}
	}
	if len(ids) != 2 {
		t.Fatalf("assets: %v", ids)
	}
	resp, pdf := e.pdf(log, "/v1/assets/labels/pdf?ids="+strings.Join(ids, ","))
	if resp.StatusCode != 200 || !strings.HasPrefix(string(pdf), "%PDF") {
		t.Fatalf("asset labels: %d", resp.StatusCode)
	}
	if resp2, pdf2 := e.pdf(log, "/v1/assets/labels/pdf?ids="+strings.Join(ids, ",")); resp2.StatusCode != 200 || len(pdf2) == 0 {
		t.Errorf("repeat labels: %d", resp2.StatusCode)
	}
	if resp, _ := e.pdf(log, "/v1/assets/labels/pdf"); resp.StatusCode != 422 {
		t.Errorf("no ids: want 422, got %d", resp.StatusCode)
	}
	if resp, _ := e.pdf(log, "/v1/assets/labels/pdf?ids=00000000-0000-0000-0000-000000000000"); resp.StatusCode != 404 {
		t.Errorf("unknown asset: want 404, got %d", resp.StatusCode)
	}
	if resp, _ := e.pdf(token("lb-c", "", "client_user"), "/v1/assets/labels/pdf?ids="+ids[0]); resp.StatusCode != 403 {
		t.Errorf("a client cannot print labels: %d", resp.StatusCode)
	}
	if resp, pdf := e.pdf(mgr, "/v1/items/labels/pdf?ids="+item); resp.StatusCode != 200 || !strings.HasPrefix(string(pdf), "%PDF") {
		t.Errorf("item labels: %d", resp.StatusCode)
	}
	// Where a scanned sticker lands.
	code, a, _ := e.do("GET", "/v1/assets/"+ids[0], log, nil)
	if code != 200 || a["asset"].(map[string]any)["id"] != ids[0] {
		t.Errorf("asset lookup: %d %v", code, a)
	}
	if code, _, _ := e.do("GET", "/v1/assets/not-an-id", log, nil); code != 404 {
		t.Errorf("bad id: want 404, got %d", code)
	}
	if code, it, _ := e.do("GET", "/v1/items/"+item, log, nil); code != 200 || it["on_hand"].(float64) != 2 {
		t.Errorf("item lookup: %d %v", code, it)
	}
}

func TestEmployeeFileContractsAndPrivacy(t *testing.T) {
	e := setup(t)
	n := time.Now().UnixNano()
	actor := fmt.Sprintf("ef-emp-%d", n)
	hr := token("ef-hr", "", "hr")
	mgr := token("ef-mgr", "", "manager")
	fin := token("ef-fin", "", "finance")
	me := token(actor, "", "employee")
	other := token("ef-other", "", "employee")

	_, rec, _ := e.do("POST", "/v1/employees", hr, map[string]any{"actor": actor, "full_name": "File Test", "position": "Technician", "hire_date": "2024-01-15"})
	id := rec["id"].(string)

	priv := map[string]any{"date_of_birth": "1990-05-20", "national_id": "ID-123", "cnps_number": "CNPS-9", "phone": "+237600000000", "bank_name": "Test Bank", "bank_account": "0001-2345", "emergency_name": "Kin", "emergency_phone": "+237611111111"}
	if code, _, _ := e.do("PUT", "/v1/employees/"+id+"/private", mgr, priv); code != 403 {
		t.Errorf("a manager cannot edit personal details: %d", code)
	}
	if code, _, _ := e.do("PUT", "/v1/employees/"+id+"/private", hr, map[string]any{"date_of_birth": "2999-01-01"}); code != 422 {
		t.Errorf("future birth date: want 422, got %d", code)
	}
	code, f, _ := e.do("PUT", "/v1/employees/"+id+"/private", hr, priv)
	if code != 200 || f["private"].(map[string]any)["national_id"] != "ID-123" {
		t.Fatalf("private: %d %v", code, f)
	}
	// HR and the person see the personal details; a manager sees the file without them; colleagues see nothing.
	if _, f, _ = e.do("GET", "/v1/employees/"+id+"/file", me, nil); f["private"] == nil {
		t.Errorf("an employee sees their own details")
	}
	if _, f, _ = e.do("GET", "/v1/employees/"+id+"/file", mgr, nil); f["private"] != nil || f["employee"] == nil {
		t.Errorf("a manager sees the file without personal details: %v", f)
	}
	if code, _, _ := e.do("GET", "/v1/employees/"+id+"/file", other, nil); code != 404 {
		t.Errorf("a colleague must not open another's file: %d", code)
	}

	// Employment contracts: a CDD needs an end; activation needs a signing date; one live contract at a time.
	if code, _, _ := e.do("POST", "/v1/employees/"+id+"/contracts", hr, map[string]any{"type": "CDD", "position": "Technician", "start_date": "2024-01-15"}); code != 422 {
		t.Errorf("CDD without an end: want 422, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/employees/"+id+"/contracts", mgr, map[string]any{"type": "CDI", "position": "Technician", "start_date": "2024-01-15"}); code != 403 {
		t.Errorf("manager cannot draft employment contracts: %d", code)
	}
	code, c1, _ := e.do("POST", "/v1/employees/"+id+"/contracts", hr, map[string]any{"type": "CDD", "position": "Technician", "start_date": "2024-01-15", "end_date": "2024-12-31", "probation_end": "2024-03-15"})
	if code != 201 || c1["status"] != "draft" {
		t.Fatalf("contract: %d %v", code, c1)
	}
	if code, _, _ := e.do("POST", "/v1/employment-contracts/"+c1["id"].(string)+"/activate", hr, nil); code != 422 {
		t.Errorf("activate unsigned: want 422, got %d", code)
	}
	_, c1, _ = e.do("POST", "/v1/employees/"+id+"/contracts", hr, map[string]any{"type": "CDD", "position": "Senior Technician", "start_date": "2024-01-15", "end_date": "2024-12-31", "signed_on": "2024-01-10"})
	if code, a, _ := e.do("POST", "/v1/employment-contracts/"+c1["id"].(string)+"/activate", hr, nil); code != 200 || a["status"] != "active" {
		t.Fatalf("activate: %d %v", code, a)
	}
	_, c2, _ := e.do("POST", "/v1/employees/"+id+"/contracts", hr, map[string]any{"type": "CDI", "position": "Lead Technician", "start_date": "2025-01-01", "signed_on": "2024-12-20"})
	if code, _, _ := e.do("POST", "/v1/employment-contracts/"+c2["id"].(string)+"/activate", hr, nil); code != 200 {
		t.Fatalf("activate the CDI: %d", code)
	}
	_, f, _ = e.do("GET", "/v1/employees/"+id+"/file", hr, nil)
	live := 0
	for _, c := range f["contracts"].([]any) {
		if c.(map[string]any)["status"] == "active" {
			live++
		}
	}
	if live != 1 || f["employee"].(map[string]any)["position"] != "Lead Technician" {
		t.Errorf("one live contract and the position follows it: live=%d %v", live, f["employee"])
	}
	if code, _, _ := e.do("GET", "/v1/employees/"+id+"/file", fin, nil); code != 200 {
		t.Errorf("finance reads the file: %d", code)
	}
	if code, _, _ := e.do("PATCH", "/v1/employees/"+id, hr, map[string]any{"status": "retired"}); code != 422 {
		t.Errorf("bad status: want 422, got %d", code)
	}
	resp, csv := e.pdf(hr, "/v1/exports/employees")
	if resp.StatusCode != 200 || !strings.Contains(string(csv), actor) || strings.Contains(string(csv), "ID-123") || strings.Contains(string(csv), "0001-2345") {
		t.Errorf("staff register: %d (it must not carry personal details)", resp.StatusCode)
	}
	if resp, _ := e.pdf(me, "/v1/exports/employees"); resp.StatusCode != 403 {
		t.Errorf("an employee cannot export the register: %d", resp.StatusCode)
	}
}

func TestCompanyBankAccountRegister(t *testing.T) {
	e := setup(t)
	fin := token("ba-fin", "", "finance")
	aud := token("ba-aud", "", "auditor")
	sales := token("ba-sales", "", "sales")
	acct := fmt.Sprintf("5%07d", time.Now().UnixNano()/1000%10_000_000)
	e.do("POST", "/v1/accounting/accounts", fin, map[string]any{"code": acct, "name": "Register test " + acct})
	body := map[string]any{"ledger_account": acct, "bank_name": "Test Bank", "account_name": "HEKATE SYSTEMS SARL", "account_number": "00012345678", "branch": "Douala", "signatories": "CEO and CFO"}
	if code, _, _ := e.do("POST", "/v1/bank/accounts", sales, body); code != 403 {
		t.Errorf("sales cannot register accounts: %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/bank/accounts", fin, map[string]any{"ledger_account": acct}); code != 422 {
		t.Errorf("missing fields: want 422, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/bank/accounts", fin, map[string]any{"ledger_account": "9999999", "bank_name": "B", "account_name": "N", "account_number": "1"}); code != 422 {
		t.Errorf("unknown ledger account: want 422, got %d", code)
	}
	code, b, _ := e.do("POST", "/v1/bank/accounts", fin, body)
	if code != 201 || b["status"] != "active" || b["ledger_balance"].(float64) != 0 {
		t.Fatalf("create: %d %v", code, b)
	}
	if code, _, _ := e.do("POST", "/v1/bank/accounts", fin, body); code != 409 {
		t.Errorf("one bank account per ledger account: want 409, got %d", code)
	}
	if code, l, _ := e.do("GET", "/v1/bank/accounts", aud, nil); code != 200 || len(listOf(t, l)) < 1 {
		t.Errorf("the auditor reads the register: %d", code)
	}
	if code, _, _ := e.do("GET", "/v1/bank/accounts", sales, nil); code != 403 {
		t.Errorf("sales cannot read it: %d", code)
	}
	id := b["id"].(string)
	if code, u, _ := e.do("PATCH", "/v1/bank/accounts/"+id, fin, map[string]any{"signatories": "CEO, CFO and COO"}); code != 200 || u["signatories"] != "CEO, CFO and COO" {
		t.Errorf("update: %d %v", code, u)
	}
	if code, u, _ := e.do("PATCH", "/v1/bank/accounts/"+id, fin, map[string]any{"close": true}); code != 200 || u["status"] != "closed" || u["closed_on"] == nil {
		t.Errorf("close: %d %v", code, u)
	}
	resp, csv := e.pdf(aud, "/v1/exports/bank-accounts")
	if resp.StatusCode != 200 || !strings.Contains(string(csv), "00012345678") {
		t.Errorf("export: %d", resp.StatusCode)
	}
}

func TestNewHistoriesAreAppendOnlyInTheDatabase(t *testing.T) {
	e := setup(t)
	mgr := token("ao-mgr", "", "manager")
	sup := token("ao-sup", "", "support")
	ta := e.newClient(mgr, "Append Co "+fmt.Sprint(time.Now().UnixNano()))
	_, c, _ := e.do("POST", "/v1/contracts", mgr, map[string]any{"tenant_id": ta, "title": "T", "start_date": day(0), "end_date": day(30)})
	_, f, _ := e.do("POST", "/v1/form-requests", sup, map[string]any{"tenant_id": ta, "title": "T", "fields": []map[string]any{{"key": "a", "label": "A", "type": "text"}}})
	conn, err := pgx.Connect(context.Background(), adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	for name, q := range map[string]string{
		"edit contract history":   fmt.Sprintf(`UPDATE contracts.events SET action='x' WHERE contract_id='%s'`, c["id"]),
		"delete contract history": fmt.Sprintf(`DELETE FROM contracts.events WHERE contract_id='%s'`, c["id"]),
		"edit form history":       fmt.Sprintf(`UPDATE forms.events SET action='x' WHERE request_id='%s'`, f["id"]),
		"delete form history":     fmt.Sprintf(`DELETE FROM forms.events WHERE request_id='%s'`, f["id"]),
		"clause on a live one":    fmt.Sprintf(`INSERT INTO contracts.clauses (contract_id, tenant_id, position, title, body) VALUES ('%s','%s',1,'a','b')`, c["id"], ta),
	} {
		if name == "clause on a live one" {
			conn.Exec(context.Background(), fmt.Sprintf(`UPDATE contracts.contracts SET status='active' WHERE id='%s'`, c["id"]))
		}
		if _, err := conn.Exec(context.Background(), q); err == nil {
			t.Errorf("%s must be refused by the database", name)
		}
	}
}
