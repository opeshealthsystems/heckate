// Package hr is the HR context: employee records, leave and expense claims.
// Payroll (CNPS, IRPP) is intentionally absent: its rates must be supplied and checked by the accountant.
package hr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

var (
	hrRoles   = []string{"hr", "ceo"}
	reviewers = []string{"hr", "manager", "ceo"}
	staff     = []string{"ceo", "manager", "finance", "sales", "support", "engineer", "hr", "employee", "logistics", "procurement"}
)

func Register(a *platform.App) {
	a.Route("POST /v1/employees", hrRoles, platform.Idempotent(createEmployee))
	a.Route("GET /v1/employees", reviewers, listEmployees)
	a.Route("GET /v1/employees/me", staff, myRecord)
	a.Route("POST /v1/holidays", hrRoles, platform.Idempotent(addHoliday))

	a.Route("POST /v1/leave", staff, platform.Idempotent(requestLeave))
	a.Route("GET /v1/leave", staff, listLeave)
	a.Route("POST /v1/leave/{id}/approve", reviewers, platform.Idempotent(decideLeave("approved")))
	a.Route("POST /v1/leave/{id}/reject", reviewers, platform.Idempotent(decideLeave("rejected")))
	a.Route("POST /v1/leave/{id}/cancel", staff, platform.Idempotent(cancelLeave))

	a.Route("POST /v1/expenses", staff, platform.Idempotent(createClaim))
	a.Route("GET /v1/expenses", staff, listClaims)
	a.Route("POST /v1/expenses/{id}/approve", reviewers, platform.Idempotent(decideClaim("approved")))
	a.Route("POST /v1/expenses/{id}/reject", reviewers, platform.Idempotent(decideClaim("rejected")))
	a.Route("POST /v1/expenses/{id}/pay", []string{"finance", "ceo", "manager"}, platform.Idempotent(payClaim))
}

func nf(what string) error { return platform.Err(http.StatusNotFound, "Not found", what+" not found") }
func bad(msg string) error {
	return platform.Err(http.StatusUnprocessableEntity, "Invalid request", msg)
}
func isNF(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || (err != nil && strings.Contains(err.Error(), "invalid input syntax"))
}

// ---- employees ----

type Employee struct {
	ID         string    `json:"id"`
	Actor      string    `json:"actor"`
	FullName   string    `json:"full_name"`
	Position   string    `json:"position"`
	Department string    `json:"department"`
	HireDate   time.Time `json:"hire_date"`
	Status     string    `json:"status"`
	LeaveDays  *int      `json:"annual_leave_entitlement,omitempty"`
	LeaveTaken *int      `json:"annual_leave_taken,omitempty"`
	LeaveLeft  *int      `json:"annual_leave_balance,omitempty"`
}

const empCols = `id, actor, full_name, position, department, hire_date, status`

func scanEmp(row pgx.Row) (*Employee, error) {
	var e Employee
	err := row.Scan(&e.ID, &e.Actor, &e.FullName, &e.Position, &e.Department, &e.HireDate, &e.Status)
	if isNF(err) {
		return nil, nf("employee")
	}
	return &e, err
}

func createEmployee(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Actor      string `json:"actor"`
		FullName   string `json:"full_name"`
		Position   string `json:"position"`
		Department string `json:"department"`
		HireDate   string `json:"hire_date"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	hire, err := time.Parse("2006-01-02", in.HireDate)
	if err != nil || strings.TrimSpace(in.Actor) == "" || strings.TrimSpace(in.FullName) == "" {
		return nil, bad("actor (the sign-in subject), full_name and hire_date (YYYY-MM-DD) are required")
	}
	e, err := scanEmp(s.Tx.QueryRow(r.Context(), `INSERT INTO hr.employees (actor, full_name, position, department, hire_date) VALUES ($1,$2,$3,$4,$5) RETURNING `+empCols,
		strings.TrimSpace(in.Actor), strings.TrimSpace(in.FullName), in.Position, in.Department, hire))
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return nil, platform.Err(http.StatusConflict, "Exists", "an employee record already exists for this account")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "create", "employee", e.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: e}, nil
}

func listEmployees(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+empCols+` FROM hr.employees ORDER BY full_name LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Employee{}
	for rows.Next() {
		e, err := scanEmp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

// balance computes annual leave entitlement, days taken (approved) and days left for a year.
func balance(ctx context.Context, s *platform.Scope, actor string, hire time.Time, year int) (ent, taken, left int, err error) {
	var annual int
	if err = s.Tx.QueryRow(ctx, `SELECT annual_leave_days FROM hr.settings`).Scan(&annual); err != nil {
		return
	}
	ent = Entitlement(annual, hire, year)
	err = s.Tx.QueryRow(ctx, `SELECT coalesce(sum(days),0) FROM hr.leave_requests WHERE actor=$1 AND kind='annual' AND status='approved' AND extract(year from start_date)=$2`, actor, year).Scan(&taken)
	left = ent - taken
	return
}

func myRecord(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	e, err := scanEmp(s.Tx.QueryRow(ctx, `SELECT `+empCols+` FROM hr.employees WHERE actor=$1`, s.Claims.Sub))
	if err != nil {
		return nil, err
	}
	ent, taken, left, err := balance(ctx, s, e.Actor, e.HireDate, time.Now().UTC().Year())
	if err != nil {
		return nil, err
	}
	e.LeaveDays, e.LeaveTaken, e.LeaveLeft = &ent, &taken, &left
	return &platform.Result{Status: http.StatusOK, Body: e}, nil
}

func addHoliday(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Day  string `json:"day"`
		Name string `json:"name"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	day, err := time.Parse("2006-01-02", in.Day)
	if err != nil || strings.TrimSpace(in.Name) == "" {
		return nil, bad("day (YYYY-MM-DD) and name are required")
	}
	if _, err := s.Tx.Exec(r.Context(), `INSERT INTO hr.holidays (day, name) VALUES ($1,$2) ON CONFLICT (day) DO UPDATE SET name=EXCLUDED.name`, day, strings.TrimSpace(in.Name)); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: map[string]string{"day": in.Day, "name": in.Name}}, nil
}

// ---- leave ----

type Leave struct {
	ID        string    `json:"id"`
	Actor     string    `json:"actor"`
	Kind      string    `json:"kind"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Days      int       `json:"days"`
	Reason    string    `json:"reason"`
	Status    string    `json:"status"`
	DecidedBy *string   `json:"decided_by"`
	CreatedAt time.Time `json:"created_at"`
}

const leaveCols = `id, actor, kind, start_date, end_date, days, reason, status, decided_by, created_at`

func scanLeave(row pgx.Row) (*Leave, error) {
	var l Leave
	err := row.Scan(&l.ID, &l.Actor, &l.Kind, &l.StartDate, &l.EndDate, &l.Days, &l.Reason, &l.Status, &l.DecidedBy, &l.CreatedAt)
	if isNF(err) {
		return nil, nf("leave request")
	}
	return &l, err
}

func holidaySet(ctx context.Context, s *platform.Scope, from, to time.Time) (map[string]bool, error) {
	rows, err := s.Tx.Query(ctx, `SELECT day FROM hr.holidays WHERE day BETWEEN $1 AND $2`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out[d.Format("2006-01-02")] = true
	}
	return out, rows.Err()
}

func requestLeave(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Kind   string `json:"kind"`
		Start  string `json:"start_date"`
		End    string `json:"end_date"`
		Reason string `json:"reason"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	start, e1 := time.Parse("2006-01-02", in.Start)
	end, e2 := time.Parse("2006-01-02", in.End)
	if e1 != nil || e2 != nil || end.Before(start) || end.Sub(start) > 366*24*time.Hour {
		return nil, bad("start_date and end_date must be YYYY-MM-DD, end not before start, within a year")
	}
	switch in.Kind {
	case "annual", "sick", "unpaid", "other":
	default:
		return nil, bad("kind must be annual, sick, unpaid or other")
	}
	ctx := r.Context()
	emp, err := scanEmp(s.Tx.QueryRow(ctx, `SELECT `+empCols+` FROM hr.employees WHERE actor=$1`, s.Claims.Sub))
	if err != nil {
		return nil, platform.Err(http.StatusConflict, "No employee record", "ask HR to create your employee record first")
	}
	hol, err := holidaySet(ctx, s, start, end)
	if err != nil {
		return nil, err
	}
	days := WorkingDays(start, end, hol)
	if days == 0 {
		return nil, bad("that range has no working days")
	}
	rows, err := s.Tx.Query(ctx, `SELECT start_date, end_date FROM hr.leave_requests WHERE actor=$1 AND status IN ('pending','approved')`, s.Claims.Sub)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var a, b time.Time
		if err := rows.Scan(&a, &b); err != nil {
			rows.Close()
			return nil, err
		}
		if Overlaps(start, end, a, b) {
			rows.Close()
			return nil, platform.Err(http.StatusConflict, "Overlap", "you already have leave requested or approved on those dates")
		}
	}
	rows.Close()
	if in.Kind == "annual" {
		if start.Year() != end.Year() {
			return nil, bad("annual leave cannot span two calendar years; request each year separately")
		}
		_, _, left, err := balance(ctx, s, emp.Actor, emp.HireDate, start.Year())
		if err != nil {
			return nil, err
		}
		// Pending requests count against the balance so it cannot be oversubscribed.
		var pending int
		if err := s.Tx.QueryRow(ctx, `SELECT coalesce(sum(days),0) FROM hr.leave_requests WHERE actor=$1 AND kind='annual' AND status='pending' AND extract(year from start_date)=$2`, emp.Actor, start.Year()).Scan(&pending); err != nil {
			return nil, err
		}
		if days > left-pending {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Not enough leave", fmt.Sprintf("%d days requested, %d available", days, left-pending))
		}
	}
	l, err := scanLeave(s.Tx.QueryRow(ctx, `INSERT INTO hr.leave_requests (actor, kind, start_date, end_date, days, reason) VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+leaveCols,
		s.Claims.Sub, in.Kind, start, end, days, in.Reason))
	if err != nil {
		return nil, err
	}
	if err := s.Emit(ctx, "leave.requested", map[string]any{"id": l.ID, "actor": l.Actor, "days": l.Days}); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: l}, nil
}

func listLeave(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	q := `SELECT ` + leaveCols + ` FROM hr.leave_requests`
	var args []any
	if r.URL.Query().Get("all") != "1" || !s.Claims.HasAny(reviewers...) {
		q += ` WHERE actor=$1`
		args = append(args, s.Claims.Sub)
	}
	rows, err := s.Tx.Query(r.Context(), q+` ORDER BY start_date DESC LIMIT 500`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Leave{}
	for rows.Next() {
		l, err := scanLeave(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func decideLeave(to string) platform.HandlerFunc {
	return func(r *http.Request, s *platform.Scope) (*platform.Result, error) {
		ctx := r.Context()
		cur, err := scanLeave(s.Tx.QueryRow(ctx, `SELECT `+leaveCols+` FROM hr.leave_requests WHERE id::text=$1 FOR UPDATE`, r.PathValue("id")))
		if err != nil {
			return nil, err
		}
		if cur.Status != "pending" {
			return nil, platform.Err(http.StatusConflict, "Already decided", "only a pending request can be decided")
		}
		if cur.Actor == s.Claims.Sub {
			return nil, platform.Err(http.StatusForbidden, "Forbidden", "you cannot decide your own leave")
		}
		l, err := scanLeave(s.Tx.QueryRow(ctx, `UPDATE hr.leave_requests SET status=$2, decided_by=$3, decided_at=now() WHERE id=$1 RETURNING `+leaveCols, cur.ID, to, s.Claims.Sub))
		if err != nil {
			return nil, err
		}
		if err := s.Emit(ctx, "leave."+to, map[string]any{"id": l.ID, "actor": l.Actor}); err != nil {
			return nil, err
		}
		if err := s.Audit(ctx, to, "leave", l.ID); err != nil {
			return nil, err
		}
		return &platform.Result{Status: http.StatusOK, Body: l}, nil
	}
}

func cancelLeave(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	l, err := scanLeave(s.Tx.QueryRow(r.Context(), `UPDATE hr.leave_requests SET status='cancelled' WHERE id::text=$1 AND actor=$2 AND status='pending' RETURNING `+leaveCols, r.PathValue("id"), s.Claims.Sub))
	if err != nil {
		var he *platform.HTTPError
		if errors.As(err, &he) {
			return nil, platform.Err(http.StatusConflict, "Cannot cancel", "only your own pending requests can be cancelled")
		}
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: l}, nil
}

// ---- expense claims ----

type Claim struct {
	ID          string    `json:"id"`
	Actor       string    `json:"actor"`
	ExpenseDate time.Time `json:"expense_date"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Amount      int64     `json:"amount"`
	Status      string    `json:"status"`
	DecidedBy   *string   `json:"decided_by"`
	PaidBy      *string   `json:"paid_by"`
	CreatedAt   time.Time `json:"created_at"`
}

const claimCols = `id, actor, expense_date, category, description, amount, status, decided_by, paid_by, created_at`

func scanClaim(row pgx.Row) (*Claim, error) {
	var c Claim
	err := row.Scan(&c.ID, &c.Actor, &c.ExpenseDate, &c.Category, &c.Description, &c.Amount, &c.Status, &c.DecidedBy, &c.PaidBy, &c.CreatedAt)
	if isNF(err) {
		return nil, nf("expense claim")
	}
	return &c, err
}

func createClaim(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Date        string `json:"expense_date"`
		Category    string `json:"category"`
		Description string `json:"description"`
		Amount      int64  `json:"amount"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	d, err := time.Parse("2006-01-02", in.Date)
	if err != nil || d.After(time.Now().UTC().AddDate(0, 0, 1)) || d.Before(time.Now().UTC().AddDate(0, -6, 0)) {
		return nil, bad("expense_date must be within the last six months, YYYY-MM-DD")
	}
	if in.Amount <= 0 || in.Amount > 100_000_000 || strings.TrimSpace(in.Category) == "" || strings.TrimSpace(in.Description) == "" {
		return nil, bad("category, description and a positive amount in XAF are required")
	}
	c, err := scanClaim(s.Tx.QueryRow(r.Context(), `INSERT INTO hr.expense_claims (actor, expense_date, category, description, amount) VALUES ($1,$2,$3,$4,$5) RETURNING `+claimCols,
		s.Claims.Sub, d, strings.TrimSpace(in.Category), strings.TrimSpace(in.Description), in.Amount))
	if err != nil {
		return nil, err
	}
	if err := s.Emit(r.Context(), "expense.submitted", map[string]any{"id": c.ID, "amount": c.Amount}); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: c}, nil
}

func listClaims(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	q := `SELECT ` + claimCols + ` FROM hr.expense_claims`
	var args []any
	if r.URL.Query().Get("all") != "1" || !s.Claims.HasAny("hr", "manager", "ceo", "finance") {
		q += ` WHERE actor=$1`
		args = append(args, s.Claims.Sub)
	}
	rows, err := s.Tx.Query(r.Context(), q+` ORDER BY expense_date DESC LIMIT 500`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Claim{}
	for rows.Next() {
		c, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func lockClaim(r *http.Request, s *platform.Scope) (*Claim, error) {
	return scanClaim(s.Tx.QueryRow(r.Context(), `SELECT `+claimCols+` FROM hr.expense_claims WHERE id::text=$1 FOR UPDATE`, r.PathValue("id")))
}

func decideClaim(to string) platform.HandlerFunc {
	return func(r *http.Request, s *platform.Scope) (*platform.Result, error) {
		cur, err := lockClaim(r, s)
		if err != nil {
			return nil, err
		}
		if cur.Status != "submitted" {
			return nil, platform.Err(http.StatusConflict, "Already decided", "only a submitted claim can be decided")
		}
		if cur.Actor == s.Claims.Sub {
			return nil, platform.Err(http.StatusForbidden, "Forbidden", "you cannot decide your own expense claim")
		}
		c, err := scanClaim(s.Tx.QueryRow(r.Context(), `UPDATE hr.expense_claims SET status=$2, decided_by=$3 WHERE id=$1 RETURNING `+claimCols, cur.ID, to, s.Claims.Sub))
		if err != nil {
			return nil, err
		}
		if err := s.Audit(r.Context(), to, "expense", c.ID); err != nil {
			return nil, err
		}
		return &platform.Result{Status: http.StatusOK, Body: c}, nil
	}
}

func payClaim(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	cur, err := lockClaim(r, s)
	if err != nil {
		return nil, err
	}
	if cur.Status != "approved" {
		return nil, platform.Err(http.StatusConflict, "Cannot pay", "only an approved claim can be paid")
	}
	if cur.DecidedBy != nil && *cur.DecidedBy == s.Claims.Sub {
		return nil, platform.Err(http.StatusForbidden, "Second person required", "the person who approved this claim cannot also pay it")
	}
	if cur.Actor == s.Claims.Sub {
		return nil, platform.Err(http.StatusForbidden, "Forbidden", "you cannot pay your own expense claim")
	}
	c, err := scanClaim(s.Tx.QueryRow(r.Context(), `UPDATE hr.expense_claims SET status='paid', paid_by=$2 WHERE id=$1 RETURNING `+claimCols, cur.ID, s.Claims.Sub))
	if err != nil {
		return nil, err
	}
	if err := s.Emit(r.Context(), "expense.paid", map[string]any{"id": c.ID, "amount": c.Amount, "actor": c.Actor}); err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "pay", "expense", c.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: c}, nil
}

// ClaimOwner returns who submitted an expense claim, if the caller can see it.
func ClaimOwner(ctx context.Context, s *platform.Scope, id string) (string, error) {
	var actor string
	err := s.Tx.QueryRow(ctx, `SELECT actor FROM hr.expense_claims WHERE id::text = $1`, id).Scan(&actor)
	if isNF(err) {
		return "", nf("expense claim")
	}
	return actor, err
}
