// Package sales is the Sales & Invoicing context: quotations, invoices, payments and credit notes.
package sales

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
	"github.com/opeshealthsystems/heckate/api/internal/modules/clients"
	"github.com/opeshealthsystems/heckate/api/internal/modules/documents"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// Poster receives accounting entries inside the request transaction. Accounting implements it;
// a nil Poster is allowed so the module works without the ledger.
type Poster interface {
	InvoiceIssued(ctx context.Context, s *platform.Scope, e InvoiceEntry) error
	PaymentReceived(ctx context.Context, s *platform.Scope, e PaymentEntry) error
	CreditNoteIssued(ctx context.Context, s *platform.Scope, e CreditEntry) error
}

type InvoiceEntry struct {
	Ref, Tenant          string
	Net, TVA, CAC, Total int64
	Date                 time.Time
}
type PaymentEntry struct {
	InvoiceRef, Tenant, Method string
	Amount                     int64
	Date                       time.Time
}
type CreditEntry struct {
	Ref, InvoiceRef, Tenant string
	Net, TVA, CAC, Total    int64
	Date                    time.Time
}

type module struct {
	post Poster
	docs docs.Options
}

var (
	// salesView may read quotes, invoices and their documents: the people who handle money, and the client themselves.
	salesView      = platform.Join([]string{"ceo", "manager", "finance", "sales"}, platform.ClientAll)
	quoteWriters   = []string{"ceo", "manager", "sales"}
	invoiceWriters = []string{"ceo", "manager", "finance", "sales"}
	financeRoles   = []string{"ceo", "manager", "finance"}
	approvers      = []string{"ceo", "manager"}
)

// Recorder lets other contexts (online payments) settle invoices without going through HTTP.
type Recorder interface {
	// Outstanding returns an invoice's status and unpaid balance.
	Outstanding(ctx context.Context, s *platform.Scope, invoiceID string) (status string, balance int64, tenant string, err error)
	// RecordPayment books a payment on an issued invoice, posting it to the ledger like a manual one.
	RecordPayment(ctx context.Context, s *platform.Scope, invoiceID string, amount int64, method, reference string) error
}

func Register(a *platform.App, p Poster, o docs.Options) Recorder {
	m := &module{post: p, docs: o}
	a.Route("POST /v1/quotes", quoteWriters, platform.Idempotent(m.createQuote))
	a.Route("GET /v1/quotes", salesView, m.listQuotes)
	a.Route("GET /v1/quotes/{id}", salesView, m.getQuote)
	a.Route("GET /v1/quotes/{id}/pdf", salesView, m.quotePDF)
	a.Route("PUT /v1/quotes/{id}/lines", quoteWriters, m.replaceQuoteLines)
	a.Route("POST /v1/quotes/{id}/send", quoteWriters, platform.Idempotent(m.sendQuote))
	a.Route("POST /v1/quotes/{id}/approve", []string{"client_admin"}, platform.Idempotent(m.decideQuote("approved")))
	a.Route("POST /v1/quotes/{id}/reject", []string{"client_admin"}, platform.Idempotent(m.decideQuote("rejected")))
	a.Route("POST /v1/quotes/{id}/invoice", invoiceWriters, platform.Idempotent(m.invoiceFromQuote))

	a.Route("POST /v1/invoices", invoiceWriters, platform.Idempotent(m.createInvoice))
	a.Route("GET /v1/invoices", salesView, m.listInvoices)
	a.Route("GET /v1/invoices/{id}", salesView, m.getInvoice)
	a.Route("GET /v1/invoices/{id}/pdf", salesView, m.invoicePDF)
	a.Route("GET /v1/invoices/{id}/payments", salesView, m.listPayments)
	a.Route("GET /v1/payments/{id}/receipt", salesView, m.receiptPDF)
	a.Route("GET /v1/credit-notes/{ref}/pdf", salesView, m.creditNotePDF)
	a.Route("POST /v1/invoices/{id}/issue", financeRoles, platform.Idempotent(m.issueInvoice))
	a.Route("POST /v1/invoices/{id}/approve", approvers, platform.Idempotent(m.approveInvoice))
	a.Route("POST /v1/invoices/{id}/payments", financeRoles, platform.Idempotent(m.recordPayment))
	a.Route("POST /v1/invoices/{id}/credit-notes", financeRoles, platform.Idempotent(m.creditNote))
	a.Route("GET /v1/reports/sales", financeRoles, report)
	a.Route("GET /v1/reports/ageing", financeRoles, ageing)
	a.Route("POST /v1/recurring-invoices", invoiceWriters, platform.Idempotent(m.createRecurring))
	a.Route("GET /v1/recurring-invoices", invoiceWriters, m.listRecurring)
	a.Route("GET /v1/invoice-reminders", financeRoles, m.listReminders)
	a.Route("POST /v1/invoice-reminders/run", financeRoles, platform.Idempotent(m.runReminders))
	a.Route("POST /v1/recurring-invoices/run", financeRoles, platform.Idempotent(m.runRecurring))
	a.Route("POST /v1/recurring-invoices/{id}/active", invoiceWriters, m.setRecurringActive)
	return m
}

type settings struct {
	TVABP, CACPct int
	Threshold     int64
}

func loadSettings(ctx context.Context, s *platform.Scope) (settings, error) {
	var st settings
	err := s.Tx.QueryRow(ctx, `SELECT tva_bp, cac_pct_of_tva, approval_threshold FROM sales.settings`).Scan(&st.TVABP, &st.CACPct, &st.Threshold)
	return st, err
}

func validLines(lines []Line) error {
	if len(lines) == 0 || len(lines) > 200 {
		return platform.Err(http.StatusUnprocessableEntity, "Invalid lines", "between 1 and 200 lines are required")
	}
	for _, l := range lines {
		q := qty100(l.Quantity)
		if strings.TrimSpace(l.Description) == "" || q <= 0 || l.UnitPrice < 0 || l.UnitPrice > 1_000_000_000_000 || l.Quantity > 1_000_000 {
			return platform.Err(http.StatusUnprocessableEntity, "Invalid lines", "each line needs a description, a positive quantity and a non-negative unit price in XAF")
		}
	}
	return nil
}

func notFound(what string) error {
	return platform.Err(http.StatusNotFound, "Not found", what+" not found")
}

// ---- quotes ----

type Quote struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id"`
	Ref        *string    `json:"ref"`
	Title      string     `json:"title"`
	Status     string     `json:"status"`
	ValidUntil *time.Time `json:"valid_until"`
	Notes      string     `json:"notes"`
	CreatedBy  string     `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	Lines      []Line     `json:"lines,omitempty"`
	Net        int64      `json:"net"`
	TVA        int64      `json:"tva"`
	CAC        int64      `json:"cac"`
	Total      int64      `json:"total"`
}

const quoteCols = `id, tenant_id, ref, title, status, valid_until, notes, created_by, created_at`

func scanQuote(row pgx.Row) (*Quote, error) {
	var q Quote
	err := row.Scan(&q.ID, &q.TenantID, &q.Ref, &q.Title, &q.Status, &q.ValidUntil, &q.Notes, &q.CreatedBy, &q.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) || (err != nil && strings.Contains(err.Error(), "invalid input syntax")) {
		return nil, notFound("quote")
	}
	return &q, err
}

func readLines(ctx context.Context, s *platform.Scope, table, fk, id string) ([]Line, error) {
	rows, err := s.Tx.Query(ctx, `SELECT description, quantity::float8, unit_price FROM sales.`+table+` WHERE `+fk+` = $1 ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Line{}
	for rows.Next() {
		var l Line
		if err := rows.Scan(&l.Description, &l.Quantity, &l.UnitPrice); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func writeLines(ctx context.Context, s *platform.Scope, table, fk, id, tenant string, lines []Line) error {
	if _, err := s.Tx.Exec(ctx, `DELETE FROM sales.`+table+` WHERE `+fk+` = $1`, id); err != nil {
		return err
	}
	for i, l := range lines {
		if _, err := s.Tx.Exec(ctx, `INSERT INTO sales.`+table+` (`+fk+`, tenant_id, position, description, quantity, unit_price) VALUES ($1,$2,$3,$4,$5,$6)`,
			id, tenant, i+1, strings.TrimSpace(l.Description), float64(qty100(l.Quantity))/100, l.UnitPrice); err != nil {
			return err
		}
	}
	return nil
}

func fillQuote(ctx context.Context, s *platform.Scope, q *Quote, withLines bool) error {
	lines, err := readLines(ctx, s, "quote_lines", "quote_id", q.ID)
	if err != nil {
		return err
	}
	st, err := loadSettings(ctx, s)
	if err != nil {
		return err
	}
	q.Net, q.TVA, q.CAC, q.Total = Totals(lines, st.TVABP, st.CACPct)
	if withLines {
		q.Lines = lines
	}
	return nil
}

func (m *module) createQuote(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		TenantID   string `json:"tenant_id"`
		Title      string `json:"title"`
		ValidUntil string `json:"valid_until"`
		Notes      string `json:"notes"`
		Lines      []Line `json:"lines"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Title) == "" || in.TenantID == "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid quote", "tenant_id and title are required")
	}
	if err := validLines(in.Lines); err != nil {
		return nil, err
	}
	var valid any
	if in.ValidUntil != "" {
		d, err := time.Parse("2006-01-02", in.ValidUntil)
		if err != nil {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid quote", "valid_until must be YYYY-MM-DD")
		}
		valid = d
	}
	ctx := r.Context()
	q, err := scanQuote(s.Tx.QueryRow(ctx, `INSERT INTO sales.quotes (tenant_id, title, valid_until, notes, created_by) VALUES ($1,$2,$3,$4,$5) RETURNING `+quoteCols,
		in.TenantID, strings.TrimSpace(in.Title), valid, in.Notes, s.Claims.Sub))
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid quote", "unknown tenant")
		}
		return nil, err
	}
	if err := writeLines(ctx, s, "quote_lines", "quote_id", q.ID, q.TenantID, in.Lines); err != nil {
		return nil, err
	}
	if err := fillQuote(ctx, s, q, true); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "create", "quote", q.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: q}, nil
}

func (m *module) listQuotes(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	tc, ta := s.TenantFilter("tenant_id", 1)
	rows, err := s.Tx.Query(r.Context(), `SELECT `+quoteCols+` FROM sales.quotes WHERE true`+tc+` ORDER BY created_at DESC LIMIT 200`, ta...)
	if err != nil {
		return nil, err
	}
	var qs []*Quote
	for rows.Next() {
		q, err := scanQuote(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		qs = append(qs, q)
	}
	rows.Close()
	for _, q := range qs {
		if err := fillQuote(r.Context(), s, q, false); err != nil {
			return nil, err
		}
	}
	if qs == nil {
		qs = []*Quote{}
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": qs}}, nil
}

func (m *module) getQuote(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	q, err := scanQuote(s.Tx.QueryRow(r.Context(), `SELECT `+quoteCols+` FROM sales.quotes WHERE id::text = $1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	if err := fillQuote(r.Context(), s, q, true); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: q}, nil
}

func lockQuote(ctx context.Context, s *platform.Scope, id string) (*Quote, error) {
	return scanQuote(s.Tx.QueryRow(ctx, `SELECT `+quoteCols+` FROM sales.quotes WHERE id::text = $1 FOR UPDATE`, id))
}

func (m *module) replaceQuoteLines(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Lines []Line `json:"lines"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if err := validLines(in.Lines); err != nil {
		return nil, err
	}
	ctx := r.Context()
	q, err := lockQuote(ctx, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if q.Status != "draft" {
		return nil, platform.Err(http.StatusConflict, "Quote is locked", "only draft quotes can be edited")
	}
	if err := writeLines(ctx, s, "quote_lines", "quote_id", q.ID, q.TenantID, in.Lines); err != nil {
		return nil, err
	}
	if err := fillQuote(ctx, s, q, true); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: q}, nil
}

func (m *module) sendQuote(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	q, err := lockQuote(ctx, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if q.Status != "draft" {
		return nil, platform.Err(http.StatusConflict, "Cannot send", "only a draft quote can be sent")
	}
	doc, err := documents.Issue(ctx, s, "QT", "Quotation: "+q.Title, q.TenantID, "")
	if err != nil {
		return nil, err
	}
	q, err = scanQuote(s.Tx.QueryRow(ctx, `UPDATE sales.quotes SET status='sent', ref=$2 WHERE id=$1 RETURNING `+quoteCols, q.ID, doc.Ref))
	if err != nil {
		return nil, err
	}
	if err := fillQuote(ctx, s, q, true); err != nil {
		return nil, err
	}
	if err := s.Emit(ctx, "quote.sent", map[string]any{"id": q.ID, "tenant_id": q.TenantID, "ref": doc.Ref}); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "send", "quote", q.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: q}, nil
}

func (m *module) decideQuote(to string) platform.HandlerFunc {
	return func(r *http.Request, s *platform.Scope) (*platform.Result, error) {
		ctx := r.Context()
		q, err := lockQuote(ctx, s, r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		if q.Status != "sent" {
			return nil, platform.Err(http.StatusConflict, "Cannot decide", "only a sent quote can be approved or rejected")
		}
		if q.TenantID != s.Claims.TenantID {
			return nil, notFound("quote")
		}
		q, err = scanQuote(s.Tx.QueryRow(ctx, `UPDATE sales.quotes SET status=$2, decided_by=$3 WHERE id=$1 RETURNING `+quoteCols, q.ID, to, s.Claims.Sub))
		if err != nil {
			return nil, err
		}
		if err := fillQuote(ctx, s, q, true); err != nil {
			return nil, err
		}
		if err := s.Emit(ctx, "quote."+to, map[string]any{"id": q.ID, "tenant_id": q.TenantID}); err != nil {
			return nil, err
		}
		if err := s.Audit(ctx, to, "quote", q.ID); err != nil {
			return nil, err
		}
		return &platform.Result{Status: http.StatusOK, Body: q}, nil
	}
}

// ---- invoices ----

type Invoice struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	QuoteID     *string    `json:"quote_id"`
	Ref         *string    `json:"ref"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	IssueDate   *time.Time `json:"issue_date"`
	DueDate     *time.Time `json:"due_date"`
	Net         int64      `json:"net"`
	TVA         int64      `json:"tva"`
	CAC         int64      `json:"cac"`
	Total       int64      `json:"total"`
	Paid        int64      `json:"paid"`
	Credited    int64      `json:"credited"`
	Balance     int64      `json:"balance"`
	CreatedBy   string     `json:"created_by"`
	RequestedBy *string    `json:"requested_by"`
	ApprovedBy  *string    `json:"approved_by"`
	CreatedAt   time.Time  `json:"created_at"`
	Lines       []Line     `json:"lines,omitempty"`
}

const invCols = `i.id, i.tenant_id, i.quote_id, i.ref, i.title, i.status, i.issue_date, i.due_date, i.net, i.tva, i.cac, i.total,
 COALESCE((SELECT sum(amount) FROM sales.payments p WHERE p.invoice_id = i.id),0),
 COALESCE((SELECT sum(total) FROM sales.credit_notes c WHERE c.invoice_id = i.id),0),
 i.created_by, i.requested_by, i.approved_by, i.created_at`

func scanInvoice(row pgx.Row) (*Invoice, error) {
	var v Invoice
	err := row.Scan(&v.ID, &v.TenantID, &v.QuoteID, &v.Ref, &v.Title, &v.Status, &v.IssueDate, &v.DueDate, &v.Net, &v.TVA, &v.CAC, &v.Total,
		&v.Paid, &v.Credited, &v.CreatedBy, &v.RequestedBy, &v.ApprovedBy, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) || (err != nil && strings.Contains(err.Error(), "invalid input syntax")) {
		return nil, notFound("invoice")
	}
	v.Balance = v.Total - v.Paid - v.Credited
	return &v, err
}

func loadInvoice(ctx context.Context, s *platform.Scope, id string, lock bool, withLines bool) (*Invoice, error) {
	sql := `SELECT ` + invCols + ` FROM sales.invoices i WHERE i.id::text = $1`
	if lock {
		sql += ` FOR UPDATE OF i`
	}
	v, err := scanInvoice(s.Tx.QueryRow(ctx, sql, id))
	if err != nil {
		return nil, err
	}
	if withLines {
		if v.Lines, err = readLines(ctx, s, "invoice_lines", "invoice_id", v.ID); err != nil {
			return nil, err
		}
	}
	return v, nil
}

// retotal recomputes a draft invoice's figures from its lines.
func retotal(ctx context.Context, s *platform.Scope, id string) error {
	lines, err := readLines(ctx, s, "invoice_lines", "invoice_id", id)
	if err != nil {
		return err
	}
	st, err := loadSettings(ctx, s)
	if err != nil {
		return err
	}
	net, tva, cac, total := Totals(lines, st.TVABP, st.CACPct)
	_, err = s.Tx.Exec(ctx, `UPDATE sales.invoices SET net=$2, tva=$3, cac=$4, total=$5 WHERE id=$1`, id, net, tva, cac, total)
	return err
}

func (m *module) createInvoice(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		TenantID string `json:"tenant_id"`
		Title    string `json:"title"`
		DueDate  string `json:"due_date"`
		Lines    []Line `json:"lines"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Title) == "" || in.TenantID == "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid invoice", "tenant_id and title are required")
	}
	if err := validLines(in.Lines); err != nil {
		return nil, err
	}
	due, err := parseDue(in.DueDate)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	id, err := newDraftInvoice(ctx, s, in.TenantID, strings.TrimSpace(in.Title), due, in.Lines)
	if err != nil {
		return nil, err
	}
	v, err := loadInvoice(ctx, s, id, false, true)
	if err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "create", "invoice", id); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: v}, nil
}

// newDraftInvoice inserts a draft invoice with its lines and totals and returns its id.
func newDraftInvoice(ctx context.Context, s *platform.Scope, tenantID, title string, due any, lines []Line) (string, error) {
	var id string
	if err := s.Tx.QueryRow(ctx, `INSERT INTO sales.invoices (tenant_id, title, due_date, created_by) VALUES ($1,$2,$3,$4) RETURNING id`,
		tenantID, title, due, s.Claims.Sub).Scan(&id); err != nil {
		if strings.Contains(err.Error(), "foreign key") {
			return "", platform.Err(http.StatusUnprocessableEntity, "Invalid invoice", "unknown tenant")
		}
		return "", err
	}
	if err := writeLines(ctx, s, "invoice_lines", "invoice_id", id, tenantID, lines); err != nil {
		return "", err
	}
	if err := retotal(ctx, s, id); err != nil {
		return "", err
	}
	return id, nil
}

func parseDue(v string) (any, error) {
	if v == "" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid invoice", "due_date must be YYYY-MM-DD")
	}
	return d, nil
}

func (m *module) invoiceFromQuote(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	q, err := lockQuote(ctx, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if q.Status != "approved" {
		return nil, platform.Err(http.StatusConflict, "Cannot invoice", "the client must approve the quote first")
	}
	var exists bool
	if err := s.Tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sales.invoices WHERE quote_id = $1)`, q.ID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, platform.Err(http.StatusConflict, "Already invoiced", "an invoice already exists for this quote")
	}
	lines, err := readLines(ctx, s, "quote_lines", "quote_id", q.ID)
	if err != nil {
		return nil, err
	}
	var id string
	if err := s.Tx.QueryRow(ctx, `INSERT INTO sales.invoices (tenant_id, quote_id, title, due_date, created_by) VALUES ($1,$2,$3,(now() + interval '30 days')::date,$4) RETURNING id`,
		q.TenantID, q.ID, q.Title, s.Claims.Sub).Scan(&id); err != nil {
		return nil, err
	}
	if err := writeLines(ctx, s, "invoice_lines", "invoice_id", id, q.TenantID, lines); err != nil {
		return nil, err
	}
	if err := retotal(ctx, s, id); err != nil {
		return nil, err
	}
	v, err := loadInvoice(ctx, s, id, false, true)
	if err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "create", "invoice", id); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: v}, nil
}

func (m *module) listInvoices(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	tc, ta := s.TenantFilter("i.tenant_id", 1)
	rows, err := s.Tx.Query(r.Context(), `SELECT `+invCols+` FROM sales.invoices i WHERE true`+tc+` ORDER BY i.created_at DESC LIMIT 200`, ta...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Invoice{}
	for rows.Next() {
		v, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func (m *module) getInvoice(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	v, err := loadInvoice(r.Context(), s, r.PathValue("id"), false, true)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: v}, nil
}

// issue finalises the invoice: allocates its reference, freezes it and posts it to the ledger.
func (m *module) finalize(ctx context.Context, s *platform.Scope, v *Invoice, approver *string) (*Invoice, error) {
	if v.Total <= 0 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Cannot issue", "an invoice with a zero total cannot be issued")
	}
	doc, err := documents.Issue(ctx, s, "INV", "Invoice: "+v.Title, v.TenantID, "")
	if err != nil {
		return nil, err
	}
	out, err := scanInvoice(s.Tx.QueryRow(ctx, `UPDATE sales.invoices i SET status='issued', ref=$2, issue_date=now()::date, approved_by=COALESCE($3, approved_by)
		WHERE i.id=$1 RETURNING `+invCols, v.ID, doc.Ref, approver))
	if err != nil {
		return nil, err
	}
	if m.post != nil {
		if err := m.post.InvoiceIssued(ctx, s, InvoiceEntry{Ref: doc.Ref, Tenant: out.TenantID, Net: out.Net, TVA: out.TVA, CAC: out.CAC, Total: out.Total, Date: *out.IssueDate}); err != nil {
			return nil, err
		}
	}
	if err := s.Emit(ctx, "invoice.issued", map[string]any{"id": out.ID, "tenant_id": out.TenantID, "ref": doc.Ref, "total": out.Total}); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "issue", "invoice", out.ID); err != nil {
		return nil, err
	}
	out.Lines, err = readLines(ctx, s, "invoice_lines", "invoice_id", out.ID)
	return out, err
}

func (m *module) issueInvoice(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	v, err := loadInvoice(ctx, s, r.PathValue("id"), true, false)
	if err != nil {
		return nil, err
	}
	if v.Status != "draft" {
		return nil, platform.Err(http.StatusConflict, "Cannot issue", "only a draft invoice can be issued")
	}
	st, err := loadSettings(ctx, s)
	if err != nil {
		return nil, err
	}
	if v.Total >= st.Threshold {
		// Dual control: a second person must approve before an invoice of this size goes out.
		nv, err := scanInvoice(s.Tx.QueryRow(ctx, `UPDATE sales.invoices i SET status='pending_approval', requested_by=$2 WHERE i.id=$1 RETURNING `+invCols, v.ID, s.Claims.Sub))
		if err != nil {
			return nil, err
		}
		if err := s.Emit(ctx, "invoice.approval_requested", map[string]any{"id": nv.ID, "total": nv.Total}); err != nil {
			return nil, err
		}
		if err := s.Audit(ctx, "request_approval", "invoice", nv.ID); err != nil {
			return nil, err
		}
		return &platform.Result{Status: http.StatusAccepted, Body: nv}, nil
	}
	out, err := m.finalize(ctx, s, v, nil)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}

func (m *module) approveInvoice(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	v, err := loadInvoice(ctx, s, r.PathValue("id"), true, false)
	if err != nil {
		return nil, err
	}
	if v.Status != "pending_approval" {
		return nil, platform.Err(http.StatusConflict, "Cannot approve", "the invoice is not waiting for approval")
	}
	if v.RequestedBy != nil && *v.RequestedBy == s.Claims.Sub || v.CreatedBy == s.Claims.Sub {
		return nil, platform.Err(http.StatusForbidden, "Second approver required", "the person who prepared or requested this invoice cannot approve it")
	}
	sub := s.Claims.Sub
	out, err := m.finalize(ctx, s, v, &sub)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}

var methods = map[string]bool{"momo": true, "orange_money": true, "bank": true, "card": true, "cash": true}

func settle(ctx context.Context, s *platform.Scope, id string) (*Invoice, error) {
	v, err := loadInvoice(ctx, s, id, false, false)
	if err != nil {
		return nil, err
	}
	if v.Status == "issued" && v.Balance <= 0 {
		if _, err := s.Tx.Exec(ctx, `UPDATE sales.invoices SET status='paid' WHERE id=$1`, id); err != nil {
			return nil, err
		}
		if err := s.Emit(ctx, "invoice.paid", map[string]any{"id": id, "tenant_id": v.TenantID}); err != nil {
			return nil, err
		}
		return loadInvoice(ctx, s, id, false, false)
	}
	return v, nil
}

func (m *module) recordPayment(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Amount    int64  `json:"amount"`
		Method    string `json:"method"`
		Reference string `json:"reference"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.Amount <= 0 || !methods[in.Method] {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid payment", "amount must be positive and method one of momo, orange_money, bank, card, cash")
	}
	out, err := m.applyPayment(r.Context(), s, r.PathValue("id"), in.Amount, in.Method, in.Reference)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: out}, nil
}

func (m *module) Outstanding(ctx context.Context, s *platform.Scope, id string) (string, int64, string, error) {
	v, err := loadInvoice(ctx, s, id, false, false)
	if err != nil {
		return "", 0, "", err
	}
	return v.Status, v.Balance, v.TenantID, nil
}

func (m *module) RecordPayment(ctx context.Context, s *platform.Scope, id string, amount int64, method, reference string) error {
	if !methods[method] {
		return platform.Err(http.StatusUnprocessableEntity, "Invalid payment", "unknown payment method")
	}
	_, err := m.applyPayment(ctx, s, id, amount, method, reference)
	return err
}

// applyPayment is the one place a payment is recorded, whoever initiated it.
func (m *module) applyPayment(ctx context.Context, s *platform.Scope, id string, amount int64, method, reference string) (*Invoice, error) {
	v, err := loadInvoice(ctx, s, id, true, false)
	if err != nil {
		return nil, err
	}
	if v.Status != "issued" {
		return nil, platform.Err(http.StatusConflict, "Cannot record payment", "payments can only be recorded against an issued, unpaid invoice")
	}
	if amount > v.Balance {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Overpayment", "amount exceeds the outstanding balance")
	}
	rct, err := documents.Issue(ctx, s, "RCT", "Receipt for "+*v.Ref, v.TenantID, "")
	if err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `INSERT INTO sales.payments (invoice_id, tenant_id, amount, method, reference, recorded_by, receipt_ref) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		v.ID, v.TenantID, amount, method, reference, s.Claims.Sub, rct.Ref); err != nil {
		return nil, err
	}
	if m.post != nil {
		if err := m.post.PaymentReceived(ctx, s, PaymentEntry{InvoiceRef: *v.Ref, Tenant: v.TenantID, Method: method, Amount: amount, Date: time.Now().UTC()}); err != nil {
			return nil, err
		}
	}
	if err := s.Emit(ctx, "payment.recorded", map[string]any{"invoice_id": v.ID, "amount": amount, "method": method}); err != nil {
		return nil, err
	}
	out, err := settle(ctx, s, v.ID)
	if err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "payment", "invoice", v.ID); err != nil {
		return nil, err
	}
	return out, nil
}

func (m *module) creditNote(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Net    int64  `json:"net"`
		Reason string `json:"reason"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.Net <= 0 || strings.TrimSpace(in.Reason) == "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid credit note", "a positive net amount and a reason are required")
	}
	ctx := r.Context()
	v, err := loadInvoice(ctx, s, r.PathValue("id"), true, false)
	if err != nil {
		return nil, err
	}
	if v.Status != "issued" && v.Status != "paid" {
		return nil, platform.Err(http.StatusConflict, "Cannot credit", "only an issued invoice can be credited")
	}
	st, err := loadSettings(ctx, s)
	if err != nil {
		return nil, err
	}
	tva, cac := Taxes(in.Net, st.TVABP, st.CACPct)
	total := in.Net + tva + cac
	if v.Credited+total > v.Total {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Too much credit", "credit notes cannot exceed the invoice total")
	}
	doc, err := documents.Issue(ctx, s, "CN", "Credit note for "+*v.Ref, v.TenantID, "")
	if err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `INSERT INTO sales.credit_notes (invoice_id, tenant_id, ref, reason, net, tva, cac, total, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		v.ID, v.TenantID, doc.Ref, strings.TrimSpace(in.Reason), in.Net, tva, cac, total, s.Claims.Sub); err != nil {
		return nil, err
	}
	if m.post != nil {
		if err := m.post.CreditNoteIssued(ctx, s, CreditEntry{Ref: doc.Ref, InvoiceRef: *v.Ref, Tenant: v.TenantID, Net: in.Net, TVA: tva, CAC: cac, Total: total, Date: time.Now().UTC()}); err != nil {
			return nil, err
		}
	}
	if err := s.Emit(ctx, "credit_note.created", map[string]any{"ref": doc.Ref, "invoice_id": v.ID, "total": total}); err != nil {
		return nil, err
	}
	out, err := settle(ctx, s, v.ID)
	if err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "credit_note", "invoice", v.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: map[string]any{"credit_note_ref": doc.Ref, "total": total, "invoice": out}}, nil
}

func report(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var issued, collected, credited, outstanding, overdue, openQuotes, pipeline int64
	err := s.Tx.QueryRow(r.Context(), `SELECT
		COALESCE((SELECT sum(total) FROM sales.invoices WHERE status IN ('issued','paid')),0),
		COALESCE((SELECT sum(amount) FROM sales.payments),0),
		COALESCE((SELECT sum(total) FROM sales.credit_notes),0),
		COALESCE((SELECT sum(i.total - COALESCE((SELECT sum(amount) FROM sales.payments p WHERE p.invoice_id=i.id),0)
		                              - COALESCE((SELECT sum(total) FROM sales.credit_notes c WHERE c.invoice_id=i.id),0))
		          FROM sales.invoices i WHERE i.status='issued'),0),
		(SELECT count(*) FROM sales.invoices WHERE status='issued' AND due_date < current_date),
		(SELECT count(*) FROM sales.quotes WHERE status='sent'),
		COALESCE((SELECT sum(l.quantity::float8 * l.unit_price)::bigint FROM sales.quote_lines l JOIN sales.quotes q ON q.id=l.quote_id WHERE q.status='sent'),0)`).
		Scan(&issued, &collected, &credited, &outstanding, &overdue, &openQuotes, &pipeline)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{
		"invoiced": issued, "collected": collected, "credited": credited, "outstanding": outstanding,
		"overdue_invoices": overdue, "open_quotes": openQuotes, "pipeline_net": pipeline,
	}}, nil
}

func toDocLines(lines []Line) []docs.Line {
	out := make([]docs.Line, len(lines))
	for i, l := range lines {
		out[i] = docs.Line{Description: l.Description, Quantity: l.Quantity, UnitPrice: l.UnitPrice}
	}
	return out
}

// invoicePDF serves the canonical PDF of an issued invoice. It prints nothing that changes after
// issue (payments, balance), and the first rendering is the one that is stored and hashed.
func (m *module) invoicePDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	v, err := loadInvoice(ctx, s, r.PathValue("id"), false, true)
	if err != nil {
		return nil, err
	}
	if (v.Status != "issued" && v.Status != "paid") || v.Ref == nil || v.IssueDate == nil {
		return nil, platform.Err(http.StatusConflict, "Not issued", "only an issued invoice has a PDF")
	}
	return documents.ServePDF(ctx, s, *v.Ref, func() ([]byte, error) {
		client, err := clients.Party(ctx, s, v.TenantID)
		if err != nil {
			return nil, err
		}
		return docs.RenderInvoice(m.docs, *v.Ref, *v.IssueDate, v.DueDate, v.Title, client, toDocLines(v.Lines), docs.Money{Net: v.Net, TVA: v.TVA, CAC: v.CAC, Total: v.Total})
	})
}

// quotePDF serves the canonical PDF of a quotation that has been sent to the client.
func (m *module) quotePDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	q, err := scanQuote(s.Tx.QueryRow(ctx, `SELECT `+quoteCols+` FROM sales.quotes WHERE id::text = $1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	if q.Status == "draft" || q.Ref == nil {
		return nil, platform.Err(http.StatusConflict, "Not sent", "only a sent quotation has a PDF")
	}
	if err := fillQuote(ctx, s, q, true); err != nil {
		return nil, err
	}
	return documents.ServePDF(ctx, s, *q.Ref, func() ([]byte, error) {
		client, err := clients.Party(ctx, s, q.TenantID)
		if err != nil {
			return nil, err
		}
		entry, err := documents.Lookup(ctx, s, *q.Ref)
		if err != nil {
			return nil, err
		}
		return docs.RenderQuote(m.docs, *q.Ref, entry.IssuedAt, q.ValidUntil, q.Title, client, toDocLines(q.Lines), docs.Money{Net: q.Net, TVA: q.TVA, CAC: q.CAC, Total: q.Total}, q.Notes)
	})
}

// listPayments lists the payments recorded against an invoice, each with its receipt reference.
func (m *module) listPayments(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT p.id, p.amount, p.method, p.reference, p.paid_at, p.receipt_ref
		FROM sales.payments p WHERE p.invoice_id::text = $1 ORDER BY p.paid_at, p.id`, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, method, reference string
		var amount int64
		var paid time.Time
		var rct *string
		if err := rows.Scan(&id, &amount, &method, &reference, &paid, &rct); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "amount": amount, "method": method, "reference": reference, "paid_at": paid, "receipt_ref": rct})
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"items": out}}, rows.Err()
}

// receiptPDF serves the canonical receipt of one payment.
func (m *module) receiptPDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	var tenant, invRef, method, reference string
	var amount, invTotal int64
	var paid time.Time
	var rct *string
	err := s.Tx.QueryRow(ctx, `SELECT p.tenant_id::text, i.ref, p.method, p.reference, p.amount, i.total, p.paid_at, p.receipt_ref
		FROM sales.payments p JOIN sales.invoices i ON i.id = p.invoice_id WHERE p.id::text = $1`, r.PathValue("id")).
		Scan(&tenant, &invRef, &method, &reference, &amount, &invTotal, &paid, &rct)
	if err != nil {
		return nil, orNotFound("payment", err)
	}
	if rct == nil {
		return nil, platform.Err(http.StatusConflict, "No receipt", "this payment was recorded before receipts existed")
	}
	return documents.ServePDF(ctx, s, *rct, func() ([]byte, error) {
		client, err := clients.Party(ctx, s, tenant)
		if err != nil {
			return nil, err
		}
		return docs.RenderReceipt(m.docs, *rct, paid, client, invRef, invTotal, amount, method, reference)
	})
}

// creditNotePDF serves the canonical PDF of a credit note.
func (m *module) creditNotePDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	var tenant, ref, invRef, reason string
	var net, tva, cac, total int64
	var created time.Time
	err := s.Tx.QueryRow(ctx, `SELECT c.tenant_id::text, c.ref, i.ref, c.reason, c.net, c.tva, c.cac, c.total, c.created_at
		FROM sales.credit_notes c JOIN sales.invoices i ON i.id = c.invoice_id WHERE c.ref = $1`, r.PathValue("ref")).
		Scan(&tenant, &ref, &invRef, &reason, &net, &tva, &cac, &total, &created)
	if err != nil {
		return nil, orNotFound("credit note", err)
	}
	return documents.ServePDF(ctx, s, ref, func() ([]byte, error) {
		client, err := clients.Party(ctx, s, tenant)
		if err != nil {
			return nil, err
		}
		return docs.RenderCreditNote(m.docs, ref, created, invRef, reason, client, docs.Money{Net: net, TVA: tva, CAC: cac, Total: total})
	})
}

func orNotFound(what string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound(what)
	}
	return err
}
