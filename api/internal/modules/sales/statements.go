package sales

import (
	"context"
	"net/http"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
	"github.com/opeshealthsystems/heckate/api/internal/modules/clients"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/internal/reports"
)

type stmtRow struct {
	Date        time.Time `json:"date"`
	Kind        string    `json:"kind"`
	Ref         string    `json:"ref"`
	Description string    `json:"description"`
	Debit       int64     `json:"debit"`
	Credit      int64     `json:"credit"`
	Balance     int64     `json:"balance"`
}

type clientStatement struct {
	TenantID string    `json:"tenant_id"`
	Client   string    `json:"client"`
	From     string    `json:"from"`
	To       string    `json:"to"`
	Opening  int64     `json:"opening"`
	Rows     []stmtRow `json:"rows"`
	Closing  int64     `json:"closing"`
	Debits   int64     `json:"total_debits"`
	Credits  int64     `json:"total_credits"`
	Overdue  int64     `json:"overdue"` // what is owed on invoices past their due date, as at the end of the period
}

func registerStatements(a *platform.App, m *module) {
	a.Route("GET /v1/clients/{id}/statement", salesView, m.statement)
	a.Route("GET /v1/clients/{id}/statement/pdf", salesView, m.statementPDF)
	a.Route("GET /v1/reports/ageing/pdf", financeRoles, m.ageingPDF)
	a.Route("GET /v1/exports/invoices", salesView, exportInvoices)
	a.Route("GET /v1/exports/payments", financeRoles, exportPayments)
	a.Route("GET /v1/exports/ageing", financeRoles, exportAgeing)
	a.Route("GET /v1/exports/client-statement/{id}", salesView, m.exportStatement)
}

func dateParam(r *http.Request, k string) (*time.Time, error) {
	v := r.URL.Query().Get(k)
	if v == "" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil, platform.Err(http.StatusBadRequest, "Bad date", k+" must be YYYY-MM-DD")
	}
	return &d, nil
}

// buildStatement lists what a client was billed (issued invoices), paid and credited, with a running balance.
// The opening balance is everything before the first day, so a statement for any period starts from the truth.
func buildStatement(ctx context.Context, s *platform.Scope, tenant string, from, to *time.Time) (*clientStatement, error) {
	var name string
	if err := s.Tx.QueryRow(ctx, `SELECT legal_name FROM clients.accounts WHERE tenant_id::text = $1`, tenant).Scan(&name); err != nil {
		return nil, platform.Err(http.StatusNotFound, "Not found", "client not found")
	}
	rows, err := s.Tx.Query(ctx, `
		WITH x AS (
		  SELECT i.issue_date::timestamptz AS at, 'Invoice' AS kind, COALESCE(i.ref,'') AS ref, i.title AS descr, i.total AS debit, 0::bigint AS credit
		    FROM sales.invoices i WHERE i.tenant_id::text = $1 AND i.status IN ('issued','paid')
		  UNION ALL
		  SELECT p.paid_at, 'Payment', COALESCE(i.ref,''), 'Payment by ' || replace(p.method,'_',' ') || CASE WHEN p.reference <> '' THEN ' (' || p.reference || ')' ELSE '' END, 0::bigint, p.amount
		    FROM sales.payments p JOIN sales.invoices i ON i.id = p.invoice_id WHERE p.tenant_id::text = $1
		  UNION ALL
		  SELECT c.created_at, 'Credit note', c.ref, 'Credit note: ' || c.reason, 0::bigint, c.total
		    FROM sales.credit_notes c WHERE c.tenant_id::text = $1)
		SELECT at, kind, ref, descr, debit, credit FROM x ORDER BY at, kind DESC`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	st := &clientStatement{TenantID: tenant, Client: name, Rows: []stmtRow{}}
	if from != nil {
		st.From = from.Format("2006-01-02")
	}
	if to != nil {
		st.To = to.Format("2006-01-02")
	}
	var bal int64
	for rows.Next() {
		var x stmtRow
		if err := rows.Scan(&x.Date, &x.Kind, &x.Ref, &x.Description, &x.Debit, &x.Credit); err != nil {
			return nil, err
		}
		if to != nil && x.Date.After(to.Add(24*time.Hour-time.Nanosecond)) {
			continue
		}
		bal += x.Debit - x.Credit
		if from != nil && x.Date.Before(*from) {
			st.Opening = bal
			continue
		}
		x.Balance = bal
		st.Debits += x.Debit
		st.Credits += x.Credit
		st.Rows = append(st.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	st.Closing = bal
	asAt := time.Now().UTC().Truncate(24 * time.Hour)
	if to != nil {
		asAt = *to
	}
	_ = s.Tx.QueryRow(ctx, `SELECT COALESCE(sum(GREATEST(i.total - COALESCE((SELECT sum(amount) FROM sales.payments p WHERE p.invoice_id = i.id AND p.paid_at::date <= $2),0)
		- COALESCE((SELECT sum(total) FROM sales.credit_notes c WHERE c.invoice_id = i.id AND c.created_at::date <= $2),0), 0)), 0)
		FROM sales.invoices i WHERE i.tenant_id::text = $1 AND i.status IN ('issued','paid') AND i.issue_date <= $2 AND COALESCE(i.due_date, i.issue_date) < $2`, tenant, asAt).Scan(&st.Overdue)
	return st, nil
}

func (m *module) statement(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	from, err := dateParam(r, "from")
	if err != nil {
		return nil, err
	}
	to, err := dateParam(r, "to")
	if err != nil {
		return nil, err
	}
	st, err := buildStatement(r.Context(), s, r.PathValue("id"), from, to)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: st}, nil
}

func (m *module) statementPDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	from, err := dateParam(r, "from")
	if err != nil {
		return nil, err
	}
	to, err := dateParam(r, "to")
	if err != nil {
		return nil, err
	}
	st, err := buildStatement(r.Context(), s, r.PathValue("id"), from, to)
	if err != nil {
		return nil, err
	}
	party, err := clients.Party(r.Context(), s, st.TenantID)
	if err != nil {
		return nil, err
	}
	data := make([][]string, 0, len(st.Rows)+1)
	if from != nil {
		data = append(data, []string{"", "", "", "Balance brought forward", "", "", docs.XAF(st.Opening)})
	}
	for _, x := range st.Rows {
		deb, cred := "", ""
		if x.Debit != 0 {
			deb = docs.XAF(x.Debit)
		}
		if x.Credit != 0 {
			cred = docs.XAF(x.Credit)
		}
		data = append(data, []string{docs.Day(x.Date), x.Kind, x.Ref, x.Description, deb, cred, docs.XAF(x.Balance)})
	}
	period := "All activity to date"
	switch {
	case from != nil && to != nil:
		period = docs.Day(*from) + " to " + docs.Day(*to)
	case from != nil:
		period = "From " + docs.Day(*from)
	case to != nil:
		period = "Up to " + docs.Day(*to)
	}
	facts := [][2]string{{"Client", party.Name}}
	if party.RCCM != "" {
		facts = append(facts, [2]string{"RCCM", party.RCCM})
	}
	facts = append(facts, [2]string{"Period", period}, [2]string{"Amount due", docs.XAF(st.Closing)})
	if st.Overdue > 0 {
		facts = append(facts, [2]string{"Of which overdue", docs.XAF(st.Overdue)})
	}
	note := "A positive balance is owed to HEKATE SYSTEMS SARL; a negative balance is a credit in the client's favour."
	pdf, err := docs.RenderReport(m.docs, docs.Report{
		Title: "Statement of account", Subtitle: party.Name, Class: "CONFIDENTIAL", GeneratedBy: s.Claims.Sub, Facts: facts,
		Sections: []docs.ReportSection{{
			Columns: []docs.ReportColumn{{Header: "Date", Width: 21}, {Header: "Type", Width: 21}, {Header: "Reference", Width: 33}, {Header: "Description"}, {Header: "Billed", Width: 25, Right: true}, {Header: "Paid / credited", Width: 25, Right: true}, {Header: "Balance", Width: 25, Right: true}},
			Rows:    data, Total: []string{"", "", "", "Closing balance", docs.XAF(st.Debits), docs.XAF(st.Credits), docs.XAF(st.Closing)}, Empty: "No activity in this period.",
		}},
		Note: note,
	})
	if err != nil {
		return nil, err
	}
	return reports.PDF("statement-"+safeName(party.Name), pdf), nil
}

func safeName(n string) string {
	out := make([]rune, 0, len(n))
	for _, c := range n {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			out = append(out, c)
		case len(out) > 0 && out[len(out)-1] != '-':
			out = append(out, '-')
		}
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return string(out)
}

func (m *module) exportStatement(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	from, err := dateParam(r, "from")
	if err != nil {
		return nil, err
	}
	to, err := dateParam(r, "to")
	if err != nil {
		return nil, err
	}
	st, err := buildStatement(r.Context(), s, r.PathValue("id"), from, to)
	if err != nil {
		return nil, err
	}
	res := &platform.Result{Body: map[string]any{"rows": st.Rows}}
	rows, err := reports.Rows(res, "rows")
	if err != nil {
		return nil, err
	}
	return reports.CSV("statement-"+safeName(st.Client), []reports.Col{reports.C("Date", "date"), reports.C("Type", "kind"), reports.C("Reference", "ref"), reports.C("Description", "description"), reports.C("Billed (XAF)", "debit"), reports.C("Paid or credited (XAF)", "credit"), reports.C("Balance (XAF)", "balance")}, rows)
}

func (m *module) ageingPDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	res, err := ageing(r, s)
	if err != nil {
		return nil, err
	}
	rows, err := reports.Rows(res, "clients")
	if err != nil {
		return nil, err
	}
	b, _ := reports.Body(res)
	tot, _ := b["totals"].(map[string]any)
	n := func(v any) string { f, _ := v.(float64); return docs.XAF(int64(f)) }
	data := make([][]string, 0, len(rows))
	for _, c := range rows {
		cn, _ := c["client"].(string)
		data = append(data, []string{cn, n(c["current"]), n(c["d1_30"]), n(c["d31_60"]), n(c["d61_90"]), n(c["over_90"]), n(c["total"])})
	}
	asAt, _ := b["as_at"].(string)
	pdf, err := docs.RenderReport(m.docs, docs.Report{
		Title: "Receivables ageing", Subtitle: "What clients owe, by how far past due, as at " + asAt, Class: "CONFIDENTIAL", GeneratedBy: s.Claims.Sub,
		Sections: []docs.ReportSection{{
			Columns: []docs.ReportColumn{{Header: "Client"}, {Header: "Not yet due", Width: 24, Right: true}, {Header: "1-30 days", Width: 24, Right: true}, {Header: "31-60 days", Width: 24, Right: true}, {Header: "61-90 days", Width: 24, Right: true}, {Header: "Over 90", Width: 24, Right: true}, {Header: "Total", Width: 26, Right: true}},
			Rows:    data, Total: []string{"Total", n(tot["current"]), n(tot["d1_30"]), n(tot["d31_60"]), n(tot["d61_90"]), n(tot["over_90"]), n(tot["total"])}, Empty: "Nothing is owed.",
		}},
	})
	if err != nil {
		return nil, err
	}
	return reports.PDF("receivables-ageing", pdf), nil
}

func exportAgeing(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	res, err := ageing(r, s)
	if err != nil {
		return nil, err
	}
	rows, err := reports.Rows(res, "clients")
	if err != nil {
		return nil, err
	}
	return reports.CSV("receivables-ageing", []reports.Col{reports.C("Client", "client"), reports.C("Not yet due (XAF)", "current"), reports.C("1-30 days (XAF)", "d1_30"), reports.C("31-60 days (XAF)", "d31_60"), reports.C("61-90 days (XAF)", "d61_90"), reports.C("Over 90 days (XAF)", "over_90"), reports.C("Total (XAF)", "total")}, rows)
}

func exportInvoices(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	tc, ta := s.TenantFilter("i.tenant_id", 1)
	q, err := s.Tx.Query(r.Context(), `SELECT `+invCols+` FROM sales.invoices i WHERE true`+tc+` ORDER BY i.issue_date NULLS LAST, i.created_at LIMIT 50000`, ta...)
	if err != nil {
		return nil, err
	}
	defer q.Close()
	out := []*Invoice{}
	for q.Next() {
		v, err := scanInvoice(q)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := q.Err(); err != nil {
		return nil, err
	}
	rows, err := reports.Rows(&platform.Result{Body: map[string]any{"data": out}}, "data")
	if err != nil {
		return nil, err
	}
	return reports.CSV("invoices", []reports.Col{reports.C("Reference", "ref"), reports.C("Title", "title"), reports.C("Status", "status"), reports.C("Client id", "tenant_id"), reports.C("Issued", "issue_date"), reports.C("Due", "due_date"),
		reports.C("Net (XAF)", "net"), reports.C("TVA (XAF)", "tva"), reports.C("CAC (XAF)", "cac"), reports.C("Total (XAF)", "total"), reports.C("Paid (XAF)", "paid"), reports.C("Credited (XAF)", "credited"), reports.C("Balance (XAF)", "balance")}, rows)
}

func exportPayments(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	from, err := dateParam(r, "from")
	if err != nil {
		return nil, err
	}
	to, err := dateParam(r, "to")
	if err != nil {
		return nil, err
	}
	q, err := s.Tx.Query(r.Context(), `SELECT p.paid_at, COALESCE(i.ref,''), a.legal_name, p.amount, p.method, p.reference, p.recorded_by
		FROM sales.payments p JOIN sales.invoices i ON i.id = p.invoice_id JOIN clients.accounts a ON a.tenant_id = p.tenant_id
		WHERE ($1::date IS NULL OR p.paid_at::date >= $1) AND ($2::date IS NULL OR p.paid_at::date <= $2) ORDER BY p.paid_at LIMIT 50000`, from, to)
	if err != nil {
		return nil, err
	}
	defer q.Close()
	var rows []map[string]any
	for q.Next() {
		var at time.Time
		var ref, client, method, reference, by string
		var amount int64
		if err := q.Scan(&at, &ref, &client, &amount, &method, &reference, &by); err != nil {
			return nil, err
		}
		rows = append(rows, map[string]any{"date": at.UTC().Format("2006-01-02"), "invoice": ref, "client": client, "amount": float64(amount), "method": method, "reference": reference, "by": by})
	}
	if err := q.Err(); err != nil {
		return nil, err
	}
	return reports.CSV("payments", []reports.Col{reports.C("Date", "date"), reports.C("Invoice", "invoice"), reports.C("Client", "client"), reports.C("Amount (XAF)", "amount"), reports.C("Method", "method"), reports.C("Reference", "reference"), reports.C("Recorded by", "by")}, rows)
}
