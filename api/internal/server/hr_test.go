package server

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestLeaveWorkflow(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	emp := "emp-" + sfx
	hr := token("hr-1", "", "hr")
	mgr := token("hr-mgr", "", "manager")
	me := token(emp, "", "employee")
	peer := token("peer-"+sfx, "", "employee")

	if c, _, _ := e.do("POST", "/v1/leave", me, map[string]any{"kind": "annual", "start_date": "2026-12-07", "end_date": "2026-12-11"}); c != 409 {
		t.Errorf("leave without an employee record: want 409, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/employees", me, map[string]any{"actor": emp, "full_name": "Me", "hire_date": "2020-01-01"}); c != 403 {
		t.Errorf("employees cannot create records, got %d", c)
	}
	if c, b, _ := e.do("POST", "/v1/employees", hr, map[string]any{"actor": emp, "full_name": "Test Employee", "position": "Engineer", "hire_date": "2020-01-01"}); c != 201 {
		t.Fatalf("create employee: %d %v", c, b)
	}
	if c, _, _ := e.do("POST", "/v1/employees", hr, map[string]any{"actor": emp, "full_name": "Dup", "hire_date": "2020-01-01"}); c != 409 {
		t.Errorf("duplicate record: want 409, got %d", c)
	}
	_, rec, _ := e.do("GET", "/v1/employees/me", me, nil)
	if rec["annual_leave_balance"] != float64(18) {
		t.Fatalf("starting balance: %v", rec)
	}
	if c, _, _ := e.do("GET", "/v1/employees", me, nil); c != 403 {
		t.Errorf("employees cannot list everyone, got %d", c)
	}

	req := func(kind, a, b string) (int, map[string]any) {
		c, body, _ := e.do("POST", "/v1/leave", me, map[string]any{"kind": kind, "start_date": a, "end_date": b})
		return c, body
	}
	if c, _ := req("annual", "2026-12-12", "2026-12-13"); c != 422 { // Saturday and Sunday
		t.Errorf("weekend only: want 422, got %d", c)
	}
	if c, _ := req("annual", "2026-12-31", "2027-01-04"); c != 422 {
		t.Errorf("annual leave across years: want 422, got %d", c)
	}
	if c, _ := req("annual", "2026-11-02", "2026-12-04"); c != 422 { // 25 working days > 18
		t.Errorf("more than the balance: want 422, got %d", c)
	}
	if c, _ := req("holiday", "2026-12-07", "2026-12-08"); c != 422 {
		t.Errorf("unknown kind: want 422, got %d", c)
	}
	c, l1 := req("annual", "2026-12-07", "2026-12-11")
	if c != 201 || l1["days"] != float64(5) || l1["status"] != "pending" {
		t.Fatalf("request: %d %v", c, l1)
	}
	if c, _ := req("sick", "2026-12-09", "2026-12-10"); c != 409 {
		t.Errorf("overlapping request: want 409, got %d", c)
	}
	id := l1["id"].(string)
	if c, _, _ := e.do("POST", "/v1/leave/"+id+"/approve", me, nil); c != 403 {
		t.Errorf("employee cannot approve (route), got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/leave/"+id+"/approve", peer, nil); c != 403 {
		t.Errorf("peer cannot approve, got %d", c)
	}
	if c, b, _ := e.do("POST", "/v1/leave/"+id+"/approve", mgr, nil); c != 200 || b["status"] != "approved" || b["decided_by"] != "hr-mgr" {
		t.Fatalf("approve: %d %v", c, b)
	}
	if c, _, _ := e.do("POST", "/v1/leave/"+id+"/approve", mgr, nil); c != 409 {
		t.Errorf("deciding twice: want 409, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/leave/"+id+"/cancel", me, nil); c != 409 {
		t.Errorf("cancel an approved request: want 409, got %d", c)
	}
	_, rec, _ = e.do("GET", "/v1/employees/me", me, nil)
	if rec["annual_leave_taken"] != float64(5) || rec["annual_leave_balance"] != float64(13) {
		t.Errorf("balance after approval: %v", rec)
	}

	// Holidays are not counted.
	if c, _, _ := e.do("POST", "/v1/holidays", hr, map[string]any{"day": "2026-12-22", "name": "Test holiday"}); c != 201 {
		t.Fatalf("holiday: %d", c)
	}
	if c, b := req("annual", "2026-12-21", "2026-12-23"); c != 201 || b["days"] != float64(2) {
		t.Errorf("holiday must not count: %d %v", c, b)
	} else if c, _, _ := e.do("POST", "/v1/leave/"+b["id"].(string)+"/cancel", me, nil); c != 200 {
		t.Errorf("cancel own pending request: want 200, got %d", c)
	}
	// A peer sees only their own requests; HR with all=1 sees everyone's.
	_, mine, _ := e.do("GET", "/v1/leave?all=1", peer, nil)
	for _, x := range mine["data"].([]any) {
		if x.(map[string]any)["actor"] == emp {
			t.Error("a peer must not see someone else's leave")
		}
	}
	_, all, _ := e.do("GET", "/v1/leave?all=1", hr, nil)
	seen := false
	for _, x := range all["data"].([]any) {
		if x.(map[string]any)["id"] == id {
			seen = true
		}
	}
	if !seen {
		t.Error("HR should see all leave with all=1")
	}

	// Even around the API, an employee cannot approve their own leave.
	_, l2, _ := e.do("POST", "/v1/leave", me, map[string]any{"kind": "sick", "start_date": "2026-11-09", "end_date": "2026-11-09"})
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, _ := conn.Begin(ctx)
	defer tx.Rollback(ctx)
	_, _ = tx.Exec(ctx, `SET LOCAL ROLE hekate_app`)
	_, _ = tx.Exec(ctx, `SELECT set_config('app.role','staff',true), set_config('app.roles','employee',true), set_config('app.actor',$1,true)`, emp)
	_, err = tx.Exec(ctx, `UPDATE hr.leave_requests SET status='approved' WHERE id=$1`, l2["id"])
	if err == nil || !strings.Contains(err.Error(), "only HR or management") {
		t.Errorf("direct self-approval must be blocked by the database, got %v", err)
	}
}

func TestExpenseClaims(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	emp := token("claimer-"+sfx, "", "engineer")
	mgr := token("claim-mgr", "", "manager")
	fin := token("claim-fin", "", "finance")
	day := time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02")

	bad := []map[string]any{
		{"expense_date": "2020-01-01", "category": "travel", "description": "old", "amount": 5000},
		{"expense_date": day, "category": "travel", "description": "free", "amount": 0},
		{"expense_date": day, "category": "", "description": "x", "amount": 100},
	}
	for i, b := range bad {
		if c, _, _ := e.do("POST", "/v1/expenses", emp, b); c != 422 {
			t.Errorf("bad claim %d: want 422, got %d", i, c)
		}
	}
	c, cl, _ := e.do("POST", "/v1/expenses", emp, map[string]any{"expense_date": day, "category": "travel", "description": "Taxi to client site", "amount": 15000})
	if c != 201 || cl["status"] != "submitted" {
		t.Fatalf("claim: %d %v", c, cl)
	}
	id := cl["id"].(string)
	if c, _, _ := e.do("POST", "/v1/expenses/"+id+"/approve", emp, nil); c != 403 {
		t.Errorf("self approval: want 403, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/expenses/"+id+"/pay", fin, nil); c != 409 {
		t.Errorf("pay before approval: want 409, got %d", c)
	}
	if c, b, _ := e.do("POST", "/v1/expenses/"+id+"/approve", mgr, nil); c != 200 || b["status"] != "approved" {
		t.Fatalf("approve: %d %v", c, b)
	}
	if c, _, _ := e.do("POST", "/v1/expenses/"+id+"/pay", mgr, nil); c != 403 {
		t.Errorf("the approver cannot also pay: want 403, got %d", c)
	}
	if c, b, _ := e.do("POST", "/v1/expenses/"+id+"/pay", fin, nil); c != 200 || b["status"] != "paid" || b["paid_by"] != "claim-fin" {
		t.Fatalf("pay: %d %v", c, b)
	}
	// Visibility and immutability.
	_, other, _ := e.do("GET", "/v1/expenses?all=1", token("nosy-"+sfx, "", "employee"), nil)
	for _, x := range other["data"].([]any) {
		if x.(map[string]any)["id"] == id {
			t.Error("another employee must not see this claim")
		}
	}
	conn, err := pgx.Connect(context.Background(), adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(context.Background(), `UPDATE hr.expense_claims SET amount = 1 WHERE id=$1`, id); err == nil {
		t.Error("a claim's amount must never change")
	}
}
