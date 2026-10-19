// Package payroll computes monthly payslips from a rules configuration that a second person has
// validated, takes a run through preparation, approval and payment with dual control, and posts it
// to the ledger. The seeded rates are unvalidated: no run can be created until they are checked.
package payroll

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// Poster books payroll in the ledger inside the same transaction.
type Poster interface {
	RunApproved(ctx context.Context, s *platform.Scope, e RunEntry) error
	RunPaid(ctx context.Context, s *platform.Scope, e RunEntry) error
}

type RunEntry struct {
	Period          time.Time
	Gross, Net      int64
	EmployeeContrib int64
	EmployerContrib int64
	Tax             int64
}

var (
	prepare  = []string{"hr", "finance", "manager", "ceo"}
	decide   = []string{"finance", "manager", "ceo"}
	viewers  = []string{"hr", "finance", "manager", "ceo", "auditor"}
	hrOrCEO  = []string{"hr", "ceo"}
	finOrCEO = []string{"finance", "ceo"}
	staff    = []string{"ceo", "manager", "finance", "sales", "support", "engineer", "hr", "employee", "logistics", "procurement"}
)

type mod struct {
	poster Poster
	docs   docs.Options
}

func Register(a *platform.App, p Poster, o docs.Options) {
	m := &mod{poster: p, docs: o}
	a.Route("PUT /v1/payroll/compensation/{employee_id}", hrOrCEO, m.setCompensation)
	a.Route("GET /v1/payroll/compensation", []string{"hr", "finance", "ceo"}, m.listCompensation)
	a.Route("GET /v1/payroll/config", viewers, m.getConfig)
	a.Route("PUT /v1/payroll/config", finOrCEO, m.putConfig)
	a.Route("POST /v1/payroll/config/validate", finOrCEO, platform.Idempotent(m.validateConfig))
	a.Route("POST /v1/payroll/runs", prepare, platform.Idempotent(m.createRun))
	a.Route("GET /v1/payroll/runs", viewers, m.listRuns)
	a.Route("GET /v1/payroll/runs/{id}", viewers, m.getRun)
	a.Route("DELETE /v1/payroll/runs/{id}", prepare, m.discardRun)
	a.Route("POST /v1/payroll/runs/{id}/approve", decide, platform.Idempotent(m.approve))
	a.Route("POST /v1/payroll/runs/{id}/pay", decide, platform.Idempotent(m.pay))
	a.Route("GET /v1/payslips/me", staff, m.mySlips)
	a.Route("GET /v1/payslips/{id}/pdf", staff, m.slipPDF)
}

func nf(what string) error { return platform.Err(http.StatusNotFound, "Not found", what+" not found") }
func bad(msg string) error {
	return platform.Err(http.StatusUnprocessableEntity, "Invalid request", msg)
}
func isNF(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || (err != nil && strings.Contains(err.Error(), "invalid input syntax"))
}

// guardErr turns the database's dual-control refusals into proper responses.
func guardErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "someone other than"):
		return platform.Err(http.StatusForbidden, "Dual control", "this step needs a different person from the earlier ones")
	case strings.Contains(msg, "are not validated"):
		return platform.Err(http.StatusConflict, "Rules not validated", "the payroll rules used by this run have not been validated")
	case strings.Contains(msg, "invalid payroll status change"), strings.Contains(msg, "immutable"), strings.Contains(msg, "cannot change"):
		return platform.Err(http.StatusConflict, "Wrong state", "this run cannot move that way")
	case strings.Contains(msg, "only finance or management"), strings.Contains(msg, "row-level security"):
		return platform.Err(http.StatusForbidden, "Forbidden", "your role cannot do this")
	}
	return err
}

// ---- compensation ----

func (m *mod) setCompensation(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		EffectiveFrom string `json:"effective_from"`
		BaseSalary    int64  `json:"base_salary"`
		Allowances    int64  `json:"allowances"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	from, err := time.Parse("2006-01-02", in.EffectiveFrom)
	if err != nil || in.BaseSalary < 0 || in.Allowances < 0 {
		return nil, bad("effective_from (YYYY-MM-DD) and non-negative amounts are required")
	}
	_, err = s.Tx.Exec(r.Context(), `INSERT INTO payroll.compensation (employee_id, effective_from, base_salary, allowances, set_by) VALUES ($1,$2,$3,$4,$5)`,
		r.PathValue("employee_id"), from, in.BaseSalary, in.Allowances, s.Claims.Sub)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "duplicate key"):
			return nil, platform.Err(http.StatusConflict, "Already set", "a salary already starts on that date; set a later date to change it")
		case strings.Contains(err.Error(), "foreign key"), strings.Contains(err.Error(), "invalid input syntax"):
			return nil, nf("employee")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "set", "compensation", r.PathValue("employee_id")); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: map[string]any{"employee_id": r.PathValue("employee_id"), "effective_from": in.EffectiveFrom, "base_salary": in.BaseSalary, "allowances": in.Allowances}}, nil
}

func (m *mod) listCompensation(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT DISTINCT ON (c.employee_id) c.employee_id::text, e.full_name, c.effective_from, c.base_salary, c.allowances
		FROM payroll.compensation c JOIN hr.employees e ON e.id = c.employee_id WHERE c.effective_from <= current_date
		ORDER BY c.employee_id, c.effective_from DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		EmployeeID    string    `json:"employee_id"`
		FullName      string    `json:"full_name"`
		EffectiveFrom time.Time `json:"effective_from"`
		BaseSalary    int64     `json:"base_salary"`
		Allowances    int64     `json:"allowances"`
	}
	out := []row{}
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.EmployeeID, &x.FullName, &x.EffectiveFrom, &x.BaseSalary, &x.Allowances); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

// ---- rules configuration ----

type cfgView struct {
	Config      Config     `json:"config"`
	EditedBy    string     `json:"edited_by"`
	EditedAt    time.Time  `json:"edited_at"`
	ValidatedBy *string    `json:"validated_by"`
	ValidatedAt *time.Time `json:"validated_at"`
}

func loadConfig(ctx context.Context, s *platform.Scope) (*cfgView, error) {
	var v cfgView
	var raw []byte
	if err := s.Tx.QueryRow(ctx, `SELECT config, edited_by, edited_at, validated_by, validated_at FROM payroll.config`).Scan(&raw, &v.EditedBy, &v.EditedAt, &v.ValidatedBy, &v.ValidatedAt); err != nil {
		return nil, err
	}
	return &v, json.Unmarshal(raw, &v.Config)
}

func (m *mod) getConfig(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	v, err := loadConfig(r.Context(), s)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: v}, nil
}

func (m *mod) putConfig(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var c Config
	if err := platform.DecodeBody(r, &c); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, bad(err.Error())
	}
	raw, _ := json.Marshal(c)
	if _, err := s.Tx.Exec(r.Context(), `UPDATE payroll.config SET config = $1, edited_by = $2`, raw, s.Claims.Sub); err != nil {
		return nil, guardErr(err)
	}
	if err := s.Audit(r.Context(), "edit", "payroll_config", "rules"); err != nil {
		return nil, err
	}
	v, err := loadConfig(r.Context(), s)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: v}, nil
}

// validateConfig records that someone other than the editor has checked the rules.
func (m *mod) validateConfig(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	if _, err := s.Tx.Exec(r.Context(), `UPDATE payroll.config SET validated_by = $1`, s.Claims.Sub); err != nil {
		return nil, guardErr(err)
	}
	if err := s.Audit(r.Context(), "validate", "payroll_config", "rules"); err != nil {
		return nil, err
	}
	v, err := loadConfig(r.Context(), s)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: v}, nil
}

// ---- runs ----

type Run struct {
	ID         string     `json:"id"`
	Period     string     `json:"period"`
	Status     string     `json:"status"`
	PreparedBy string     `json:"prepared_by"`
	ApprovedBy *string    `json:"approved_by"`
	PaidBy     *string    `json:"paid_by"`
	TotalGross int64      `json:"total_gross"`
	TotalNet   int64      `json:"total_net"`
	TotalCost  int64      `json:"total_cost"`
	CreatedAt  time.Time  `json:"created_at"`
	ApprovedAt *time.Time `json:"approved_at"`
	PaidAt     *time.Time `json:"paid_at"`
	Payslips   []Payslip  `json:"payslips,omitempty"`
	Skipped    []string   `json:"skipped,omitempty"`
}

type Payslip struct {
	ID           string `json:"id"`
	RunID        string `json:"run_id"`
	Period       string `json:"period,omitempty"`
	Actor        string `json:"actor"`
	FullName     string `json:"full_name"`
	Gross        int64  `json:"gross"`
	Deductions   []Line `json:"deductions"`
	IRPP         int64  `json:"irpp"`
	CAC          int64  `json:"cac"`
	Net          int64  `json:"net"`
	Employer     []Line `json:"employer_lines"`
	EmployerCost int64  `json:"employer_cost"`
}

const runCols = `id::text, to_char(period,'YYYY-MM'), status, prepared_by, approved_by, paid_by, total_gross, total_net, total_cost, created_at, approved_at, paid_at`

func scanRun(row pgx.Row) (*Run, error) {
	var x Run
	err := row.Scan(&x.ID, &x.Period, &x.Status, &x.PreparedBy, &x.ApprovedBy, &x.PaidBy, &x.TotalGross, &x.TotalNet, &x.TotalCost, &x.CreatedAt, &x.ApprovedAt, &x.PaidAt)
	if isNF(err) {
		return nil, nf("payroll run")
	}
	return &x, err
}

const slipCols = `p.id::text, p.run_id::text, to_char(p.period,'YYYY-MM'), p.actor, p.full_name, p.gross, p.deductions, p.irpp, p.cac, p.net, p.employer_lines, p.employer_cost`

func scanSlip(row pgx.Row) (*Payslip, error) {
	var p Payslip
	var d, e []byte
	if err := row.Scan(&p.ID, &p.RunID, &p.Period, &p.Actor, &p.FullName, &p.Gross, &d, &p.IRPP, &p.CAC, &p.Net, &e, &p.EmployerCost); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(d, &p.Deductions); err != nil {
		return nil, err
	}
	return &p, json.Unmarshal(e, &p.Employer)
}

func slipsOf(ctx context.Context, s *platform.Scope, runID string) ([]Payslip, error) {
	rows, err := s.Tx.Query(ctx, `SELECT `+slipCols+` FROM payroll.payslips p WHERE p.run_id = $1 ORDER BY p.full_name`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Payslip{}
	for rows.Next() {
		p, err := scanSlip(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (m *mod) createRun(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Period string `json:"period"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	period, err := time.Parse("2006-01", in.Period)
	if err != nil {
		return nil, bad("period must look like 2026-10")
	}
	if period.After(time.Now().UTC()) {
		return nil, bad("a payroll run cannot be for a future month")
	}
	ctx := r.Context()
	cv, err := loadConfig(ctx, s)
	if err != nil {
		return nil, err
	}
	if cv.ValidatedBy == nil {
		return nil, platform.Err(http.StatusConflict, "Rules not validated", "the payroll rules must be checked and validated by a second person before any run")
	}
	rawCfg, _ := json.Marshal(cv.Config)
	var id string
	if err := s.Tx.QueryRow(ctx, `INSERT INTO payroll.runs (period, config, prepared_by) VALUES ($1,$2,$3) RETURNING id::text`, period, rawCfg, s.Claims.Sub).Scan(&id); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return nil, platform.Err(http.StatusConflict, "Run exists", "a payroll run already exists for "+in.Period)
		}
		return nil, guardErr(err)
	}
	rows, err := s.Tx.Query(ctx, `SELECT e.id::text, e.actor, e.full_name, c.base_salary + c.allowances FROM hr.employees e
		LEFT JOIN LATERAL (SELECT base_salary, allowances FROM payroll.compensation WHERE employee_id = e.id AND effective_from <= $1::date + interval '1 month' - interval '1 day' ORDER BY effective_from DESC LIMIT 1) c ON true
		WHERE e.status = 'active' AND e.hire_date < $1::date + interval '1 month' ORDER BY e.full_name`, period)
	if err != nil {
		return nil, err
	}
	type emp struct {
		id, actor, name string
		gross           *int64
	}
	var emps []emp
	for rows.Next() {
		var e emp
		if err := rows.Scan(&e.id, &e.actor, &e.name, &e.gross); err != nil {
			rows.Close()
			return nil, err
		}
		emps = append(emps, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var tg, tn, tc int64
	skipped := []string{}
	for _, e := range emps {
		if e.gross == nil {
			skipped = append(skipped, e.name)
			continue
		}
		sl := Compute(*e.gross, cv.Config)
		d, _ := json.Marshal(sl.Deductions)
		el, _ := json.Marshal(sl.Employer)
		if _, err := s.Tx.Exec(ctx, `INSERT INTO payroll.payslips (run_id, period, employee_id, actor, full_name, gross, deductions, irpp, cac, net, employer_lines, employer_cost)
			VALUES ($1,$12,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, id, e.id, e.actor, e.name, sl.Gross, d, sl.IRPP, sl.CAC, sl.Net, el, sl.EmployerCost, period); err != nil {
			return nil, err
		}
		tg += sl.Gross
		tn += sl.Net
		tc += sl.EmployerCost
	}
	if tg == 0 {
		return nil, bad("no active employee has a salary for that month; set compensation first")
	}
	if _, err := s.Tx.Exec(ctx, `UPDATE payroll.runs SET total_gross=$2, total_net=$3, total_cost=$4 WHERE id=$1`, id, tg, tn, tc); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "create", "payroll_run", id); err != nil {
		return nil, err
	}
	run, err := scanRun(s.Tx.QueryRow(ctx, `SELECT `+runCols+` FROM payroll.runs WHERE id=$1`, id))
	if err != nil {
		return nil, err
	}
	run.Skipped = skipped
	if run.Payslips, err = slipsOf(ctx, s, id); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: run}, nil
}

func (m *mod) listRuns(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+runCols+` FROM payroll.runs ORDER BY period DESC LIMIT 60`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Run{}
	for rows.Next() {
		x, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func (m *mod) getRun(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	run, err := scanRun(s.Tx.QueryRow(r.Context(), `SELECT `+runCols+` FROM payroll.runs WHERE id=$1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	if run.Payslips, err = slipsOf(r.Context(), s, run.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: run}, nil
}

func (m *mod) discardRun(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	if _, err := s.Tx.Exec(ctx, `DELETE FROM payroll.payslips WHERE run_id=$1`, r.PathValue("id")); err != nil {
		if isNF(err) {
			return nil, nf("payroll run")
		}
		return nil, guardErr(err)
	}
	tag, err := s.Tx.Exec(ctx, `DELETE FROM payroll.runs WHERE id=$1 AND status='draft'`, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, platform.Err(http.StatusConflict, "Wrong state", "only a draft run can be discarded")
	}
	if err := s.Audit(ctx, "discard", "payroll_run", r.PathValue("id")); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusNoContent}, nil
}

func (m *mod) entry(ctx context.Context, s *platform.Scope, run *Run) (RunEntry, error) {
	slips, err := slipsOf(ctx, s, run.ID)
	if err != nil {
		return RunEntry{}, err
	}
	period, _ := time.Parse("2006-01", run.Period)
	e := RunEntry{Period: period.AddDate(0, 1, -1), Gross: run.TotalGross, Net: run.TotalNet}
	for _, p := range slips {
		for _, l := range p.Deductions {
			e.EmployeeContrib += l.Amount
		}
		for _, l := range p.Employer {
			e.EmployerContrib += l.Amount
		}
		e.Tax += p.IRPP + p.CAC
	}
	return e, nil
}

func (m *mod) approve(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	run, err := scanRun(s.Tx.QueryRow(ctx, `UPDATE payroll.runs SET status='approved', approved_by=$2, approved_at=now() WHERE id=$1 AND status='draft' RETURNING `+runCols, r.PathValue("id"), s.Claims.Sub))
	if err != nil {
		var he *platform.HTTPError
		if errors.As(err, &he) { // not found, or not a draft
			if _, e2 := scanRun(s.Tx.QueryRow(ctx, `SELECT `+runCols+` FROM payroll.runs WHERE id=$1`, r.PathValue("id"))); e2 == nil {
				return nil, platform.Err(http.StatusConflict, "Wrong state", "only a draft run can be approved")
			}
			return nil, err
		}
		return nil, guardErr(err)
	}
	e, err := m.entry(ctx, s, run)
	if err != nil {
		return nil, err
	}
	if err := m.poster.RunApproved(ctx, s, e); err != nil {
		return nil, err
	}
	if err := issueSlipDocs(ctx, s, run); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "approve", "payroll_run", run.ID); err != nil {
		return nil, err
	}
	if err := s.Emit(ctx, "payroll.run.approved", map[string]any{"run_id": run.ID, "period": run.Period}); err != nil {
		return nil, err
	}
	if run.Payslips, err = slipsOf(ctx, s, run.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: run}, nil
}

func (m *mod) pay(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	run, err := scanRun(s.Tx.QueryRow(ctx, `UPDATE payroll.runs SET status='paid', paid_by=$2, paid_at=now() WHERE id=$1 AND status='approved' RETURNING `+runCols, r.PathValue("id"), s.Claims.Sub))
	if err != nil {
		var he *platform.HTTPError
		if errors.As(err, &he) {
			if _, e2 := scanRun(s.Tx.QueryRow(ctx, `SELECT `+runCols+` FROM payroll.runs WHERE id=$1`, r.PathValue("id"))); e2 == nil {
				return nil, platform.Err(http.StatusConflict, "Wrong state", "only an approved run can be paid")
			}
			return nil, err
		}
		return nil, guardErr(err)
	}
	e, err := m.entry(ctx, s, run)
	if err != nil {
		return nil, err
	}
	if err := m.poster.RunPaid(ctx, s, e); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "pay", "payroll_run", run.ID); err != nil {
		return nil, err
	}
	if err := s.Emit(ctx, "payroll.run.paid", map[string]any{"run_id": run.ID, "period": run.Period}); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: run}, nil
}

// mySlips are the caller's own payslips, once their run is approved.
func (m *mod) mySlips(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+slipCols+` FROM payroll.payslips p
		WHERE p.actor = $1 ORDER BY p.period DESC LIMIT 36`, s.Claims.Sub)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Payslip{}
	for rows.Next() {
		p, err := scanSlip(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}
