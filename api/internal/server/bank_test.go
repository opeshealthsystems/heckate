package server

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestBankReconciliation(t *testing.T) {
	e := setup(t)
	fin := token("bank-fin", "", "finance")
	aud := token("bank-aud", "", "auditor")
	cli := token("bank-cli", "", "client_admin")
	// A treasury account of its own, so the ledger starts empty for this run.
	acct := fmt.Sprintf("5%07d", time.Now().UnixNano()/1000%10_000_000)
	if c, b, _ := e.do("POST", "/v1/accounting/accounts", fin, map[string]any{"code": acct, "name": "Bank test " + acct}); c != 201 {
		t.Fatalf("add account: %d %v", c, b)
	}
	y := 2026
	d := func(m, day int) string { return fmt.Sprintf("%04d-%02d-%02d", y, m, day) }
	post := func(date string, debit, credit int64) {
		line := map[string]any{"account": acct}
		other := map[string]any{"account": "101"}
		if debit > 0 {
			line["debit"], other["credit"] = debit, debit
		} else {
			line["credit"], other["debit"] = credit, credit
		}
		if c, b, _ := e.do("POST", "/v1/accounting/entries", fin, map[string]any{"journal": "BQ", "entry_date": date, "narration": "bank test", "lines": []map[string]any{line, other}}); c != 201 {
			t.Fatalf("post: %d %v", c, b)
		}
	}
	post(d(3, 1), 300000, 0)
	post(d(3, 5), 125000, 0)
	post(d(3, 5), 125000, 0) // a second, identical receipt: the bank line for 125,000 is ambiguous
	post(d(3, 9), 0, 40000)

	csv := "date,description,amount,reference\n" +
		d(3, 2) + ",TRANSFER IN ACME,300000,TR1\n" +
		d(3, 6) + ",TRANSFER IN GLOBEX,125000,TR2\n" +
		d(3, 9) + ",BANK CHARGES,-40000,\n"
	body := map[string]any{"account": acct, "opening": 0, "closing": 385000, "csv": csv}

	if c, _, _ := e.do("POST", "/v1/bank/statements", cli, body); c != 403 {
		t.Errorf("client importing: want 403, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/bank/statements", aud, body); c != 403 {
		t.Errorf("auditor importing: want 403, got %d", c)
	}
	wrong := map[string]any{"account": acct, "opening": 0, "closing": 999, "csv": csv}
	if c, _, _ := e.do("POST", "/v1/bank/statements", fin, wrong); c != 422 {
		t.Errorf("closing balance that does not add up: want 422, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/bank/statements", fin, map[string]any{"account": "601", "opening": 0, "closing": 385000, "csv": csv}); c != 422 {
		t.Errorf("non-bank account: want 422, got %d", c)
	}
	code, st, _ := e.do("POST", "/v1/bank/statements", fin, body)
	if code != 201 {
		t.Fatalf("import: %d %v", code, st)
	}
	id := st["statement"].(map[string]any)["id"].(string)
	lines := st["lines"].([]any)
	if len(lines) != 3 {
		t.Fatalf("lines: %v", lines)
	}
	// 300,000 and -40,000 each have exactly one candidate; 125,000 has two.
	if n := len(lines[0].(map[string]any)["suggestions"].([]any)); n != 1 {
		t.Errorf("300,000 suggestions: %d", n)
	}
	if n := len(lines[1].(map[string]any)["suggestions"].([]any)); n != 2 {
		t.Errorf("125,000 suggestions: want 2, got %d", n)
	}

	code, am, _ := e.do("POST", "/v1/bank/statements/"+id+"/auto-match", fin, nil)
	if code != 200 || am["auto_matched"] != float64(2) {
		t.Fatalf("auto-match: %d %v", code, am)
	}
	sum := am["summary"].(map[string]any)
	if sum["reconciled"] != false || sum["difference"] != float64(125000) {
		t.Errorf("summary after auto-match: %v", sum)
	}
	open := am["unmatched_ledger"].([]any)
	if len(open) != 2 {
		t.Fatalf("two identical receipts remain open, got %v", open)
	}
	amb := am["lines"].([]any)[1].(map[string]any)
	if amb["entry_line_id"] != nil {
		t.Error("an ambiguous line must not be matched automatically")
	}

	// Manual: pick the right one; a ledger line of a different amount is refused.
	bankLine := amb["id"].(string)
	first := int64(open[0].(map[string]any)["entry_line_id"].(float64))
	if c, _, _ := e.do("POST", "/v1/bank/lines/"+bankLine+"/match", fin, map[string]any{"entry_line_id": int64(1) << 40}); c != 422 {
		t.Errorf("unknown ledger line: want 422, got %d", c)
	}
	matched := am["lines"].([]any)[0].(map[string]any)["entry_line_id"].(float64)
	if c, _, _ := e.do("POST", "/v1/bank/lines/"+bankLine+"/match", fin, map[string]any{"entry_line_id": int64(matched)}); c != 422 {
		t.Errorf("ledger line with another amount: want 422, got %d", c)
	}
	if c, b, _ := e.do("POST", "/v1/bank/lines/"+bankLine+"/match", fin, map[string]any{"entry_line_id": first}); c != 204 {
		t.Fatalf("manual match: %d %v", c, b)
	}
	if c, _, _ := e.do("POST", "/v1/bank/lines/"+bankLine+"/match", fin, map[string]any{"entry_line_id": first}); c != 409 {
		t.Errorf("matching twice: want 409, got %d", c)
	}
	if c, _, _ := e.do("DELETE", "/v1/bank/lines/"+bankLine+"/match", fin, nil); c != 204 {
		t.Errorf("unmatch: %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/bank/lines/"+bankLine+"/match", fin, map[string]any{"entry_line_id": first}); c != 204 {
		t.Errorf("rematch: %d", c)
	}

	_, got, _ := e.do("GET", "/v1/bank/statements/"+id, aud, nil)
	if got["summary"].(map[string]any)["unmatched_ledger_total"] != float64(125000) {
		t.Errorf("one identical receipt is still outstanding: %v", got["summary"])
	}
	_, list, _ := e.do("GET", "/v1/bank/statements", aud, nil)
	if len(list["data"].([]any)) == 0 {
		t.Error("statements list is empty")
	}
	if c, _, _ := e.do("GET", "/v1/bank/statements/"+id, cli, nil); c != 403 {
		t.Errorf("client reading statements: want 403, got %d", c)
	}

	// The database refuses edits whatever route is used.
	conn, err := pgx.Connect(context.Background(), adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	for name, q := range map[string]string{
		"edit amount":      `UPDATE accounting.bank_lines SET amount = 1 WHERE statement_id = $1`,
		"delete line":      `DELETE FROM accounting.bank_lines WHERE statement_id = $1`,
		"edit statement":   `UPDATE accounting.bank_statements SET closing = 1 WHERE id = $1`,
		"delete statement": `DELETE FROM accounting.bank_statements WHERE id = $1`,
	} {
		if _, err := conn.Exec(context.Background(), q, id); err == nil || !strings.Contains(err.Error(), "cannot be") {
			t.Errorf("%s in SQL: %v", name, err)
		}
	}
}
