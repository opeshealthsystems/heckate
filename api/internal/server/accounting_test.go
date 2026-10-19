package server

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

func balances(t *testing.T, e *env, tok string) map[string]float64 {
	t.Helper()
	code, tb, _ := e.do("GET", "/v1/accounting/trial-balance", tok, nil)
	if code != 200 {
		t.Fatalf("trial balance: %d %v", code, tb)
	}
	if tb["balanced"] != true {
		t.Fatalf("trial balance must always balance: %v", tb)
	}
	out := map[string]float64{}
	for _, r := range tb["rows"].([]any) {
		m := r.(map[string]any)
		out[m["account"].(string)] = m["balance"].(float64)
	}
	return out
}

func TestSalesPostToTheLedger(t *testing.T) {
	e := setup(t)
	fin := token("fin-acc", "", "finance")
	tid := e.newClient(token("m", "", "manager"), "Ledger "+t.Name())
	before := balances(t, e, fin)

	_, inv, _ := e.do("POST", "/v1/invoices", fin, map[string]any{"tenant_id": tid, "title": "Ledgered", "lines": lines(1_000_000)})
	id := inv["id"].(string)
	if code, _, _ := e.do("POST", "/v1/invoices/"+id+"/issue", fin, nil); code != 200 {
		t.Fatal("issue failed")
	}
	after := balances(t, e, fin)
	d := func(acct string) float64 { return after[acct] - before[acct] }
	if d("411") != 1_192_500 || d("706") != -1_000_000 || d("4431") != -175_000 || d("4439") != -17_500 {
		t.Errorf("invoice posting wrong: 411=%v 706=%v 4431=%v 4439=%v", d("411"), d("706"), d("4431"), d("4439"))
	}

	e.do("POST", "/v1/invoices/"+id+"/payments", fin, map[string]any{"amount": 500_000, "method": "momo"})
	e.do("POST", "/v1/invoices/"+id+"/payments", fin, map[string]any{"amount": 100_000, "method": "cash"})
	e.do("POST", "/v1/invoices/"+id+"/credit-notes", fin, map[string]any{"net": 100_000, "reason": "discount"})
	final := balances(t, e, fin)
	f := func(acct string) float64 { return final[acct] - before[acct] }
	if f("5211") != 500_000 || f("571") != 100_000 {
		t.Errorf("settlement accounts: 5211=%v 571=%v", f("5211"), f("571"))
	}
	if f("709") != 100_000 {
		t.Errorf("credit note net must debit 709, got %v", f("709"))
	}
	// 411: invoice 1,192,500 - payments 600,000 - credit note 119,250
	if f("411") != 1_192_500-600_000-119_250 {
		t.Errorf("receivable wrong: %v", f("411"))
	}

	code, lg, _ := e.do("GET", "/v1/accounting/ledger/411", fin, nil)
	if code != 200 || len(lg["rows"].([]any)) < 4 {
		t.Errorf("ledger: %d %v", code, lg)
	}
}

func TestManualEntriesAndRoles(t *testing.T) {
	e := setup(t)
	fin := token("fin-man", "", "finance")
	good := map[string]any{"journal": "OD", "entry_date": "2026-10-01", "narration": "Capital injection",
		"lines": []map[string]any{{"account": "521", "debit": 5000}, {"account": "101", "credit": 5000}}}
	if code, b, _ := e.do("POST", "/v1/accounting/entries", fin, good); code != 201 || b["hash"] == "" {
		t.Fatalf("manual entry: %d %v", code, b)
	}
	bad := func(mod func(m map[string]any)) int {
		m := map[string]any{"journal": "OD", "entry_date": "2026-10-01", "narration": "x",
			"lines": []map[string]any{{"account": "521", "debit": 10}, {"account": "101", "credit": 10}}}
		mod(m)
		code, _, _ := e.do("POST", "/v1/accounting/entries", fin, m)
		return code
	}
	for name, f := range map[string]func(map[string]any){
		"unbalanced": func(m map[string]any) {
			m["lines"] = []map[string]any{{"account": "521", "debit": 10}, {"account": "101", "credit": 9}}
		},
		"unknown account": func(m map[string]any) {
			m["lines"] = []map[string]any{{"account": "999", "debit": 10}, {"account": "101", "credit": 10}}
		},
		"unknown journal": func(m map[string]any) { m["journal"] = "ZZ" },
		"future date":     func(m map[string]any) { m["entry_date"] = "2099-01-01" },
		"bad date":        func(m map[string]any) { m["entry_date"] = "yesterday" },
	} {
		if c := bad(f); c != 422 {
			t.Errorf("%s: want 422, got %d", name, c)
		}
	}
	// Roles: support has no access; auditor reads but cannot write; sales cannot read.
	if code, _, _ := e.do("GET", "/v1/accounting/trial-balance", token("s", "", "support"), nil); code != 403 {
		t.Errorf("support: want 403, got %d", code)
	}
	if code, _, _ := e.do("GET", "/v1/accounting/trial-balance", token("a", "", "auditor"), nil); code != 200 {
		t.Errorf("auditor read: want 200, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/accounting/entries", token("a", "", "auditor"), good); code != 403 {
		t.Errorf("auditor write: want 403, got %d", code)
	}
	if code, _, _ := e.do("GET", "/v1/accounting/trial-balance", token("c", "", "client_admin"), nil); code != 403 {
		t.Errorf("client: want 403, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/accounting/accounts", fin, map[string]any{"code": "6051", "name": "Fournitures"}); code != 201 {
		t.Errorf("add account: want 201, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/accounting/accounts", fin, map[string]any{"code": "abc", "name": "x"}); code != 422 {
		t.Errorf("bad account code: want 422, got %d", code)
	}
}

func TestLedgerIsAppendOnlyAndBalancedInTheDatabase(t *testing.T) {
	e := setup(t)
	e.do("POST", "/v1/accounting/entries", token("f", "", "finance"), map[string]any{"journal": "OD", "entry_date": "2026-10-01", "narration": "seed",
		"lines": []map[string]any{{"account": "521", "debit": 1}, {"account": "101", "credit": 1}}})
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("HEKATE_TEST_DB"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	for name, sql := range map[string]string{
		"update entry": `UPDATE accounting.entries SET narration='tampered'`,
		"delete entry": `DELETE FROM accounting.entries`,
		"update line":  `UPDATE accounting.entry_lines SET debit = debit + 1 WHERE debit > 0`,
		"delete line":  `DELETE FROM accounting.entry_lines`,
		"truncate":     `TRUNCATE accounting.entry_lines`,
	} {
		if _, err := conn.Exec(ctx, sql); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%s must be rejected as append-only, got %v", name, err)
		}
	}
	// Unbalanced entry fails at commit even for the table owner.
	tx, _ := conn.Begin(ctx)
	var seq int64
	_ = tx.QueryRow(ctx, `INSERT INTO accounting.entries (journal, entry_date, narration, created_by, prev_hash, hash) VALUES ('OD', now(), 'x', 'x', 'p', md5(random()::text)) RETURNING seq`).Scan(&seq)
	_, _ = tx.Exec(ctx, `INSERT INTO accounting.entry_lines (entry_seq, position, account, debit) VALUES ($1,1,'521',10)`, seq)
	_, _ = tx.Exec(ctx, `INSERT INTO accounting.entry_lines (entry_seq, position, account, credit) VALUES ($1,2,'101',9)`, seq)
	if err := tx.Commit(ctx); err == nil || !strings.Contains(err.Error(), "not balanced") {
		t.Errorf("unbalanced commit must fail, got %v", err)
	}
}

func TestHashChainVerifiesAndDetectsTampering(t *testing.T) {
	e := setup(t)
	fin := token("fin-chain", "", "finance")
	for i := 0; i < 3; i++ {
		e.do("POST", "/v1/accounting/entries", fin, map[string]any{"journal": "OD", "entry_date": "2026-10-02", "narration": fmt.Sprint("chain ", i),
			"lines": []map[string]any{{"account": "521", "debit": 7}, {"account": "101", "credit": 7}}})
	}
	if _, v, _ := e.do("GET", "/v1/accounting/verify", fin, nil); v["intact"] != true {
		t.Fatalf("chain should be intact: %v", v)
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("HEKATE_TEST_DB"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	// An attacker with superuser rights who disables the trigger and edits history is still caught.
	var seq int64
	var orig string
	_ = conn.QueryRow(ctx, `SELECT seq, narration FROM accounting.entries WHERE narration LIKE 'chain %' ORDER BY seq LIMIT 1`).Scan(&seq, &orig)
	restore := func() {
		_, _ = conn.Exec(ctx, `ALTER TABLE accounting.entries DISABLE TRIGGER entries_append_only`)
		_, _ = conn.Exec(ctx, `UPDATE accounting.entries SET narration=$2 WHERE seq=$1`, seq, orig)
		_, _ = conn.Exec(ctx, `ALTER TABLE accounting.entries ENABLE TRIGGER entries_append_only`)
	}
	defer restore()
	_, _ = conn.Exec(ctx, `ALTER TABLE accounting.entries DISABLE TRIGGER entries_append_only`)
	_, _ = conn.Exec(ctx, `UPDATE accounting.entries SET narration='rewritten history' WHERE seq=$1`, seq)
	_, _ = conn.Exec(ctx, `ALTER TABLE accounting.entries ENABLE TRIGGER entries_append_only`)
	_, v, _ := e.do("GET", "/v1/accounting/verify", fin, nil)
	if v["intact"] != false || v["first_broken_seq"] != float64(seq) {
		t.Errorf("tampering must be detected at seq %d: %v", seq, v)
	}
	restore()
	if _, v, _ := e.do("GET", "/v1/accounting/verify", fin, nil); v["intact"] != true {
		t.Errorf("chain should verify again after restoring: %v", v)
	}
}

func TestConcurrentPostingKeepsOneChain(t *testing.T) {
	e := setup(t)
	fin := token("fin-conc", "", "finance")
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e.do("POST", "/v1/accounting/entries", fin, map[string]any{"journal": "OD", "entry_date": "2026-10-03", "narration": fmt.Sprint("concurrent ", i),
				"lines": []map[string]any{{"account": "521", "debit": 3}, {"account": "101", "credit": 3}}})
		}(i)
	}
	wg.Wait()
	if _, v, _ := e.do("GET", "/v1/accounting/verify", fin, nil); v["intact"] != true {
		t.Errorf("concurrent postings must still form a single valid chain: %v", v)
	}
}
