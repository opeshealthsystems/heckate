package contracts

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/internal/reports"
)

// RenewalsActor is the name the audit log and the contract history show for what the scheduler does.
const RenewalsActor = "system:contract-renewals"

// alertStage is the earliest warning a contract has reached: 90, 60 or 30 days before it ends. Zero means none yet.
func alertStage(daysLeft int) int {
	switch {
	case daysLeft < 0:
		return 0
	case daysLeft <= 30:
		return 30
	case daysLeft <= 60:
		return 60
	case daysLeft <= 90:
		return 90
	}
	return 0
}

type Outcome struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	Action string `json:"action"` // renewal_due, auto_renewed or expired
	Detail string `json:"detail"`
}

func today() time.Time { return time.Now().UTC().Truncate(24 * time.Hour) }

// processRenewals ends or auto-renews contracts whose term has passed and warns about those about to end. It is
// safe to run often and from several instances: each warning is recorded once per term, and a contract is
// locked while it is changed.
func processRenewals(ctx context.Context, s *platform.Scope, asAt time.Time) ([]Outcome, error) {
	out := []Outcome{}
	rows, err := s.Tx.Query(ctx, `SELECT id::text FROM contracts.contracts WHERE status = 'active' AND end_date < $1 ORDER BY end_date, id`, asAt)
	if err != nil {
		return nil, err
	}
	var due []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		due = append(due, id)
	}
	rows.Close()
	for _, id := range due {
		c, err := lock(ctx, s, id)
		if err != nil {
			return nil, err
		}
		if c.Status != "active" || !c.EndDate.Before(asAt) {
			continue
		}
		if c.AutoRenew {
			end := c.EndDate
			for end.Before(asAt) {
				end = end.AddDate(0, c.RenewalTermMonths, 0)
			}
			if _, err := s.Tx.Exec(ctx, `UPDATE contracts.contracts SET end_date=$2 WHERE id=$1`, c.ID, end); err != nil {
				return nil, err
			}
			if err := record(ctx, s, c.ID, c.TenantID, "auto_renewed", map[string]any{"from": c.EndDate.Format("2006-01-02"), "to": end.Format("2006-01-02")}); err != nil {
				return nil, err
			}
			if err := s.Emit(ctx, "contract.auto_renewed", map[string]any{"id": c.ID, "tenant_id": c.TenantID, "number": c.Number, "to": end.Format("2006-01-02")}); err != nil {
				return nil, err
			}
			out = append(out, Outcome{c.ID, c.Number, "auto_renewed", "now ends " + end.Format("2006-01-02")})
			continue
		}
		if _, err := s.Tx.Exec(ctx, `UPDATE contracts.contracts SET status='expired' WHERE id=$1`, c.ID); err != nil {
			return nil, err
		}
		if err := record(ctx, s, c.ID, c.TenantID, "expired", map[string]any{"ended": c.EndDate.Format("2006-01-02")}); err != nil {
			return nil, err
		}
		if err := s.Emit(ctx, "contract.expired", map[string]any{"id": c.ID, "tenant_id": c.TenantID, "number": c.Number}); err != nil {
			return nil, err
		}
		out = append(out, Outcome{c.ID, c.Number, "expired", "ended " + c.EndDate.Format("2006-01-02")})
	}

	rows, err = s.Tx.Query(ctx, `SELECT id::text, number, end_date, (end_date - $1::date)::int, tenant_id::text FROM contracts.contracts
		WHERE status = 'active' AND end_date >= $1 AND end_date <= $1::date + 90 ORDER BY end_date, id`, asAt)
	if err != nil {
		return nil, err
	}
	type cand struct {
		id, number, tenant string
		end                time.Time
		left               int
	}
	var cands []cand
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.id, &c.number, &c.end, &c.left, &c.tenant); err != nil {
			rows.Close()
			return nil, err
		}
		cands = append(cands, c)
	}
	rows.Close()
	for _, c := range cands {
		stage := alertStage(c.left)
		if stage == 0 {
			continue
		}
		var reached bool
		if err := s.Tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM contracts.renewal_alerts WHERE contract_id=$1 AND end_date=$2 AND days_before <= $3)`, c.id, c.end, stage).Scan(&reached); err != nil {
			return nil, err
		}
		if reached {
			continue
		}
		tag, err := s.Tx.Exec(ctx, `INSERT INTO contracts.renewal_alerts (contract_id, end_date, days_before) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, c.id, c.end, stage)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			continue
		}
		if err := record(ctx, s, c.id, c.tenant, "renewal_due", map[string]any{"days_left": c.left, "stage": stage}); err != nil {
			return nil, err
		}
		if err := s.Emit(ctx, "contract.renewal_due", map[string]any{"id": c.id, "tenant_id": c.tenant, "number": c.number, "days_left": c.left}); err != nil {
			return nil, err
		}
		out = append(out, Outcome{c.id, c.number, "renewal_due", fmt.Sprintf("%d days left", c.left)})
	}
	return out, nil
}

func runRenewals(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	out, err := processRenewals(r.Context(), s, today())
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"done": out}}, nil
}

// RunScheduler checks renewals on a timer.
func RunScheduler(ctx context.Context, pool *pgxpool.Pool, every time.Duration) {
	pass := func() {
		n, err := RunOnce(ctx, pool)
		switch {
		case err != nil && ctx.Err() == nil:
			slog.Error("contract renewals", "err", err)
		case n > 0:
			slog.Info("contract renewals processed", "count", n)
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

// RunOnce is one scheduler pass; it returns how many things it did.
func RunOnce(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id','',true), set_config('app.role','staff',true), set_config('app.actor',$1,true), set_config('app.roles','manager',true)`, RenewalsActor); err != nil {
		return 0, err
	}
	s := &platform.Scope{Tx: tx, Claims: &platform.Claims{Sub: RenewalsActor, Roles: []string{"manager"}}}
	done, err := processRenewals(ctx, s, today())
	if err != nil {
		return 0, err
	}
	return len(done), tx.Commit(ctx)
}

// schedulePDF prints the contract schedule: the terms, what the client is promised and the clauses.
func (m *module) schedulePDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	c, err := load(r.Context(), s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	rows := [][]string{}
	for _, x := range *c.Clauses {
		rows = append(rows, []string{fmt.Sprint(x.Position), x.Title, x.Body})
	}
	sla := [][]string{}
	for _, x := range *c.SLA {
		sla = append(sla, []string{x.Severity, minutes(x.RespondMinutes), minutes(x.ResolveMinutes)})
	}
	renew := "Ends on " + docs.Day(c.EndDate) + "; not renewed automatically."
	if c.AutoRenew {
		renew = fmt.Sprintf("Ends on %s and renews automatically for %d months unless notice of %d days is given.", docs.Day(c.EndDate), c.RenewalTermMonths, c.RenewalNoticeDays)
	}
	pdf, err := docs.RenderReport(m.docs, docs.Report{
		Title: "Contract schedule", Subtitle: c.Number + ": " + c.Title, Class: "CONFIDENTIAL", GeneratedBy: s.Claims.Sub, At: time.Now().UTC(),
		Facts: [][2]string{{"Client", c.Client}, {"Kind", c.Kind}, {"Status", c.Status}, {"Term", docs.Day(c.StartDate) + " to " + docs.Day(c.EndDate) + fmt.Sprintf(" (%d months)", c.TermMonths)},
			{"Value", docs.XAF(c.Value)}, {"Billing", fmt.Sprintf("%s, %s per period", c.Billing, docs.XAF(c.PeriodAmount))}},
		Sections: []docs.ReportSection{
			{Heading: "Response and resolution times", Columns: []docs.ReportColumn{{Header: "Severity", Width: 30}, {Header: "Respond within"}, {Header: "Resolve within"}}, Rows: sla, Empty: "The standard support targets apply."},
			{Heading: "Clauses", Columns: []docs.ReportColumn{{Header: "No.", Width: 12}, {Header: "Title", Width: 50}, {Header: "Text"}}, Rows: rows, Empty: "No clauses yet."},
		},
		Note: renew + " This schedule is a printout of the register, not the signed agreement.",
	})
	if err != nil {
		return nil, err
	}
	return reports.PDF("contract-"+c.Number, pdf), nil
}

func minutes(n int) string {
	switch {
	case n%1440 == 0:
		return fmt.Sprintf("%d day(s)", n/1440)
	case n%60 == 0:
		return fmt.Sprintf("%d hour(s)", n/60)
	}
	return fmt.Sprintf("%d minutes", n)
}
