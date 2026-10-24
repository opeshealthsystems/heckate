package projects

import (
	"context"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

var costers = []string{"ceo", "manager", "finance"}

func registerProgress(a *platform.App) {
	a.Route("GET /v1/projects/portfolio", platform.Portal, portfolio)
	a.Route("POST /v1/projects/from-quote", managers, platform.Idempotent(fromQuote))
	a.Route("GET /v1/projects/{id}/progress", platform.Portal, progress)
	a.Route("POST /v1/projects/{id}/milestones", managers, platform.Idempotent(createMilestone))
	a.Route("POST /v1/milestones/{id}/done", append([]string{"engineer"}, managers...), platform.Idempotent(completeMilestone))
	a.Route("POST /v1/projects/{id}/costs", costers, platform.Idempotent(addCost))
	a.Route("DELETE /v1/costs/{id}", costers, deleteCost)
	a.Route("GET /v1/labour-rates", costers, listRates)
	a.Route("PUT /v1/labour-rates", costers, setRate)
}

type Milestone struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	DueDate *time.Time `json:"due_date"`
	Status  string     `json:"status"`
	DoneAt  *time.Time `json:"done_at"`
	Late    bool       `json:"late"`
}

type Cost struct {
	ID          string    `json:"id"`
	CostDate    time.Time `json:"cost_date"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Amount      int64     `json:"amount"`
	CreatedBy   string    `json:"created_by"`
}

// Percent is the share of the work that is done: finished tasks and milestones over all of them. A project with
// neither has no measurable progress, so it reads 0 until its status says it is done.
func Percent(tasksDone, tasks, msDone, ms int, status string) int {
	if status == "done" {
		return 100
	}
	total := tasks + ms
	if total == 0 {
		return 0
	}
	return int(math.Round(float64(tasksDone+msDone) * 100 / float64(total)))
}

type Progress struct {
	ProjectID      string      `json:"project_id"`
	Name           string      `json:"name"`
	Status         string      `json:"status"`
	Percent        int         `json:"percent"`
	TasksDone      int         `json:"tasks_done"`
	TasksTotal     int         `json:"tasks_total"`
	MilestonesDone int         `json:"milestones_done"`
	MilestonesAll  int         `json:"milestones_total"`
	LateMilestones int         `json:"late_milestones"`
	StartDate      *time.Time  `json:"start_date"`
	EndDate        *time.Time  `json:"end_date"`
	TimeElapsedPct *int        `json:"time_elapsed_percent"`
	Behind         bool        `json:"behind_schedule"`
	Budget         *int64      `json:"budget,omitempty"`
	Actual         *int64      `json:"actual,omitempty"`
	Labour         *Labour     `json:"labour,omitempty"`
	Remaining      *int64      `json:"remaining,omitempty"`
	OverBudget     *bool       `json:"over_budget,omitempty"`
	Milestones     []Milestone `json:"milestones"`
	Costs          *[]Cost     `json:"costs,omitempty"`
}

// Labour is approved timesheet time priced at the employee's rate effective on the work date. Hours with no rate
// are reported, never priced.
type Labour struct {
	Cost          int64   `json:"cost"`
	PricedHours   float64 `json:"priced_hours"`
	UnpricedHours float64 `json:"unpriced_hours"`
}

// labourSQL prices the approved timesheet hours of the project matching `where`. Columns: priced cost (numeric),
// priced hours, unpriced hours.
const labourSQL = `SELECT COALESCE(sum(t.hours * r.rate) FILTER (WHERE r.rate IS NOT NULL),0),
	COALESCE(sum(t.hours) FILTER (WHERE r.rate IS NOT NULL),0)::float8,
	COALESCE(sum(t.hours) FILTER (WHERE r.rate IS NULL),0)::float8
	FROM projects.timesheets t
	LEFT JOIN LATERAL (SELECT lr.rate FROM projects.labour_rates lr WHERE lr.actor=t.actor AND lr.effective_from<=t.work_date ORDER BY lr.effective_from DESC LIMIT 1) r ON true
	WHERE t.status='approved' AND t.project_id=`

func labourFor(ctx context.Context, tx pgx.Tx, project string) (Labour, error) {
	var l Labour
	var cost float64
	err := tx.QueryRow(ctx, `SELECT round(c)::float8, h, u FROM (`+labourSQL+`$1::uuid) x(c,h,u)`, project).Scan(&cost, &l.PricedHours, &l.UnpricedHours)
	l.Cost = int64(math.Round(cost))
	return l, err
}

func elapsed(start, end *time.Time, now time.Time) *int {
	if start == nil || end == nil || !end.After(*start) {
		return nil
	}
	p := int(math.Round(float64(now.Sub(*start)) * 100 / float64(end.Sub(*start))))
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return &p
}

func progress(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	p, err := scanProject(s.Tx.QueryRow(ctx, `SELECT `+projCols+` FROM projects.projects WHERE id::text=$1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	out := Progress{ProjectID: p.ID, Name: p.Name, Status: p.Status, StartDate: p.StartDate, EndDate: p.EndDate, Milestones: []Milestone{}}
	if err := s.Tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='done'), count(*) FROM projects.tasks WHERE project_id=$1`, p.ID).Scan(&out.TasksDone, &out.TasksTotal); err != nil {
		return nil, err
	}
	rows, err := s.Tx.Query(ctx, `SELECT id::text, title, due_date, status, done_at FROM projects.milestones WHERE project_id=$1 ORDER BY due_date NULLS LAST, created_at`, p.ID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for rows.Next() {
		var m Milestone
		if err := rows.Scan(&m.ID, &m.Title, &m.DueDate, &m.Status, &m.DoneAt); err != nil {
			rows.Close()
			return nil, err
		}
		m.Late = m.Status == "pending" && m.DueDate != nil && m.DueDate.Before(now.Truncate(24*time.Hour))
		out.MilestonesAll++
		if m.Status == "done" {
			out.MilestonesDone++
		}
		if m.Late {
			out.LateMilestones++
		}
		out.Milestones = append(out.Milestones, m)
	}
	rows.Close()
	out.Percent = Percent(out.TasksDone, out.TasksTotal, out.MilestonesDone, out.MilestonesAll, p.Status)
	out.TimeElapsedPct = elapsed(p.StartDate, p.EndDate, now)
	out.Behind = out.LateMilestones > 0 || (out.TimeElapsedPct != nil && p.Status != "done" && out.Percent+15 < *out.TimeElapsedPct)
	if s.Claims.HasAny(costers...) {
		costs := []Cost{}
		rows, err := s.Tx.Query(ctx, `SELECT id::text, cost_date, category, description, amount, created_by FROM projects.costs WHERE project_id=$1 ORDER BY cost_date DESC, created_at DESC`, p.ID)
		if err != nil {
			return nil, err
		}
		var actual int64
		for rows.Next() {
			var c Cost
			if err := rows.Scan(&c.ID, &c.CostDate, &c.Category, &c.Description, &c.Amount, &c.CreatedBy); err != nil {
				rows.Close()
				return nil, err
			}
			actual += c.Amount
			costs = append(costs, c)
		}
		rows.Close()
		lab, err := labourFor(ctx, s.Tx, p.ID)
		if err != nil {
			return nil, err
		}
		actual += lab.Cost
		out.Costs, out.Actual, out.Budget, out.Labour = &costs, &actual, p.Budget, &lab
		if p.Budget != nil {
			rem, over := *p.Budget-actual, actual > *p.Budget
			out.Remaining, out.OverBudget = &rem, &over
		}
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}

// portfolio is one line per project with its progress, for the projects overview.
func portfolio(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	tc, ta := s.TenantFilter("p.tenant_id", 1)
	showCost := s.Claims.HasAny(costers...)
	rows, err := s.Tx.Query(r.Context(), `SELECT p.id::text, p.name, p.status, p.start_date, p.end_date, p.budget,
		(SELECT count(*) FILTER (WHERE status='done') FROM projects.tasks t WHERE t.project_id=p.id),
		(SELECT count(*) FROM projects.tasks t WHERE t.project_id=p.id),
		(SELECT count(*) FILTER (WHERE status='done') FROM projects.milestones m WHERE m.project_id=p.id),
		(SELECT count(*) FROM projects.milestones m WHERE m.project_id=p.id),
		(SELECT count(*) FROM projects.milestones m WHERE m.project_id=p.id AND m.status='pending' AND m.due_date < current_date),
		(SELECT COALESCE(sum(amount),0) FROM projects.costs c WHERE c.project_id=p.id)
		+ (SELECT round(COALESCE(sum(t.hours * r.rate),0))::bigint FROM projects.timesheets t
			JOIN LATERAL (SELECT lr.rate FROM projects.labour_rates lr WHERE lr.actor=t.actor AND lr.effective_from<=t.work_date ORDER BY lr.effective_from DESC LIMIT 1) r ON true
			WHERE t.status='approved' AND t.project_id=p.id)
		FROM projects.projects p WHERE true`+tc+` ORDER BY p.created_at DESC LIMIT 200`, ta...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Progress{}
	now := time.Now().UTC()
	for rows.Next() {
		var x Progress
		var budget *int64
		var actual int64
		if err := rows.Scan(&x.ProjectID, &x.Name, &x.Status, &x.StartDate, &x.EndDate, &budget, &x.TasksDone, &x.TasksTotal, &x.MilestonesDone, &x.MilestonesAll, &x.LateMilestones, &actual); err != nil {
			return nil, err
		}
		x.Percent = Percent(x.TasksDone, x.TasksTotal, x.MilestonesDone, x.MilestonesAll, x.Status)
		x.TimeElapsedPct = elapsed(x.StartDate, x.EndDate, now)
		x.Behind = x.LateMilestones > 0 || (x.TimeElapsedPct != nil && x.Status != "done" && x.Percent+15 < *x.TimeElapsedPct)
		x.Milestones = []Milestone{}
		if showCost {
			x.Budget, x.Actual = budget, &actual
			if budget != nil {
				over := actual > *budget
				x.OverBudget = &over
			}
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func createMilestone(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Title   string `json:"title"`
		DueDate string `json:"due_date"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Title) == "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid milestone", "title is required")
	}
	due, err := parseDate(in.DueDate, "due_date")
	if err != nil {
		return nil, err
	}
	var m Milestone
	err = s.Tx.QueryRow(r.Context(), `INSERT INTO projects.milestones (project_id, tenant_id, title, due_date)
		SELECT id, tenant_id, $2, $3 FROM projects.projects WHERE id::text=$1 RETURNING id::text, title, due_date, status, done_at`,
		r.PathValue("id"), strings.TrimSpace(in.Title), due).Scan(&m.ID, &m.Title, &m.DueDate, &m.Status, &m.DoneAt)
	if isNotFound(err) {
		return nil, nf("project")
	}
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: m}, nil
}

func completeMilestone(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var m Milestone
	err := s.Tx.QueryRow(r.Context(), `UPDATE projects.milestones SET status='done', done_at=COALESCE(done_at, now()) WHERE id::text=$1 RETURNING id::text, title, due_date, status, done_at`,
		r.PathValue("id")).Scan(&m.ID, &m.Title, &m.DueDate, &m.Status, &m.DoneAt)
	if isNotFound(err) {
		return nil, nf("milestone")
	}
	if err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "complete", "milestone", m.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: m}, nil
}

func addCost(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		CostDate    string `json:"cost_date"`
		Category    string `json:"category"`
		Description string `json:"description"`
		Amount      int64  `json:"amount"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Description) == "" || in.Amount <= 0 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid cost", "description and a positive amount are required")
	}
	if in.Category == "" {
		in.Category = "other"
	}
	d := time.Now().UTC()
	if in.CostDate != "" {
		var err error
		if d, err = time.Parse("2006-01-02", in.CostDate); err != nil {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid cost", "cost_date must be YYYY-MM-DD")
		}
	}
	var c Cost
	err := s.Tx.QueryRow(r.Context(), `INSERT INTO projects.costs (project_id, tenant_id, cost_date, category, description, amount, created_by)
		SELECT id, tenant_id, $2, $3, $4, $5, $6 FROM projects.projects WHERE id::text=$1 RETURNING id::text, cost_date, category, description, amount, created_by`,
		r.PathValue("id"), d, in.Category, strings.TrimSpace(in.Description), in.Amount, s.Claims.Sub).Scan(&c.ID, &c.CostDate, &c.Category, &c.Description, &c.Amount, &c.CreatedBy)
	if isNotFound(err) {
		return nil, nf("project")
	}
	if err != nil {
		if strings.Contains(err.Error(), "check constraint") {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid cost", "category must be labour, materials, travel, subcontract or other")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "create", "project_cost", c.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: c}, nil
}

type Rate struct {
	Actor         string    `json:"actor"`
	EffectiveFrom time.Time `json:"effective_from"`
	Rate          int64     `json:"rate"`
	SetBy         string    `json:"set_by"`
}

func listRates(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT actor, effective_from, rate, set_by FROM projects.labour_rates ORDER BY actor, effective_from DESC LIMIT 2000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rate{}
	for rows.Next() {
		var x Rate
		if err := rows.Scan(&x.Actor, &x.EffectiveFrom, &x.Rate, &x.SetBy); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

// setRate records an hourly cost rate from a date. Setting the same date again corrects it; earlier dates keep
// their own rate, so past projects are not repriced.
func setRate(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Actor         string `json:"actor"`
		EffectiveFrom string `json:"effective_from"`
		Rate          int64  `json:"rate"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	in.Actor = strings.TrimSpace(in.Actor)
	if in.Actor == "" || in.Rate <= 0 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid rate", "actor and a positive rate (XAF per hour) are required")
	}
	d, err := time.Parse("2006-01-02", in.EffectiveFrom)
	if err != nil {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid rate", "effective_from must be YYYY-MM-DD")
	}
	var x Rate
	err = s.Tx.QueryRow(r.Context(), `INSERT INTO projects.labour_rates (actor, effective_from, rate, set_by) VALUES ($1,$2,$3,$4)
		ON CONFLICT (actor, effective_from) DO UPDATE SET rate=EXCLUDED.rate, set_by=EXCLUDED.set_by
		RETURNING actor, effective_from, rate, set_by`, in.Actor, d, in.Rate, s.Claims.Sub).Scan(&x.Actor, &x.EffectiveFrom, &x.Rate, &x.SetBy)
	if err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "set", "labour_rate", in.Actor+"@"+in.EffectiveFrom); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: x}, nil
}

func deleteCost(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	tag, err := s.Tx.Exec(r.Context(), `DELETE FROM projects.costs WHERE id::text=$1`, r.PathValue("id"))
	if err != nil {
		if isNotFound(err) {
			return nil, nf("cost")
		}
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, nf("cost")
	}
	if err := s.Audit(r.Context(), "delete", "project_cost", r.PathValue("id")); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusNoContent}, nil
}

// fromQuote starts a project from an approved quotation: the client, the name and the budget come from the quote,
// and each quote line becomes a task so nothing sold is forgotten.
func fromQuote(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		QuoteID   string `json:"quote_id"`
		Manager   string `json:"manager"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	start, err := parseDate(in.StartDate, "start_date")
	if err != nil {
		return nil, err
	}
	end, err := parseDate(in.EndDate, "end_date")
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	var tenant, title, status string
	var budget int64
	err = s.Tx.QueryRow(ctx, `SELECT q.tenant_id::text, q.title, q.status, COALESCE((SELECT round(sum(l.quantity * l.unit_price)) FROM sales.quote_lines l WHERE l.quote_id = q.id),0)::bigint
		FROM sales.quotes q WHERE q.id::text=$1`, in.QuoteID).Scan(&tenant, &title, &status, &budget)
	if isNotFound(err) {
		return nil, platform.Err(http.StatusNotFound, "Not found", "quote not found")
	}
	if err != nil {
		return nil, err
	}
	if status != "approved" {
		return nil, platform.Err(http.StatusConflict, "Not approved", "a project starts from an approved quotation")
	}
	var dup bool
	if err := s.Tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM projects.projects WHERE quote_id::text=$1)`, in.QuoteID).Scan(&dup); err != nil {
		return nil, err
	}
	if dup {
		return nil, platform.Err(http.StatusConflict, "Already started", "a project already exists for this quotation")
	}
	p, err := scanProject(s.Tx.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id, name, manager, start_date, end_date, budget, created_by, quote_id)
		VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8) RETURNING `+projCols, tenant, title, in.Manager, start, end, budget, s.Claims.Sub, in.QuoteID))
	if err != nil {
		if strings.Contains(err.Error(), "check constraint") {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid project", "end_date cannot be before start_date")
		}
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `INSERT INTO projects.tasks (project_id, tenant_id, title) SELECT $1, $2, left(l.description, 200) FROM sales.quote_lines l WHERE l.quote_id::text=$3 ORDER BY l.position`, p.ID, tenant, in.QuoteID); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "create", "project", p.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: p}, nil
}
