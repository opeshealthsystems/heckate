package sales

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

type Recurring struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	Title          string    `json:"title"`
	Lines          []Line    `json:"lines"`
	IntervalMonths int       `json:"interval_months"`
	DueDays        int       `json:"due_days"`
	NextRun        string    `json:"next_run"`
	Active         bool      `json:"active"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

const recCols = `id::text, tenant_id::text, title, lines, interval_months, due_days, to_char(next_run,'YYYY-MM-DD'), active, created_by, created_at`

func scanRecurring(row pgx.Row) (*Recurring, error) {
	var x Recurring
	var lines []byte
	err := row.Scan(&x.ID, &x.TenantID, &x.Title, &lines, &x.IntervalMonths, &x.DueDays, &x.NextRun, &x.Active, &x.CreatedBy, &x.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, notFound("recurring invoice")
	}
	if err != nil {
		return nil, err
	}
	return &x, json.Unmarshal(lines, &x.Lines)
}

func (m *module) createRecurring(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		TenantID       string `json:"tenant_id"`
		Title          string `json:"title"`
		Lines          []Line `json:"lines"`
		IntervalMonths int    `json:"interval_months"`
		DueDays        *int   `json:"due_days"`
		FirstRun       string `json:"first_run"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.TenantID == "" || in.Title == "" || len(in.Title) > 200 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid template", "tenant_id and a title of up to 200 characters are required")
	}
	if in.IntervalMonths != 1 && in.IntervalMonths != 3 && in.IntervalMonths != 6 && in.IntervalMonths != 12 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid template", "interval_months must be 1, 3, 6 or 12")
	}
	dueDays := 30
	if in.DueDays != nil {
		dueDays = *in.DueDays
	}
	if dueDays < 0 || dueDays > 365 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid template", "due_days must be between 0 and 365")
	}
	if err := validLines(in.Lines); err != nil {
		return nil, err
	}
	first, err := time.Parse("2006-01-02", in.FirstRun)
	if err != nil {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid template", "first_run must be YYYY-MM-DD")
	}
	lines, _ := json.Marshal(in.Lines)
	x, err := scanRecurring(s.Tx.QueryRow(r.Context(), `INSERT INTO sales.recurring (tenant_id, title, lines, interval_months, due_days, next_run, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+recCols, in.TenantID, in.Title, lines, in.IntervalMonths, dueDays, first, s.Claims.Sub))
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid template", "unknown tenant")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "create", "recurring_invoice", x.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: x}, nil
}

func (m *module) listRecurring(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+recCols+` FROM sales.recurring ORDER BY next_run, created_at LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Recurring{}
	for rows.Next() {
		x, err := scanRecurring(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func (m *module) setRecurringActive(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Active bool `json:"active"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	x, err := scanRecurring(s.Tx.QueryRow(r.Context(), `UPDATE sales.recurring SET active = $2 WHERE id::text = $1 RETURNING `+recCols, r.PathValue("id"), in.Active))
	if err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "set_active", "recurring_invoice", x.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: x}, nil
}

// runRecurring is the endpoint: finance (or a person with the role) triggers a run on demand.
func (m *module) runRecurring(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	created, err := runDueRecurring(r.Context(), s)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"created": created}}, nil
}

// runDueRecurring creates one draft invoice for every active template that is due, and moves each template
// on by its interval. A template that missed several periods catches up one draft per call, so nothing is
// billed twice or skipped silently. Drafts only: finance issues them, with the usual dual control.
func runDueRecurring(ctx context.Context, s *platform.Scope) ([]map[string]any, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	rows, err := s.Tx.Query(ctx, `SELECT `+recCols+` FROM sales.recurring WHERE active AND next_run <= $1 ORDER BY next_run, id FOR UPDATE SKIP LOCKED`, today)
	if err != nil {
		return nil, err
	}
	var due []*Recurring
	for rows.Next() {
		x, err := scanRecurring(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		due = append(due, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	created := []map[string]any{}
	for _, x := range due {
		run, _ := time.Parse("2006-01-02", x.NextRun)
		dueDate := run.AddDate(0, 0, x.DueDays)
		id, err := newDraftInvoice(ctx, s, x.TenantID, x.Title, dueDate, x.Lines)
		if err != nil {
			return nil, err
		}
		if _, err := s.Tx.Exec(ctx, `INSERT INTO sales.recurring_runs (recurring_id, run_date, invoice_id) VALUES ($1,$2,$3)`, x.ID, run, id); err != nil {
			return nil, err
		}
		if _, err := s.Tx.Exec(ctx, `UPDATE sales.recurring SET next_run = $2 WHERE id = $1`, x.ID, run.AddDate(0, x.IntervalMonths, 0)); err != nil {
			return nil, err
		}
		if err := s.Audit(ctx, "run", "recurring_invoice", x.ID); err != nil {
			return nil, err
		}
		created = append(created, map[string]any{"recurring_id": x.ID, "invoice_id": id, "run_date": x.NextRun})
	}
	return created, nil
}

// SchedulerActor is the name the audit log shows for drafts the scheduler creates.
const SchedulerActor = "system:recurring-invoices"

// RunScheduler creates the drafts for due templates on a timer, so recurring billing does not depend on
// someone remembering to press a button. Safe in every instance: rows are taken with SKIP LOCKED and a
// template can produce only one draft per date. A failed pass is logged and retried at the next tick.
func RunScheduler(ctx context.Context, pool *pgxpool.Pool, every time.Duration) {
	pass := func() {
		n, err := RunScheduledOnce(ctx, pool)
		switch {
		case err != nil && ctx.Err() == nil:
			slog.Error("recurring invoices", "err", err)
		case n > 0:
			slog.Info("recurring invoices: drafts created", "count", n)
		}
	}
	pass() // catch up at start-up instead of waiting a full interval
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			pass()
		}
	}
}

// RunScheduledOnce is one scheduler pass; it returns how many drafts it created.
func RunScheduledOnce(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id','',true), set_config('app.role','staff',true), set_config('app.actor',$1,true), set_config('app.roles','finance',true)`, SchedulerActor); err != nil {
		return 0, err
	}
	s := &platform.Scope{Tx: tx, Claims: &platform.Claims{Sub: SchedulerActor, Roles: []string{"finance"}}}
	created, err := runDueRecurring(ctx, s)
	if err != nil {
		return 0, err
	}
	return len(created), tx.Commit(ctx)
}

// ageing groups what clients still owe by how far past due it is, as at a date (today by default).
func ageing(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	asAt := time.Now().UTC().Truncate(24 * time.Hour)
	if v := r.URL.Query().Get("as_at"); v != "" {
		d, err := time.Parse("2006-01-02", v)
		if err != nil {
			return nil, platform.Err(http.StatusBadRequest, "Bad date", "as_at must be YYYY-MM-DD")
		}
		asAt = d
	}
	return ageingAt(r.Context(), s, asAt)
}

func ageingAt(ctx context.Context, s *platform.Scope, asAt time.Time) (*platform.Result, error) {
	rows, err := s.Tx.Query(ctx, `
		WITH open AS (
		  SELECT i.tenant_id, i.ref,
		         i.total - COALESCE((SELECT sum(amount) FROM sales.payments p WHERE p.invoice_id = i.id AND p.paid_at::date <= $1),0)
		                 - COALESCE((SELECT sum(total) FROM sales.credit_notes c WHERE c.invoice_id = i.id AND c.created_at::date <= $1),0) AS owed,
		         GREATEST($1::date - COALESCE(i.due_date, i.issue_date), 0) AS days_late
		  FROM sales.invoices i WHERE i.status IN ('issued','paid') AND i.issue_date <= $1)
		SELECT o.tenant_id::text, a.legal_name,
		  COALESCE(sum(owed) FILTER (WHERE days_late = 0), 0),
		  COALESCE(sum(owed) FILTER (WHERE days_late BETWEEN 1 AND 30), 0),
		  COALESCE(sum(owed) FILTER (WHERE days_late BETWEEN 31 AND 60), 0),
		  COALESCE(sum(owed) FILTER (WHERE days_late BETWEEN 61 AND 90), 0),
		  COALESCE(sum(owed) FILTER (WHERE days_late > 90), 0)
		FROM open o JOIN clients.accounts a ON a.tenant_id = o.tenant_id
		WHERE owed > 0 GROUP BY o.tenant_id, a.legal_name ORDER BY a.legal_name`, asAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		TenantID string `json:"tenant_id"`
		Client   string `json:"client"`
		Current  int64  `json:"current"`
		D1to30   int64  `json:"d1_30"`
		D31to60  int64  `json:"d31_60"`
		D61to90  int64  `json:"d61_90"`
		Over90   int64  `json:"over_90"`
		Total    int64  `json:"total"`
	}
	out := []row{}
	var tot row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.TenantID, &x.Client, &x.Current, &x.D1to30, &x.D31to60, &x.D61to90, &x.Over90); err != nil {
			return nil, err
		}
		x.Total = x.Current + x.D1to30 + x.D31to60 + x.D61to90 + x.Over90
		tot.Current += x.Current
		tot.D1to30 += x.D1to30
		tot.D31to60 += x.D31to60
		tot.D61to90 += x.D61to90
		tot.Over90 += x.Over90
		tot.Total += x.Total
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"as_at": asAt.Format("2006-01-02"), "clients": out, "totals": tot}}, rows.Err()
}
