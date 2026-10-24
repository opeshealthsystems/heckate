package sales

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// RemindersActor is the name the audit log shows for reminders the scheduler sends.
const RemindersActor = "system:payment-reminders"

// reminderStage maps days past due to the reminder stage: 1 overdue, 2 a week late, 3 a month late.
func reminderStage(daysLate int) int {
	switch {
	case daysLate >= 30:
		return 3
	case daysLate >= 7:
		return 2
	case daysLate >= 1:
		return 1
	}
	return 0
}

type reminderSent struct {
	InvoiceID string `json:"invoice_id"`
	Ref       string `json:"ref"`
	TenantID  string `json:"tenant_id"`
	Stage     int    `json:"stage"`
	DaysLate  int    `json:"days_late"`
	Owed      int64  `json:"owed"`
}

// sendReminders reminds clients about invoices that are still owed after their due date. An invoice gets
// at most one reminder per stage, and only the highest stage it has reached (so a long-overdue invoice
// found for the first time gets one reminder, not three). The reminder is a push notification to the
// client's admins, sent through the outbox in the same transaction that records it.
func sendReminders(ctx context.Context, s *platform.Scope, asAt time.Time) ([]reminderSent, error) {
	rows, err := s.Tx.Query(ctx, `
		WITH open AS (
		  SELECT i.id, i.tenant_id, i.ref,
		         i.total - COALESCE((SELECT sum(amount) FROM sales.payments p WHERE p.invoice_id = i.id),0)
		                 - COALESCE((SELECT sum(total) FROM sales.credit_notes c WHERE c.invoice_id = i.id),0) AS owed,
		         ($1::date - COALESCE(i.due_date, i.issue_date)) AS days_late
		  FROM sales.invoices i WHERE i.status IN ('issued','paid') AND i.issue_date <= $1)
		SELECT id::text, tenant_id::text, COALESCE(ref,''), owed, days_late FROM open
		WHERE owed > 0 AND days_late >= 1 ORDER BY days_late DESC, id`, asAt)
	if err != nil {
		return nil, err
	}
	type cand struct {
		id, tenant, ref string
		owed            int64
		late            int
	}
	var cands []cand
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.id, &c.tenant, &c.ref, &c.owed, &c.late); err != nil {
			rows.Close()
			return nil, err
		}
		cands = append(cands, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sent := []reminderSent{}
	for _, c := range cands {
		stage := reminderStage(c.late)
		var higher bool
		if err := s.Tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sales.invoice_reminders WHERE invoice_id = $1 AND stage >= $2)`, c.id, stage).Scan(&higher); err != nil {
			return nil, err
		}
		if higher {
			continue
		}
		tag, err := s.Tx.Exec(ctx, `INSERT INTO sales.invoice_reminders (invoice_id, stage, days_late, owed) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`, c.id, stage, c.late, c.owed)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			continue
		}
		r := reminderSent{InvoiceID: c.id, Ref: c.ref, TenantID: c.tenant, Stage: stage, DaysLate: c.late, Owed: c.owed}
		if err := s.Emit(ctx, "invoice.reminder", map[string]any{"id": c.id, "tenant_id": c.tenant, "ref": c.ref, "stage": stage, "days_late": c.late, "owed": c.owed}); err != nil {
			return nil, err
		}
		sent = append(sent, r)
	}
	return sent, nil
}

func today() time.Time { return time.Now().UTC().Truncate(24 * time.Hour) }

func (m *module) runReminders(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	sent, err := sendReminders(r.Context(), s, today())
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"sent": sent}}, nil
}

func (m *module) listReminders(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `
		SELECT r.invoice_id::text, COALESCE(i.ref,''), i.tenant_id::text, r.stage, r.days_late, r.owed, r.sent_at
		FROM sales.invoice_reminders r JOIN sales.invoices i ON i.id = r.invoice_id
		ORDER BY r.sent_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		InvoiceID string    `json:"invoice_id"`
		Ref       string    `json:"ref"`
		TenantID  string    `json:"tenant_id"`
		Stage     int       `json:"stage"`
		DaysLate  int       `json:"days_late"`
		Owed      int64     `json:"owed"`
		SentAt    time.Time `json:"sent_at"`
	}
	out := []row{}
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.InvoiceID, &x.Ref, &x.TenantID, &x.Stage, &x.DaysLate, &x.Owed, &x.SentAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

// RunReminderScheduler sends due reminders on a timer. Safe in every instance: the primary key on
// (invoice, stage) lets only one of them record a given reminder.
func RunReminderScheduler(ctx context.Context, pool *pgxpool.Pool, every time.Duration) {
	pass := func() {
		n, err := RunRemindersOnce(ctx, pool)
		switch {
		case err != nil && ctx.Err() == nil:
			slog.Error("payment reminders", "err", err)
		case n > 0:
			slog.Info("payment reminders sent", "count", n)
		}
	}
	pass()
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

// RunRemindersOnce is one scheduler pass; it returns how many reminders it sent.
func RunRemindersOnce(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id','',true), set_config('app.role','staff',true), set_config('app.actor',$1,true), set_config('app.roles','finance',true)`, RemindersActor); err != nil {
		return 0, err
	}
	s := &platform.Scope{Tx: tx, Claims: &platform.Claims{Sub: RemindersActor, Roles: []string{"finance"}}}
	sent, err := sendReminders(ctx, s, today())
	if err != nil {
		return 0, err
	}
	return len(sent), tx.Commit(ctx)
}
