package server

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestPayrollDualControlAndValidation(t *testing.T) {
	e := setup(t)
	n := time.Now().UnixNano()
	period := func(k int64) string { return fmt.Sprintf("%d-%02d", 1990+(n/1000+k*7)%35, 1+(n/1000/25+k*5)%12) }
	pa, pb := period(1), period(2)
	emp := fmt.Sprintf("pay-emp-%d", n)
	hr := token("pay-hr", "", "hr")
	fin1 := token("pay-fin1", "", "finance")
	fin2 := token("pay-fin2", "", "finance")
	ceo := token("pay-ceo", "", "ceo")
	me := token(emp, "", "employee")

	code, rec, _ := e.do("POST", "/v1/employees", hr, map[string]any{"actor": emp, "full_name": "Payroll Test " + emp, "hire_date": "1990-01-01"})
	if code != 201 {
		t.Fatalf("employee: %d %v", code, rec)
	}
	empID := rec["id"].(string)
	comp := map[string]any{"effective_from": "1990-01-01", "base_salary": 900000, "allowances": 100000}
	if c, _, _ := e.do("PUT", "/v1/payroll/compensation/"+empID, fin1, comp); c != 403 {
		t.Errorf("finance setting salaries: want 403, got %d", c)
	}
	if c, _, _ := e.do("PUT", "/v1/payroll/compensation/"+empID, me, comp); c != 403 {
		t.Errorf("employee setting own salary: want 403, got %d", c)
	}
	if c, b, _ := e.do("PUT", "/v1/payroll/compensation/"+empID, hr, comp); c != 201 {
		t.Fatalf("set compensation: %d %v", c, b)
	}
	if c, _, _ := e.do("PUT", "/v1/payroll/compensation/"+empID, hr, comp); c != 409 {
		t.Errorf("same-day salary twice: want 409, got %d", c)
	}

	// The rules need a second person's validation, and any change takes it away.
	_, cfg, _ := e.do("GET", "/v1/payroll/config", fin1, nil)
	rules := cfg["config"].(map[string]any)
	if c, _, _ := e.do("PUT", "/v1/payroll/config", hr, rules); c != 403 {
		t.Errorf("hr editing rules: want 403, got %d", c)
	}
	bad := map[string]any{}
	for k, v := range rules {
		bad[k] = v
	}
	bad["cac_bp"] = 20000
	if c, _, _ := e.do("PUT", "/v1/payroll/config", fin1, bad); c != 422 {
		t.Errorf("nonsense rules: want 422, got %d", c)
	}
	edited := map[string]any{}
	for k, v := range rules {
		edited[k] = v
	}
	edited["cac_bp"] = 1001
	if c, _, _ := e.do("PUT", "/v1/payroll/config", fin1, edited); c != 200 {
		t.Fatal("edit rules")
	}
	if c, _, _ := e.do("PUT", "/v1/payroll/config", fin1, rules); c != 200 {
		t.Fatal("restore rules")
	}
	if c, _, _ := e.do("POST", "/v1/payroll/config/validate", fin1, nil); c != 403 {
		t.Errorf("editor validating own rules: want 403, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/payroll/runs", fin1, map[string]any{"period": pa}); c != 409 {
		t.Errorf("run with unvalidated rules: want 409, got %d", c)
	}
	if c, b, _ := e.do("POST", "/v1/payroll/config/validate", fin2, nil); c != 200 || b["validated_by"] != "pay-fin2" {
		t.Fatalf("validate: %d %v", c, b)
	}

	// Prepare: the seeded rates give a known payslip for 1,000,000.
	code, run, _ := e.do("POST", "/v1/payroll/runs", fin1, map[string]any{"period": pa})
	if code != 201 || run["status"] != "draft" {
		t.Fatalf("create run: %d %v", code, run)
	}
	runID := run["id"].(string)
	var slip map[string]any
	for _, p := range run["payslips"].([]any) {
		if p.(map[string]any)["actor"] == emp {
			slip = p.(map[string]any)
		}
	}
	if slip == nil || slip["gross"] != float64(1_000_000) || slip["net"] != float64(806_126) || slip["irpp"] != float64(148_067) {
		t.Fatalf("payslip: %v", slip)
	}
	if c, _, _ := e.do("POST", "/v1/payroll/runs", fin1, map[string]any{"period": pa}); c != 409 {
		t.Errorf("second run for the month: want 409, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/payroll/runs", fin1, map[string]any{"period": "2999-01"}); c != 422 {
		t.Errorf("future month: want 422, got %d", c)
	}
	if _, mine, _ := e.do("GET", "/v1/payslips/me", me, nil); len(mine["data"].([]any)) != 0 {
		t.Errorf("a draft payslip must not be visible to the employee: %v", mine)
	}
	if c, _, _ := e.do("GET", "/v1/payroll/runs/"+runID, me, nil); c != 403 {
		t.Errorf("employee reading a run: want 403, got %d", c)
	}

	// Approve: not the preparer, not HR, then a second finance user.
	if c, _, _ := e.do("POST", "/v1/payroll/runs/"+runID+"/approve", hr, nil); c != 403 {
		t.Errorf("hr approving: want 403, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/payroll/runs/"+runID+"/approve", fin1, nil); c != 403 {
		t.Errorf("preparer approving own run: want 403, got %d", c)
	}
	if c, b, _ := e.do("POST", "/v1/payroll/runs/"+runID+"/approve", fin2, nil); c != 200 || b["status"] != "approved" {
		t.Fatalf("approve: %d %v", c, b)
	}
	if c, _, _ := e.do("POST", "/v1/payroll/runs/"+runID+"/approve", fin2, nil); c != 409 {
		t.Errorf("approve twice: want 409, got %d", c)
	}
	if c, _, _ := e.do("DELETE", "/v1/payroll/runs/"+runID, fin1, nil); c != 409 {
		t.Errorf("discard an approved run: want 409, got %d", c)
	}
	_, mine, _ := e.do("GET", "/v1/payslips/me", me, nil)
	if len(mine["data"].([]any)) != 1 || mine["data"].([]any)[0].(map[string]any)["net"] != float64(806_126) {
		t.Errorf("employee should now see their payslip: %v", mine)
	}
	slipID := mine["data"].([]any)[0].(map[string]any)["id"].(string)
	resp, pdfA := e.pdf(me, "/v1/payslips/"+slipID+"/pdf")
	if resp.StatusCode != 200 || !bytes.HasPrefix(pdfA, []byte("%PDF-")) || !bytes.Contains(pdfA, []byte("806 126 XAF")) {
		t.Fatalf("employee payslip pdf: %d", resp.StatusCode)
	}
	if _, pdfB := e.pdf(fin2, "/v1/payslips/"+slipID+"/pdf"); !bytes.Equal(pdfA, pdfB) {
		t.Error("payroll staff must get the same file as the employee")
	}
	if r, _ := e.pdf(token("pay-nosy-"+emp, "", "employee"), "/v1/payslips/"+slipID+"/pdf"); r.StatusCode != 404 {
		t.Errorf("another employee: want 404, got %d", r.StatusCode)
	}

	// Pay: neither the preparer nor the approver.
	for who, tk := range map[string]string{"preparer": fin1, "approver": fin2} {
		if c, _, _ := e.do("POST", "/v1/payroll/runs/"+runID+"/pay", tk, nil); c != 403 {
			t.Errorf("%s paying: want 403, got %d", who, c)
		}
	}
	if c, b, _ := e.do("POST", "/v1/payroll/runs/"+runID+"/pay", ceo, nil); c != 200 || b["status"] != "paid" {
		t.Fatalf("pay: %d %v", c, b)
	}

	// The ledger has the accrual and the payment, each balanced.
	owner, err := pgx.Connect(context.Background(), adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close(context.Background())
	var entries int
	var debit, credit int64
	if err := owner.QueryRow(context.Background(), `SELECT count(DISTINCT en.seq), COALESCE(sum(l.debit),0), COALESCE(sum(l.credit),0)
		FROM accounting.entries en JOIN accounting.entry_lines l ON l.entry_seq = en.seq WHERE en.source_ref = $1`, "PAY-"+pa).Scan(&entries, &debit, &credit); err != nil {
		t.Fatal(err)
	}
	if entries != 2 || debit != credit || debit == 0 {
		t.Errorf("ledger for %s: %d entries, debit %d credit %d", pa, entries, debit, credit)
	}

	// Changing the rules after a run was prepared makes that run unapprovable until re-validated.
	code, runB, _ := e.do("POST", "/v1/payroll/runs", fin1, map[string]any{"period": pb})
	if code != 201 {
		t.Fatalf("second run: %d %v", code, runB)
	}
	changed := map[string]any{}
	for k, v := range rules {
		changed[k] = v
	}
	changed["cac_bp"] = 1100
	if c, _, _ := e.do("PUT", "/v1/payroll/config", fin1, changed); c != 200 {
		t.Fatalf("change rules: %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/payroll/runs/"+runB["id"].(string)+"/approve", fin2, nil); c != 409 {
		t.Errorf("approve with stale rules: want 409, got %d", c)
	}
	// Restore the original rules and have them validated again; the old run is approvable once more.
	if c, _, _ := e.do("PUT", "/v1/payroll/config", fin1, rules); c != 200 {
		t.Fatal("restore rules")
	}
	if c, _, _ := e.do("POST", "/v1/payroll/config/validate", ceo, nil); c != 200 {
		t.Fatal("revalidate")
	}
	if c, b, _ := e.do("POST", "/v1/payroll/runs/"+runB["id"].(string)+"/approve", fin2, nil); c != 200 {
		t.Errorf("approve after revalidation: %d %v", c, b)
	}
}

// The same rules hold against raw SQL: the database, not only the API, refuses a self-approved run.
func TestPayrollGuardsHoldInTheDatabase(t *testing.T) {
	setup(t)
	ctx := context.Background()
	c, err := pgx.Connect(ctx, adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	tx, err := c.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.roles','finance',true), set_config('app.actor','sql-prep',true)`); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO payroll.runs (period, config, prepared_by) SELECT '1971-03-01', config, 'sql-prep' FROM payroll.config RETURNING id::text`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	try := func(q string, args ...any) string {
		if _, err := tx.Exec(ctx, `SAVEPOINT s`); err != nil {
			t.Fatal(err)
		}
		_, err := tx.Exec(ctx, q, args...)
		if _, e2 := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT s`); e2 != nil {
			t.Fatal(e2)
		}
		if err == nil {
			return ""
		}
		return err.Error()
	}
	if m := try(`UPDATE payroll.runs SET status='approved', approved_by='sql-prep' WHERE id=$1`, id); !strings.Contains(m, "someone other than the preparer") {
		t.Errorf("self-approval in SQL: %q", m)
	}
	if m := try(`UPDATE payroll.runs SET status='paid', paid_by='sql-pay' WHERE id=$1`, id); !strings.Contains(m, "invalid payroll status change") {
		t.Errorf("skipping approval in SQL: %q", m)
	}
	if m := try(`UPDATE payroll.runs SET prepared_by='someone-else' WHERE id=$1`, id); !strings.Contains(m, "cannot change") {
		t.Errorf("rewriting the preparer in SQL: %q", m)
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.roles','hr',true)`); err != nil {
		t.Fatal(err)
	}
	if m := try(`UPDATE payroll.runs SET status='approved', approved_by='sql-approver' WHERE id=$1`, id); !strings.Contains(m, "only finance or management") {
		t.Errorf("HR approving in SQL: %q", m)
	}
	if m := try(`UPDATE payroll.config SET validated_by = edited_by`); !strings.Contains(m, "someone other than the person who edited") {
		t.Errorf("self-validation in SQL: %q", m)
	}
}
