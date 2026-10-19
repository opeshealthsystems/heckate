// Package support is the Support & Incidents bounded context.
// It owns the "support" schema; other modules talk to it through events or this API only.
package support

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// Severity is the incident severity, P1 (critical) to P4 (low).
type Severity string

const (
	P1 Severity = "P1"
	P2 Severity = "P2"
	P3 Severity = "P3"
	P4 Severity = "P4"
)

// Valid reports whether s is one of P1..P4.
func (s Severity) Valid() bool {
	switch s {
	case P1, P2, P3, P4:
		return true
	}
	return false
}

var channels = map[string]bool{"app": true, "portal": true, "email": true, "phone": true}

// transitions staff may make; clients may only confirm (closed) or reopen (in_progress) a resolved ticket.
var transitions = map[string][]string{
	"open":        {"in_progress", "resolved"},
	"in_progress": {"resolved", "open"},
	"resolved":    {"closed", "in_progress"},
	"closed":      {},
}

func canMove(from, to string) bool {
	for _, t := range transitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

type Ticket struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	Subject         string     `json:"subject"`
	Description     string     `json:"description"`
	Severity        Severity   `json:"severity"`
	Channel         string     `json:"channel"`
	Status          string     `json:"status"`
	Reporter        string     `json:"reporter"`
	Assignee        *string    `json:"assignee"`
	RespondDueAt    *time.Time `json:"respond_due_at"`
	SLADueAt        *time.Time `json:"sla_due_at"`
	FirstResponseAt *time.Time `json:"first_response_at"`
	ResolvedAt      *time.Time `json:"resolved_at"`
	SLABreached     bool       `json:"sla_breached"`
	Rating          *int       `json:"rating"`
	RatingComment   *string    `json:"rating_comment"`
	Version         int        `json:"version"`
	CreatedAt       time.Time  `json:"created_at"`
}

const ticketCols = `id, tenant_id, subject, description, severity, channel, status, reporter, assignee,
 respond_due_at, sla_due_at, first_response_at, resolved_at,
 (CASE WHEN resolved_at IS NULL THEN now() > sla_due_at ELSE resolved_at > sla_due_at END) AS sla_breached,
 rating, rating_comment, version, created_at`

func scanTicket(row pgx.Row) (*Ticket, error) {
	var t Ticket
	err := row.Scan(&t.ID, &t.TenantID, &t.Subject, &t.Description, &t.Severity, &t.Channel, &t.Status, &t.Reporter, &t.Assignee,
		&t.RespondDueAt, &t.SLADueAt, &t.FirstResponseAt, &t.ResolvedAt, &t.SLABreached, &t.Rating, &t.RatingComment, &t.Version, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, platform.Err(http.StatusNotFound, "Not found", "ticket not found")
	}
	return &t, err
}

func etag(v int) string { return fmt.Sprintf(`W/"%d"`, v) }

// Register mounts the module's routes.
func Register(a *platform.App) {
	a.Route("GET /v1/tickets", nil, list)
	a.Route("POST /v1/tickets", nil, platform.Idempotent(create))
	a.Route("GET /v1/tickets/{id}", nil, get)
	a.Route("PATCH /v1/tickets/{id}", nil, patch)
	a.Route("GET /v1/tickets/{id}/comments", nil, listComments)
	a.Route("POST /v1/tickets/{id}/comments", nil, platform.Idempotent(addComment))
	a.Route("POST /v1/tickets/{id}/rating", []string{"client_user", "client_admin"}, rate)
	a.Route("GET /v1/reports/support", []string{"ceo", "manager", "support"}, report)
}

type newTicket struct {
	TenantID    string   `json:"tenant_id"`
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	Severity    Severity `json:"severity"`
	Channel     string   `json:"channel"`
}

func create(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	if s.Claims.Scope() == "other" {
		return nil, platform.Err(http.StatusForbidden, "Forbidden", "role cannot open tickets")
	}
	var in newTicket
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	in.Subject = strings.TrimSpace(in.Subject)
	switch {
	case in.Subject == "" || len(in.Subject) > 200:
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid ticket", "subject is required (max 200 chars)")
	case !in.Severity.Valid():
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid ticket", "severity must be P1..P4")
	case !channels[in.Channel]:
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid ticket", "channel must be app, portal, email or phone")
	}
	tenant := s.Claims.TenantID
	if s.Claims.Scope() == "staff" {
		tenant = in.TenantID
	}
	if tenant == "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid ticket", "tenant_id is required")
	}
	row := s.Tx.QueryRow(r.Context(), `
		INSERT INTO support.tickets (tenant_id, subject, description, severity, channel, reporter, respond_due_at, sla_due_at)
		SELECT $1, $2, $3, p.severity, $5, $6, now() + p.respond_minutes * interval '1 minute', now() + p.resolve_minutes * interval '1 minute'
		FROM support.sla_policies p WHERE p.severity = $4
		RETURNING `+ticketCols, tenant, in.Subject, in.Description, in.Severity, in.Channel, s.Claims.Sub)
	t, err := scanTicket(row)
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "row-level security") {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid ticket", "unknown tenant")
		}
		return nil, err
	}
	ctx := r.Context()
	if err := s.Emit(ctx, "ticket.created", map[string]any{"id": t.ID, "tenant_id": t.TenantID, "severity": t.Severity, "sla_due_at": t.SLADueAt}); err != nil {
		return nil, err
	}
	if t.Severity == P1 {
		// P1 pages the on-call engineer and the Manager.
		if err := s.Emit(ctx, "ticket.p1.page", map[string]any{"id": t.ID, "tenant_id": t.TenantID, "subject": t.Subject}); err != nil {
			return nil, err
		}
	}
	if err := s.Audit(ctx, "create", "ticket", t.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: t, Header: map[string]string{"ETag": etag(t.Version), "Location": "/v1/tickets/" + t.ID}}, nil
}

func parseLimit(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if n < 1 || n > 100 {
		return 25
	}
	return n
}

func list(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	q := r.URL.Query()
	limit := parseLimit(r)
	var curAt *time.Time
	var curID *string
	if c := q.Get("cursor"); c != "" {
		b, err := base64.RawURLEncoding.DecodeString(c)
		parts := strings.SplitN(string(b), "|", 2)
		if err != nil || len(parts) != 2 {
			return nil, platform.Err(http.StatusBadRequest, "Bad cursor", "")
		}
		at, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return nil, platform.Err(http.StatusBadRequest, "Bad cursor", "")
		}
		curAt, curID = &at, &parts[1]
	}
	var status, sev *string
	if v := q.Get("status"); v != "" {
		status = &v
	}
	if v := q.Get("severity"); v != "" {
		sev = &v
	}
	rows, err := s.Tx.Query(r.Context(), `SELECT `+ticketCols+` FROM support.tickets
		WHERE ($1::text IS NULL OR status = $1) AND ($2::text IS NULL OR severity = $2)
		  AND ($3::timestamptz IS NULL OR (created_at, id) < ($3, $4::uuid))
		ORDER BY created_at DESC, id DESC LIMIT $5`, status, sev, curAt, curID, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var next *string
	if len(out) > limit {
		out = out[:limit]
		last := out[limit-1]
		c := base64.RawURLEncoding.EncodeToString([]byte(last.CreatedAt.Format(time.RFC3339Nano) + "|" + last.ID))
		next = &c
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out, "next_cursor": next}}, nil
}

func load(ctx context.Context, s *platform.Scope, id string, lock bool) (*Ticket, error) {
	sql := `SELECT ` + ticketCols + ` FROM support.tickets WHERE id = $1`
	if lock {
		sql += ` FOR UPDATE`
	}
	t, err := scanTicket(s.Tx.QueryRow(ctx, sql, id))
	if err != nil && strings.Contains(err.Error(), "invalid input syntax") {
		return nil, platform.Err(http.StatusNotFound, "Not found", "ticket not found")
	}
	return t, err
}

func get(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	t, err := load(r.Context(), s, r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: t, Header: map[string]string{"ETag": etag(t.Version)}}, nil
}

type patchTicket struct {
	Status   *string `json:"status"`
	Assignee *string `json:"assignee"`
}

func patch(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in patchTicket
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	ctx := r.Context()
	t, err := load(ctx, s, r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if m := r.Header.Get("If-Match"); m != "" && m != etag(t.Version) {
		return nil, platform.Err(http.StatusPreconditionFailed, "Precondition failed", "ticket was modified; reload and retry")
	}
	staff := s.Claims.Scope() == "staff"
	if !staff && (in.Assignee != nil || s.Claims.Scope() != "client") {
		return nil, platform.Err(http.StatusForbidden, "Forbidden", "clients may only confirm or reopen a resolved ticket")
	}
	status := t.Status
	if in.Status != nil {
		to := *in.Status
		if !canMove(t.Status, to) {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid transition", t.Status+" -> "+to+" is not allowed")
		}
		if !staff && !(t.Status == "resolved" && (to == "closed" || to == "in_progress")) {
			return nil, platform.Err(http.StatusForbidden, "Forbidden", "clients may only confirm or reopen a resolved ticket")
		}
		status = to
	}
	assignee := t.Assignee
	if in.Assignee != nil {
		assignee = in.Assignee
	}
	row := s.Tx.QueryRow(ctx, `UPDATE support.tickets SET status=$2, assignee=$3, version=version+1,
		resolved_at = CASE WHEN $2 = 'resolved' THEN now() WHEN $2 IN ('open','in_progress') THEN NULL ELSE resolved_at END,
		first_response_at = COALESCE(first_response_at, CASE WHEN $4 THEN now() END)
		WHERE id=$1 RETURNING `+ticketCols, t.ID, status, assignee, staff)
	nt, err := scanTicket(row)
	if err != nil {
		return nil, err
	}
	if status != t.Status {
		if err := s.Emit(ctx, "ticket.status_changed", map[string]any{"id": nt.ID, "tenant_id": nt.TenantID, "subject": nt.Subject, "from": t.Status, "to": status, "actor": s.Claims.Sub, "by": s.Claims.Scope()}); err != nil {
			return nil, err
		}
	}
	if err := s.Audit(ctx, "update", "ticket", nt.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: nt, Header: map[string]string{"ETag": etag(nt.Version)}}, nil
}

type Comment struct {
	ID        string    `json:"id"`
	TicketID  string    `json:"ticket_id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	Internal  bool      `json:"internal"`
	CreatedAt time.Time `json:"created_at"`
}

func listComments(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	t, err := load(r.Context(), s, r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	rows, err := s.Tx.Query(r.Context(), `SELECT id, ticket_id, author, body, internal, created_at FROM support.ticket_comments WHERE ticket_id=$1 ORDER BY created_at, id`, t.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Comment{}
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.TicketID, &c.Author, &c.Body, &c.Internal, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func addComment(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Body     string `json:"body"`
		Internal bool   `json:"internal"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	in.Body = strings.TrimSpace(in.Body)
	if in.Body == "" || len(in.Body) > 10000 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid comment", "body is required (max 10000 chars)")
	}
	staff := s.Claims.Scope() == "staff"
	if in.Internal && !staff {
		return nil, platform.Err(http.StatusForbidden, "Forbidden", "only staff can write internal notes")
	}
	ctx := r.Context()
	t, err := load(ctx, s, r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	var c Comment
	err = s.Tx.QueryRow(ctx, `INSERT INTO support.ticket_comments (ticket_id, tenant_id, author, body, internal) VALUES ($1,$2,$3,$4,$5)
		RETURNING id, ticket_id, author, body, internal, created_at`, t.ID, t.TenantID, s.Claims.Sub, in.Body, in.Internal).
		Scan(&c.ID, &c.TicketID, &c.Author, &c.Body, &c.Internal, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	if staff && !in.Internal {
		if _, err := s.Tx.Exec(ctx, `UPDATE support.tickets SET first_response_at = COALESCE(first_response_at, now()), version=version+1 WHERE id=$1`, t.ID); err != nil {
			return nil, err
		}
	}
	if !in.Internal {
		if err := s.Emit(ctx, "ticket.commented", map[string]any{"id": t.ID, "tenant_id": t.TenantID, "subject": t.Subject, "comment_id": c.ID, "actor": s.Claims.Sub, "by": s.Claims.Scope()}); err != nil {
			return nil, err
		}
	}
	if err := s.Audit(ctx, "create", "ticket_comment", c.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: c}, nil
}

func rate(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.Rating < 1 || in.Rating > 5 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid rating", "rating must be 1..5")
	}
	ctx := r.Context()
	t, err := load(ctx, s, r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if t.Status != "resolved" && t.Status != "closed" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Not ready", "only resolved or closed tickets can be rated")
	}
	row := s.Tx.QueryRow(ctx, `UPDATE support.tickets SET rating=$2, rating_comment=$3, version=version+1 WHERE id=$1 RETURNING `+ticketCols, t.ID, in.Rating, in.Comment)
	nt, err := scanTicket(row)
	if err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "rate", "ticket", nt.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: nt}, nil
}

func report(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var open, breached, p1Open, rated int
	var avg *float64
	err := s.Tx.QueryRow(r.Context(), `SELECT
		count(*) FILTER (WHERE status IN ('open','in_progress')),
		count(*) FILTER (WHERE CASE WHEN resolved_at IS NULL THEN now() > sla_due_at ELSE resolved_at > sla_due_at END),
		count(*) FILTER (WHERE severity='P1' AND status IN ('open','in_progress')),
		count(rating), avg(rating)::float8
		FROM support.tickets`).Scan(&open, &breached, &p1Open, &rated, &avg)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{
		"open": open, "sla_breached": breached, "p1_open": p1Open, "rated": rated, "avg_rating": avg,
	}}, nil
}

// TicketTenant returns the tenant of a ticket the caller is allowed to see (other modules use it to
// authorise attachments without reading the support schema themselves).
func TicketTenant(ctx context.Context, s *platform.Scope, id string) (string, error) {
	var tenant string
	err := s.Tx.QueryRow(ctx, `SELECT tenant_id FROM support.tickets WHERE id::text = $1`, id).Scan(&tenant)
	if isMissing(err) {
		return "", platform.Err(http.StatusNotFound, "Not found", "ticket not found")
	}
	return tenant, err
}

func isMissing(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || (err != nil && strings.Contains(err.Error(), "invalid input syntax"))
}
