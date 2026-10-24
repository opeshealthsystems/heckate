package accounting

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/internal/reports"
)

type printer struct{ docs docs.Options }

func registerReports(a *platform.App, o docs.Options) {
	p := &printer{docs: o}
	a.Route("GET /v1/accounting/statements/pdf", readers, p.statementsPDF)
	a.Route("GET /v1/accounting/trial-balance/pdf", readers, p.trialBalancePDF)
	a.Route("GET /v1/accounting/ledger/{account}/pdf", readers, p.ledgerPDF)
	a.Route("GET /v1/exports/trial-balance", readers, exportTrialBalance)
	a.Route("GET /v1/exports/ledger", readers, exportLedger)
}

func periodText(r *http.Request) string {
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	switch {
	case from != "" && to != "":
		return from + " to " + to
	case from != "":
		return "from " + from
	case to != "":
		return "up to " + to
	}
	return "all entries to date"
}

func num(v any) int64 {
	f, _ := v.(float64)
	return int64(f)
}

func str(v any) string { s, _ := v.(string); return s }

func lineRows(v any) [][]string {
	var out [][]string
	arr, _ := v.([]any)
	for _, x := range arr {
		m, _ := x.(map[string]any)
		out = append(out, []string{str(m["account"]), str(m["name"]), docs.XAF(num(m["amount"]))})
	}
	return out
}

func (p *printer) statementsPDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	res, err := statements(r, s)
	if err != nil {
		return nil, err
	}
	b, err := reports.Body(res)
	if err != nil {
		return nil, err
	}
	inc, _ := b["income_statement"].(map[string]any)
	bs, _ := b["balance_sheet"].(map[string]any)
	cols := []docs.ReportColumn{{Header: "Account", Width: 24}, {Header: "Name"}, {Header: "Amount", Width: 42, Right: true}}
	balanced := "The balance sheet balances."
	if bs["balanced"] != true {
		balanced = "WARNING: assets and liabilities do not agree; do not rely on this sheet."
	}
	pdf, err := docs.RenderReport(p.docs, docs.Report{
		Title: "Financial statements", Subtitle: "Income statement for " + periodText(r) + " and balance sheet as at " + str(bs["as_at"]),
		Class: "CONFIDENTIAL", GeneratedBy: s.Claims.Sub, At: time.Now().UTC(),
		Sections: []docs.ReportSection{
			{Heading: "Income statement: revenues", Columns: cols, Rows: lineRows(inc["revenues"]), Total: []string{"", "Total revenues", docs.XAF(num(inc["total_revenues"]))}, Empty: "No revenue in this period."},
			{Heading: "Income statement: expenses", Columns: cols, Rows: lineRows(inc["expenses"]), Total: []string{"", "Total expenses", docs.XAF(num(inc["total_expenses"]))}, Empty: "No expenses in this period."},
			{Heading: "Result for the period", Columns: cols, Rows: [][]string{{"", "Revenues less expenses", docs.XAF(num(inc["result"]))}}},
			{Heading: "Balance sheet: assets", Columns: cols, Rows: lineRows(bs["assets"]), Total: []string{"", "Total assets", docs.XAF(num(bs["total_assets"]))}},
			{Heading: "Balance sheet: liabilities and equity", Columns: cols, Rows: lineRows(bs["liabilities_and_equity"]), Total: []string{"", "Total liabilities and equity", docs.XAF(num(bs["total_liabilities_and_equity"]))}},
		},
		Note: balanced + " " + str(b["note"]),
	})
	if err != nil {
		return nil, err
	}
	return reports.PDF("financial-statements", pdf), nil
}

func tbRows(rows []map[string]any) [][]string {
	out := make([][]string, 0, len(rows))
	for _, m := range rows {
		out = append(out, []string{str(m["account"]), str(m["name"]), docs.XAF(num(m["debit"])), docs.XAF(num(m["credit"])), docs.XAF(num(m["balance"]))})
	}
	return out
}

func (p *printer) trialBalancePDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	res, err := trialBalance(r, s)
	if err != nil {
		return nil, err
	}
	rows, err := reports.Rows(res, "rows")
	if err != nil {
		return nil, err
	}
	b, _ := reports.Body(res)
	note := "Debits equal credits."
	if b["balanced"] != true {
		note = "WARNING: debits and credits differ."
	}
	pdf, err := docs.RenderReport(p.docs, docs.Report{
		Title: "Trial balance", Subtitle: strings.ToUpper(periodText(r)[:1]) + periodText(r)[1:], Class: "CONFIDENTIAL", GeneratedBy: s.Claims.Sub,
		Sections: []docs.ReportSection{{
			Columns: []docs.ReportColumn{{Header: "Account", Width: 20}, {Header: "Name"}, {Header: "Debit", Width: 36, Right: true}, {Header: "Credit", Width: 36, Right: true}, {Header: "Balance", Width: 36, Right: true}},
			Rows:    tbRows(rows), Total: []string{"", "Totals", docs.XAF(num(b["total_debit"])), docs.XAF(num(b["total_credit"])), ""}, Empty: "No entries in this period.",
		}},
		Note: note,
	})
	if err != nil {
		return nil, err
	}
	return reports.PDF("trial-balance", pdf), nil
}

func (p *printer) ledgerPDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	acct := r.PathValue("account")
	var name string
	if err := s.Tx.QueryRow(r.Context(), `SELECT name FROM accounting.accounts WHERE code = $1`, acct).Scan(&name); err != nil {
		return nil, platform.Err(http.StatusNotFound, "Not found", "no such account")
	}
	res, err := ledger(r, s)
	if err != nil {
		return nil, err
	}
	rows, err := reports.Rows(res, "rows")
	if err != nil {
		return nil, err
	}
	var data [][]string
	for _, m := range rows {
		d, _ := time.Parse(time.RFC3339Nano, str(m["entry_date"]))
		data = append(data, []string{docs.Day(d), str(m["journal"]), str(m["ref"]), str(m["narration"]), docs.XAF(num(m["debit"])), docs.XAF(num(m["credit"])), docs.XAF(num(m["balance"]))})
	}
	b, _ := reports.Body(res)
	pdf, err := docs.RenderReport(p.docs, docs.Report{
		Title: "General ledger", Subtitle: fmt.Sprintf("Account %s, %s", acct, name), Class: "CONFIDENTIAL", GeneratedBy: s.Claims.Sub,
		Sections: []docs.ReportSection{{
			Columns: []docs.ReportColumn{{Header: "Date", Width: 22}, {Header: "Jnl", Width: 12}, {Header: "Ref", Width: 26}, {Header: "Narration"}, {Header: "Debit", Width: 26, Right: true}, {Header: "Credit", Width: 26, Right: true}, {Header: "Balance", Width: 28, Right: true}},
			Rows:    data, Total: []string{"", "", "", "Closing balance", "", "", docs.XAF(num(b["balance"]))}, Empty: "No entries on this account.",
		}},
	})
	if err != nil {
		return nil, err
	}
	return reports.PDF("ledger-"+acct, pdf), nil
}

func exportTrialBalance(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	res, err := trialBalance(r, s)
	if err != nil {
		return nil, err
	}
	rows, err := reports.Rows(res, "rows")
	if err != nil {
		return nil, err
	}
	return reports.CSV("trial-balance", []reports.Col{reports.C("Account", "account"), reports.C("Name", "name"), reports.C("Debit (XAF)", "debit"), reports.C("Credit (XAF)", "credit"), reports.C("Balance (XAF)", "balance")}, rows)
}

// exportLedger lists every ledger line in the period with its entry, for the accountant's own spreadsheets.
func exportLedger(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	from, to, err := period(r)
	if err != nil {
		return nil, err
	}
	q, err := s.Tx.Query(r.Context(), `SELECT e.seq, e.entry_date, e.journal, e.ref, e.narration, e.source, l.account, a.name, l.debit, l.credit, coalesce(l.party::text,''), e.created_by
		FROM accounting.entries e JOIN accounting.entry_lines l ON l.entry_seq = e.seq JOIN accounting.accounts a ON a.code = l.account
		WHERE ($1::date IS NULL OR e.entry_date >= $1) AND ($2::date IS NULL OR e.entry_date <= $2) ORDER BY e.seq, l.position`, from, to)
	if err != nil {
		return nil, err
	}
	defer q.Close()
	var rows []map[string]any
	for q.Next() {
		var seq, debit, credit int64
		var date time.Time
		var journal, ref, narration, source, account, name, party, by string
		if err := q.Scan(&seq, &date, &journal, &ref, &narration, &source, &account, &name, &debit, &credit, &party, &by); err != nil {
			return nil, err
		}
		rows = append(rows, map[string]any{"seq": float64(seq), "date": date.Format("2006-01-02"), "journal": journal, "ref": ref, "narration": narration, "source": source,
			"account": account, "name": name, "debit": float64(debit), "credit": float64(credit), "party": party, "by": by})
	}
	if err := q.Err(); err != nil {
		return nil, err
	}
	return reports.CSV("ledger", []reports.Col{reports.C("Entry", "seq"), reports.C("Date", "date"), reports.C("Journal", "journal"), reports.C("Reference", "ref"), reports.C("Narration", "narration"), reports.C("Source", "source"),
		reports.C("Account", "account"), reports.C("Account name", "name"), reports.C("Debit (XAF)", "debit"), reports.C("Credit (XAF)", "credit"), reports.C("Party", "party"), reports.C("Posted by", "by")}, rows)
}
