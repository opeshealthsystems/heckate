// Package projects is the Projects & Field Service context.
package projects

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
	"github.com/opeshealthsystems/heckate/api/internal/modules/clients"
	"github.com/opeshealthsystems/heckate/api/internal/modules/documents"
	"github.com/opeshealthsystems/heckate/api/internal/modules/inventory"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

var (
	managers    = []string{"ceo", "manager"}
	dispatchers = []string{"ceo", "manager", "support"}
	staffRoles  = []string{"ceo", "manager", "finance", "sales", "support", "engineer", "hr", "employee"}
	fieldRoles  = []string{"ceo", "manager", "support", "engineer"}
)

var pdfOptions docs.Options

func Register(a *platform.App, o docs.Options) {
	pdfOptions = o
	a.Route("POST /v1/projects", managers, platform.Idempotent(createProject))
	a.Route("GET /v1/projects", platform.Portal, listProjects)
	a.Route("GET /v1/projects/{id}", platform.Portal, getProject)
	a.Route("PATCH /v1/projects/{id}", managers, updateProject)
	a.Route("POST /v1/projects/{id}/tasks", append([]string{"engineer"}, managers...), platform.Idempotent(createTask))
	a.Route("PATCH /v1/tasks/{id}", append([]string{"engineer"}, managers...), updateTask)

	a.Route("POST /v1/interventions", dispatchers, platform.Idempotent(createIntervention))
	a.Route("GET /v1/interventions", platform.Portal, listInterventions)
	a.Route("GET /v1/interventions/{id}", platform.Portal, getIntervention)
	a.Route("GET /v1/interventions/{id}/pdf", platform.Portal, interventionPDF)
	a.Route("POST /v1/interventions/{id}/check-in", []string{"engineer"}, platform.Idempotent(checkIn))
	a.Route("POST /v1/interventions/{id}/check-out", []string{"engineer"}, platform.Idempotent(checkOut))

	a.Route("POST /v1/timesheets", staffRoles, platform.Idempotent(createTimesheet))
	a.Route("GET /v1/timesheets", staffRoles, listTimesheets)
	a.Route("GET /v1/exports/timesheets", staffRoles, exportTimesheets)
	a.Route("POST /v1/timesheets/{id}/submit", staffRoles, platform.Idempotent(submitTimesheet))
	a.Route("POST /v1/timesheets/{id}/approve", managers, platform.Idempotent(approveTimesheet))
}

func nf(what string) error { return platform.Err(http.StatusNotFound, "Not found", what+" not found") }

func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || (err != nil && strings.Contains(err.Error(), "invalid input syntax"))
}

func parseDate(v, field string) (any, error) {
	if v == "" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid date", field+" must be YYYY-MM-DD")
	}
	return d, nil
}

// ---- projects and tasks ----

type Project struct {
	ID        string     `json:"id"`
	TenantID  string     `json:"tenant_id"`
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	Manager   *string    `json:"manager"`
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
	Budget    *int64     `json:"budget"`
	CreatedAt time.Time  `json:"created_at"`
	Tasks     *[]Task    `json:"tasks,omitempty"` // a pointer so an empty list is still sent on the detail view
}

type Task struct {
	ID       string     `json:"id"`
	Title    string     `json:"title"`
	Status   string     `json:"status"`
	Assignee *string    `json:"assignee"`
	DueDate  *time.Time `json:"due_date"`
}

const projCols = `id, tenant_id, name, status, manager, start_date, end_date, budget, created_at`

func scanProject(row pgx.Row) (*Project, error) {
	var p Project
	err := row.Scan(&p.ID, &p.TenantID, &p.Name, &p.Status, &p.Manager, &p.StartDate, &p.EndDate, &p.Budget, &p.CreatedAt)
	if isNotFound(err) {
		return nil, nf("project")
	}
	return &p, err
}

func createProject(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		TenantID  string `json:"tenant_id"`
		Name      string `json:"name"`
		Manager   string `json:"manager"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		Budget    *int64 `json:"budget"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.TenantID == "" || strings.TrimSpace(in.Name) == "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid project", "tenant_id and name are required")
	}
	start, err := parseDate(in.StartDate, "start_date")
	if err != nil {
		return nil, err
	}
	end, err := parseDate(in.EndDate, "end_date")
	if err != nil {
		return nil, err
	}
	p, err := scanProject(s.Tx.QueryRow(r.Context(), `INSERT INTO projects.projects (tenant_id, name, manager, start_date, end_date, budget, created_by)
		VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7) RETURNING `+projCols, in.TenantID, strings.TrimSpace(in.Name), in.Manager, start, end, in.Budget, s.Claims.Sub))
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "foreign key"):
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid project", "unknown tenant")
		case strings.Contains(err.Error(), "check constraint"):
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid project", "end_date cannot be before start_date and budget cannot be negative")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "create", "project", p.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: p}, nil
}

func listProjects(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	tc, ta := s.TenantFilter("tenant_id", 1)
	rows, err := s.Tx.Query(r.Context(), `SELECT `+projCols+` FROM projects.projects WHERE true`+tc+` ORDER BY created_at DESC LIMIT 200`, ta...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func getProject(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	p, err := scanProject(s.Tx.QueryRow(ctx, `SELECT `+projCols+` FROM projects.projects WHERE id::text=$1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	rows, err := s.Tx.Query(ctx, `SELECT id, title, status, assignee, due_date FROM projects.tasks WHERE project_id=$1 ORDER BY created_at`, p.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.Assignee, &t.DueDate); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	p.Tasks = &tasks
	return &platform.Result{Status: http.StatusOK, Body: p}, rows.Err()
}

func updateProject(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Status  *string `json:"status"`
		Manager *string `json:"manager"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	p, err := scanProject(s.Tx.QueryRow(r.Context(), `UPDATE projects.projects SET status=COALESCE($2,status), manager=COALESCE($3,manager) WHERE id::text=$1 RETURNING `+projCols,
		r.PathValue("id"), in.Status, in.Manager))
	if err != nil {
		if strings.Contains(err.Error(), "status_check") {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid project", "unknown status")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "update", "project", p.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: p}, nil
}

func createTask(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Title    string `json:"title"`
		Assignee string `json:"assignee"`
		DueDate  string `json:"due_date"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Title) == "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid task", "title is required")
	}
	due, err := parseDate(in.DueDate, "due_date")
	if err != nil {
		return nil, err
	}
	var t Task
	err = s.Tx.QueryRow(r.Context(), `INSERT INTO projects.tasks (project_id, tenant_id, title, assignee, due_date)
		SELECT id, tenant_id, $2, NULLIF($3,''), $4 FROM projects.projects WHERE id::text=$1
		RETURNING id, title, status, assignee, due_date`, r.PathValue("id"), strings.TrimSpace(in.Title), in.Assignee, due).
		Scan(&t.ID, &t.Title, &t.Status, &t.Assignee, &t.DueDate)
	if isNotFound(err) {
		return nil, nf("project")
	}
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: t}, nil
}

func updateTask(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Status   *string `json:"status"`
		Assignee *string `json:"assignee"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	var t Task
	err := s.Tx.QueryRow(r.Context(), `UPDATE projects.tasks SET status=COALESCE($2,status), assignee=COALESCE($3,assignee) WHERE id::text=$1
		RETURNING id, title, status, assignee, due_date`, r.PathValue("id"), in.Status, in.Assignee).Scan(&t.ID, &t.Title, &t.Status, &t.Assignee, &t.DueDate)
	if isNotFound(err) {
		return nil, nf("task")
	}
	if err != nil {
		if strings.Contains(err.Error(), "status_check") {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid task", "status must be todo, doing or done")
		}
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: t}, nil
}

// ---- interventions ----

type Intervention struct {
	ID           string          `json:"id"`
	TenantID     string          `json:"tenant_id"`
	ProjectID    *string         `json:"project_id"`
	TicketID     *string         `json:"ticket_id"`
	Engineer     string          `json:"engineer"`
	Status       string          `json:"status"`
	ScheduledFor *time.Time      `json:"scheduled_for"`
	Summary      string          `json:"summary"`
	CheckinAt    *time.Time      `json:"checkin_at"`
	CheckinLat   *float64        `json:"checkin_lat"`
	CheckinLng   *float64        `json:"checkin_lng"`
	CheckoutAt   *time.Time      `json:"checkout_at"`
	CheckoutLat  *float64        `json:"checkout_lat"`
	CheckoutLng  *float64        `json:"checkout_lng"`
	Report       *string         `json:"report"`
	SignedBy     *string         `json:"signed_by"`
	HasSignature bool            `json:"has_signature"`
	Signature    *string         `json:"signature,omitempty"`
	DocumentRef  *string         `json:"document_ref"`
	Parts        json.RawMessage `json:"parts"`
	CreatedAt    time.Time       `json:"created_at"`
}

const ivCols = `id, tenant_id, project_id, ticket_id, engineer, status, scheduled_for, summary, checkin_at, checkin_lat, checkin_lng,
 checkout_at, checkout_lat, checkout_lng, report, signed_by, signature IS NOT NULL, document_ref, parts, created_at`

func scanIV(row pgx.Row) (*Intervention, error) {
	var v Intervention
	err := row.Scan(&v.ID, &v.TenantID, &v.ProjectID, &v.TicketID, &v.Engineer, &v.Status, &v.ScheduledFor, &v.Summary, &v.CheckinAt, &v.CheckinLat, &v.CheckinLng,
		&v.CheckoutAt, &v.CheckoutLat, &v.CheckoutLng, &v.Report, &v.SignedBy, &v.HasSignature, &v.DocumentRef, &v.Parts, &v.CreatedAt)
	if isNotFound(err) {
		return nil, nf("intervention")
	}
	return &v, err
}

func createIntervention(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		TenantID     string `json:"tenant_id"`
		ProjectID    string `json:"project_id"`
		TicketID     string `json:"ticket_id"`
		Engineer     string `json:"engineer"`
		ScheduledFor string `json:"scheduled_for"`
		Summary      string `json:"summary"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.TenantID == "" || strings.TrimSpace(in.Engineer) == "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid intervention", "tenant_id and engineer are required")
	}
	var when any
	if in.ScheduledFor != "" {
		t, err := time.Parse(time.RFC3339, in.ScheduledFor)
		if err != nil {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid intervention", "scheduled_for must be an RFC 3339 timestamp")
		}
		when = t
	}
	v, err := scanIV(s.Tx.QueryRow(r.Context(), `INSERT INTO projects.interventions (tenant_id, project_id, ticket_id, engineer, scheduled_for, summary, created_by)
		VALUES ($1,NULLIF($2,'')::uuid,NULLIF($3,'')::uuid,$4,$5,$6,$7) RETURNING `+ivCols, in.TenantID, in.ProjectID, in.TicketID, strings.TrimSpace(in.Engineer), when, in.Summary, s.Claims.Sub))
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "invalid input syntax") {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid intervention", "unknown tenant or project")
		}
		return nil, err
	}
	if err := s.Emit(r.Context(), "intervention.scheduled", map[string]any{"id": v.ID, "engineer": v.Engineer, "tenant_id": v.TenantID}); err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "create", "intervention", v.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: v}, nil
}

func listInterventions(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	q := `SELECT ` + ivCols + ` FROM projects.interventions WHERE true`
	var args []any
	// Engineers see their own work; support, managers and clients (via RLS) see the rest.
	if s.Claims.HasAny("engineer") && !s.Claims.HasAny("ceo", "manager", "support") {
		args = append(args, s.Claims.Sub)
		q += fmt.Sprintf(` AND engineer = $%d`, len(args))
	}
	tc, ta := s.TenantFilter("tenant_id", len(args)+1)
	q += tc
	args = append(args, ta...)
	rows, err := s.Tx.Query(r.Context(), q+` ORDER BY created_at DESC LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Intervention{}
	for rows.Next() {
		v, err := scanIV(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func getIntervention(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	v, err := scanIV(s.Tx.QueryRow(r.Context(), `SELECT `+ivCols+` FROM projects.interventions WHERE id::text=$1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	if v.HasSignature {
		var sig string
		if err := s.Tx.QueryRow(r.Context(), `SELECT signature FROM projects.interventions WHERE id=$1`, v.ID).Scan(&sig); err != nil {
			return nil, err
		}
		v.Signature = &sig
	}
	return &platform.Result{Status: http.StatusOK, Body: v}, nil
}

func lockOwn(ctx context.Context, s *platform.Scope, id string) (*Intervention, error) {
	v, err := scanIV(s.Tx.QueryRow(ctx, `SELECT `+ivCols+` FROM projects.interventions WHERE id::text=$1 FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}
	if v.Engineer != s.Claims.Sub {
		return nil, platform.Err(http.StatusForbidden, "Forbidden", "this intervention is assigned to another engineer")
	}
	return v, nil
}

func checkIn(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
		At  string  `json:"at"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if !ValidCoord(in.Lat, in.Lng) {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid position", "lat and lng are out of range")
	}
	at, err := DeviceTime(in.At, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	v, err := lockOwn(ctx, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if v.Status != "scheduled" {
		return nil, platform.Err(http.StatusConflict, "Cannot check in", "only a scheduled intervention can be started")
	}
	nv, err := scanIV(s.Tx.QueryRow(ctx, `UPDATE projects.interventions SET status='in_progress', checkin_at=$2, checkin_lat=$3, checkin_lng=$4 WHERE id=$1 RETURNING `+ivCols, v.ID, at, in.Lat, in.Lng))
	if err != nil {
		return nil, err
	}
	if err := s.Emit(ctx, "intervention.started", map[string]any{"id": nv.ID, "tenant_id": nv.TenantID}); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "check_in", "intervention", nv.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: nv}, nil
}

func checkOut(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Lat       float64 `json:"lat"`
		Lng       float64 `json:"lng"`
		At        string  `json:"at"`
		Report    string  `json:"report"`
		SignedBy  string  `json:"signed_by"`
		Signature string  `json:"signature"`
		Parts     []struct {
			ItemID      string `json:"item_id"`
			WarehouseID string `json:"warehouse_id"`
			Qty         int    `json:"qty"`
		} `json:"parts"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if !ValidCoord(in.Lat, in.Lng) {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid position", "lat and lng are out of range")
	}
	if len(strings.TrimSpace(in.Report)) < 10 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Report required", "describe the work done (at least 10 characters)")
	}
	if (in.Signature == "") != (strings.TrimSpace(in.SignedBy) == "") {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Incomplete signature", "signed_by and signature must be provided together")
	}
	if in.Signature != "" && !ValidSignature(in.Signature) {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid signature", "signature must be a PNG data URL of at most 200 KB")
	}
	at, err := DeviceTime(in.At, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	v, err := lockOwn(ctx, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if v.Status != "in_progress" {
		return nil, platform.Err(http.StatusConflict, "Cannot check out", "check in first")
	}
	if v.CheckinAt != nil && at.Before(*v.CheckinAt) {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid time", "check-out cannot be before check-in")
	}
	// Spare parts used come out of stock in the same transaction as the completed report.
	for _, p := range in.Parts {
		if err := inventory.Issue(ctx, s, p.ItemID, p.WarehouseID, p.Qty, "intervention:"+v.ID); err != nil {
			return nil, err
		}
	}
	parts, _ := json.Marshal(in.Parts)
	if in.Parts == nil {
		parts = []byte("[]")
	}
	doc, err := documents.Issue(ctx, s, "FRM-27", "Intervention report and acceptance", v.TenantID, "")
	if err != nil {
		return nil, err
	}
	var sig, by any
	if in.Signature != "" {
		sig, by = in.Signature, strings.TrimSpace(in.SignedBy)
	}
	nv, err := scanIV(s.Tx.QueryRow(ctx, `UPDATE projects.interventions SET status='completed', checkout_at=$2, checkout_lat=$3, checkout_lng=$4,
		report=$5, signed_by=$6, signature=$7, document_ref=$8, parts=$9 WHERE id=$1 RETURNING `+ivCols, v.ID, at, in.Lat, in.Lng, strings.TrimSpace(in.Report), by, sig, doc.Ref, parts))
	if err != nil {
		return nil, err
	}
	if err := s.Emit(ctx, "intervention.completed", map[string]any{"id": nv.ID, "tenant_id": nv.TenantID, "ref": doc.Ref, "signed": nv.HasSignature}); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "check_out", "intervention", nv.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: nv}, nil
}

// ---- timesheets ----

type Timesheet struct {
	ID         string    `json:"id"`
	Actor      string    `json:"actor"`
	WorkDate   time.Time `json:"work_date"`
	Hours      float64   `json:"hours"`
	ProjectID  *string   `json:"project_id"`
	TicketID   *string   `json:"ticket_id"`
	Note       string    `json:"note"`
	Status     string    `json:"status"`
	ApprovedBy *string   `json:"approved_by"`
}

const tsCols = `id, actor, work_date, hours::float8, project_id, ticket_id, note, status, approved_by`

func scanTS(row pgx.Row) (*Timesheet, error) {
	var t Timesheet
	err := row.Scan(&t.ID, &t.Actor, &t.WorkDate, &t.Hours, &t.ProjectID, &t.TicketID, &t.Note, &t.Status, &t.ApprovedBy)
	if isNotFound(err) {
		return nil, nf("timesheet")
	}
	return &t, err
}

func createTimesheet(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		WorkDate  string  `json:"work_date"`
		Hours     float64 `json:"hours"`
		ProjectID string  `json:"project_id"`
		TicketID  string  `json:"ticket_id"`
		Note      string  `json:"note"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	d, err := time.Parse("2006-01-02", in.WorkDate)
	if err != nil || d.After(time.Now().UTC().AddDate(0, 0, 1)) {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid timesheet", "work_date must be a past or current date, YYYY-MM-DD")
	}
	if in.Hours <= 0 || in.Hours > 24 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid timesheet", "hours must be between 0 and 24")
	}
	// A person cannot log more than 24 hours in a day across entries.
	var logged float64
	if err := s.Tx.QueryRow(r.Context(), `SELECT coalesce(sum(hours),0)::float8 FROM projects.timesheets WHERE actor=$1 AND work_date=$2`, s.Claims.Sub, d).Scan(&logged); err != nil {
		return nil, err
	}
	if logged+in.Hours > 24 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Too many hours", "more than 24 hours logged for that day")
	}
	t, err := scanTS(s.Tx.QueryRow(r.Context(), `INSERT INTO projects.timesheets (actor, work_date, hours, project_id, ticket_id, note)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,$6) RETURNING `+tsCols, s.Claims.Sub, d, in.Hours, in.ProjectID, in.TicketID, in.Note))
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "invalid input syntax") {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid timesheet", "unknown project or invalid id")
		}
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: t}, nil
}

func listTimesheets(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	q := `SELECT ` + tsCols + ` FROM projects.timesheets`
	var args []any
	reviewer := s.Claims.HasAny("ceo", "manager", "hr", "finance")
	if r.URL.Query().Get("all") != "1" || !reviewer {
		q += ` WHERE actor = $1`
		args = append(args, s.Claims.Sub)
	}
	rows, err := s.Tx.Query(r.Context(), q+` ORDER BY work_date DESC, id LIMIT 500`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Timesheet{}
	for rows.Next() {
		t, err := scanTS(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func submitTimesheet(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	t, err := scanTS(s.Tx.QueryRow(r.Context(), `UPDATE projects.timesheets SET status='submitted' WHERE id::text=$1 AND actor=$2 AND status='draft' RETURNING `+tsCols, r.PathValue("id"), s.Claims.Sub))
	if err != nil {
		var he *platform.HTTPError
		if errors.As(err, &he) {
			return nil, platform.Err(http.StatusConflict, "Cannot submit", "only your own draft timesheets can be submitted")
		}
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: t}, nil
}

func approveTimesheet(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	cur, err := scanTS(s.Tx.QueryRow(r.Context(), `SELECT `+tsCols+` FROM projects.timesheets WHERE id::text=$1 FOR UPDATE`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	if cur.Actor == s.Claims.Sub {
		return nil, platform.Err(http.StatusForbidden, "Forbidden", "you cannot approve your own timesheet")
	}
	if cur.Status != "submitted" {
		return nil, platform.Err(http.StatusConflict, "Cannot approve", "only a submitted timesheet can be approved")
	}
	t, err := scanTS(s.Tx.QueryRow(r.Context(), `UPDATE projects.timesheets SET status='approved', approved_by=$2 WHERE id=$1 RETURNING `+tsCols, cur.ID, s.Claims.Sub))
	if err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "approve", "timesheet", t.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: t}, nil
}

// InterventionTenant returns the tenant of an intervention the caller can see.
func InterventionTenant(ctx context.Context, s *platform.Scope, id string) (string, error) {
	var tenant string
	err := s.Tx.QueryRow(ctx, `SELECT tenant_id FROM projects.interventions WHERE id::text = $1`, id).Scan(&tenant)
	if isNotFound(err) {
		return "", nf("intervention")
	}
	return tenant, err
}

// interventionPDF serves the canonical acceptance document of a completed site visit.
func interventionPDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	v, err := scanIV(s.Tx.QueryRow(ctx, `SELECT `+ivCols+` FROM projects.interventions WHERE id::text=$1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	if v.Status != "completed" || v.DocumentRef == nil || v.CheckinAt == nil || v.CheckoutAt == nil {
		return nil, platform.Err(http.StatusConflict, "Not completed", "only a completed site visit has a PDF")
	}
	return documents.ServePDF(ctx, s, *v.DocumentRef, func() ([]byte, error) {
		client, err := clients.Party(ctx, s, v.TenantID)
		if err != nil {
			return nil, err
		}
		var dataURL *string
		if err := s.Tx.QueryRow(ctx, `SELECT signature FROM projects.interventions WHERE id=$1`, v.ID).Scan(&dataURL); err != nil {
			return nil, err
		}
		var sig []byte
		if dataURL != nil {
			// The upload was validated as a PNG data URL when the visit was completed.
			sig, _ = base64.StdEncoding.DecodeString(strings.TrimPrefix(*dataURL, "data:image/png;base64,"))
		}
		pos := func(lat, lng *float64) string {
			if lat == nil || lng == nil {
				return "position not recorded"
			}
			return fmt.Sprintf("%.5f, %.5f", *lat, *lng)
		}
		signedBy := ""
		if v.SignedBy != nil {
			signedBy = *v.SignedBy
		}
		report := ""
		if v.Report != nil {
			report = *v.Report
		}
		return docs.RenderVisit(pdfOptions, *v.DocumentRef, client, docs.Visit{
			Engineer: v.Engineer, Summary: v.Summary, CheckIn: *v.CheckinAt, CheckOut: *v.CheckoutAt,
			InPos: pos(v.CheckinLat, v.CheckinLng), OutPos: pos(v.CheckoutLat, v.CheckoutLng),
			Report: report, SignedBy: signedBy, Signature: sig,
		})
	})
}
