package server

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func csvLines(b []byte) []string {
	s := strings.TrimPrefix(string(b), "\xEF\xBB\xBF")
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

func TestStatementOfAccountAndExports(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	fin := token("rep-fin", "", "finance")
	mgr := token("rep-mgr", "", "manager")
	sales := token("rep-sales", "", "sales")
	ta := e.newClient(token("m", "", "manager"), "Statement Co "+sfx)
	tb := e.newClient(token("m", "", "manager"), "Other Statement Co "+sfx)
	clientA := token("rep-ca", ta, "client_admin")
	clientB := token("rep-cb", tb, "client_admin")

	// 1,000,000 net invoiced to A (with TVA and CAC the total is 1,192,500), 192,500 paid, 119,250 credited (100,000 net).
	_, d, _ := e.do("POST", "/v1/invoices", fin, map[string]any{"tenant_id": ta, "title": "=cmd|' /C calc'!A0 " + sfx, "due_date": time.Now().UTC().AddDate(0, 0, -10).Format("2006-01-02"), "lines": lines(1_000_000)})
	id := d["id"].(string)
	e.do("POST", "/v1/invoices/"+id+"/issue", fin, nil)
	e.do("POST", "/v1/invoices/"+id+"/payments", fin, map[string]any{"amount": 192_500, "method": "bank"})
	e.do("POST", "/v1/invoices/"+id+"/credit-notes", fin, map[string]any{"net": 100_000, "reason": "discount"})

	c, st, _ := e.do("GET", "/v1/clients/"+ta+"/statement", fin, nil)
	if c != 200 || st["closing"] != float64(1_192_500-192_500-119_250) || st["total_debits"] != float64(1_192_500) || st["total_credits"] != float64(311_750) || st["opening"] != float64(0) {
		t.Fatalf("statement: %d %v", c, st)
	}
	rows := st["rows"].([]any)
	if len(rows) != 3 || rows[2].(map[string]any)["balance"] != st["closing"] {
		t.Errorf("an invoice, a payment and a credit note, ending on the closing balance: %v", rows)
	}
	if st["overdue"] != float64(880_750) {
		t.Errorf("the invoice was due 10 days ago and 880,750 is still owed on it: %v", st["overdue"])
	}
	// From tomorrow the opening balance carries everything, and the statement is empty.
	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	_, st2, _ := e.do("GET", "/v1/clients/"+ta+"/statement?from="+tomorrow, fin, nil)
	if st2["opening"] != st["closing"] || len(st2["rows"].([]any)) != 0 || st2["closing"] != st["closing"] {
		t.Errorf("a later period starts from the balance brought forward: %v", st2)
	}
	if c, _, _ := e.do("GET", "/v1/clients/"+ta+"/statement?from=nope", fin, nil); c != 400 {
		t.Errorf("bad date: want 400, got %d", c)
	}

	// The PDF: named, formatted, and not a numbered document.
	r, pdf := e.pdf(fin, "/v1/clients/"+ta+"/statement/pdf")
	body := string(pdf)
	if r.StatusCode != 200 || !strings.HasPrefix(body, "%PDF") || !strings.Contains(r.Header.Get("Content-Disposition"), "statement-Statement-Co") {
		t.Fatalf("statement pdf: %d %s", r.StatusCode, r.Header.Get("Content-Disposition"))
	}
	for _, want := range []string{"STATEMENT OF ACCOUNT", "Statement Co " + sfx, "880 750 XAF", "Closing balance", "CONFIDENTIAL", "not a numbered document"} {
		if !strings.Contains(body, want) {
			t.Errorf("the statement must show %q", want)
		}
	}
	if strings.Contains(body, "Scan to verify") {
		t.Error("a statement is not a verifiable numbered document and must not carry a QR code")
	}

	// A client sees only their own statement; staff outside finance and sales do not see it at all.
	if c, _, _ := e.do("GET", "/v1/clients/"+ta+"/statement", clientA, nil); c != 200 {
		t.Errorf("a client reading their own statement: got %d", c)
	}
	if c, _, _ := e.do("GET", "/v1/clients/"+ta+"/statement", clientB, nil); c != 404 && c != 403 {
		t.Errorf("a client reading another client's statement: want 404 or 403, got %d", c)
	}
	if r, _ := e.pdf(clientB, "/v1/clients/"+ta+"/statement/pdf"); r.StatusCode != 404 && r.StatusCode != 403 {
		t.Errorf("another client's statement pdf: got %d", r.StatusCode)
	}
	if c, _, _ := e.do("GET", "/v1/clients/"+ta+"/statement", token("rep-eng", "", "engineer"), nil); c != 403 {
		t.Errorf("an engineer reading a statement: want 403, got %d", c)
	}

	// CSV exports: Excel-friendly, whole-number amounts, and a cell that looks like a formula is neutralised.
	r, inv := e.pdf(fin, "/v1/exports/invoices")
	lines2 := csvLines(inv)
	if r.StatusCode != 200 || !strings.HasPrefix(r.Header.Get("Content-Type"), "text/csv") || !strings.HasPrefix(string(inv), "\xEF\xBB\xBFReference,Title,Status") {
		t.Fatalf("invoices csv: %d %s %q", r.StatusCode, r.Header.Get("Content-Type"), lines2[0])
	}
	found := false
	for _, l := range lines2 {
		if strings.Contains(l, sfx) {
			found = true
			if strings.Contains(l, ",=cmd") || strings.Contains(l, ",\"=cmd") || !strings.Contains(l, "'=cmd") || !strings.Contains(l, "1192500") {
				t.Errorf("formula neutralised and amounts plain: %s", l)
			}
		}
	}
	if !found {
		t.Error("the invoice is missing from the export")
	}
	_, mine := e.pdf(clientB, "/v1/exports/invoices")
	if strings.Contains(string(mine), sfx) {
		t.Error("a client's invoice export must contain only their own invoices")
	}
	r, pay := e.pdf(fin, "/v1/exports/payments")
	if r.StatusCode != 200 || !strings.Contains(string(pay), "192500") {
		t.Errorf("payments csv: %d", r.StatusCode)
	}
	if r, _ := e.pdf(sales, "/v1/exports/payments"); r.StatusCode != 403 {
		t.Errorf("sales exporting payments: want 403, got %d", r.StatusCode)
	}
	if r, _ := e.pdf(clientA, "/v1/exports/ageing"); r.StatusCode != 403 {
		t.Errorf("a client exporting ageing: want 403, got %d", r.StatusCode)
	}
	r, ag := e.pdf(fin, "/v1/exports/ageing")
	if r.StatusCode != 200 || !strings.Contains(string(ag), "Statement Co "+sfx) {
		t.Errorf("ageing csv: %d", r.StatusCode)
	}
	r, agp := e.pdf(fin, "/v1/reports/ageing/pdf")
	if r.StatusCode != 200 || !strings.Contains(string(agp), "RECEIVABLES AGEING") || !strings.Contains(string(agp), "Statement Co") {
		t.Errorf("ageing pdf: %d", r.StatusCode)
	}

	// Accounting printouts and exports for the people who read the books; sales cannot.
	for path, want := range map[string]string{
		"/v1/accounting/trial-balance/pdf": "TRIAL BALANCE", "/v1/accounting/statements/pdf": "FINANCIAL STATEMENTS", "/v1/accounting/ledger/411/pdf": "GENERAL LEDGER",
	} {
		r, b := e.pdf(mgr, path)
		if r.StatusCode != 200 || !strings.Contains(string(b), want) || !strings.Contains(string(b), "CONFIDENTIAL") {
			t.Errorf("%s: %d", path, r.StatusCode)
		}
		if r, _ := e.pdf(sales, path); r.StatusCode != 403 {
			t.Errorf("sales on %s: want 403, got %d", path, r.StatusCode)
		}
	}
	if r, _ := e.pdf(mgr, "/v1/accounting/ledger/999999/pdf"); r.StatusCode != http.StatusNotFound {
		t.Errorf("a ledger for an unknown account: want 404, got %d", r.StatusCode)
	}
	for _, path := range []string{"/v1/exports/ledger", "/v1/exports/trial-balance"} {
		r, b := e.pdf(token("rep-aud", "", "auditor"), path)
		if r.StatusCode != 200 || !strings.HasPrefix(string(b), "\xEF\xBB\xBF") {
			t.Errorf("the auditor exporting %s: %d", path, r.StatusCode)
		}
		if r, _ := e.pdf(sales, path); r.StatusCode != 403 {
			t.Errorf("sales on %s: want 403, got %d", path, r.StatusCode)
		}
	}
	if _, led := e.pdf(mgr, "/v1/exports/ledger"); !strings.Contains(string(led), "Debit (XAF)") || len(csvLines(led)) < 2 {
		t.Error("the ledger export must list the posted lines")
	}

	// Operational exports.
	for _, path := range []string{"/v1/exports/stock", "/v1/exports/assets", "/v1/exports/expense-claims", "/v1/exports/timesheets"} {
		r, b := e.pdf(mgr, path)
		if r.StatusCode != 200 || !strings.HasPrefix(string(b), "\xEF\xBB\xBF") {
			t.Errorf("%s: %d", path, r.StatusCode)
		}
		if r, _ := e.pdf("", path); r.StatusCode != 401 {
			t.Errorf("anonymous %s: want 401, got %d", path, r.StatusCode)
		}
	}
	if r, _ := e.pdf(clientA, "/v1/exports/stock"); r.StatusCode != 403 {
		t.Errorf("a client exporting stock: want 403, got %d", r.StatusCode)
	}
}
