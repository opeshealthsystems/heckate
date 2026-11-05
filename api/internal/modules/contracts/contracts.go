// Package contracts is the Contracts context: multi-year service agreements (for example maintenance for a bank
// or a hospital over five years) with their clauses, the response times they promise, what they cover, and
// their renewals.
package contracts

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

var (
	writers = []string{"ceo", "manager", "sales"}
	readers = []string{"ceo", "manager", "sales", "finance", "support", "engineer"}
	// clients may read their own company's contracts once they are signed off (draft ones stay internal).
	everyone = platform.Join(readers, platform.ClientAll)
)

type module struct{ docs docs.Options }

func Register(a *platform.App, o docs.Options) {
	m := &module{docs: o}
	a.Route("POST /v1/contracts", writers, platform.Idempotent(create))
	a.Route("GET /v1/contracts", everyone, list)
	a.Route("GET /v1/contracts/expiring", readers, expiring)
	a.Route("GET /v1/contracts/{id}", everyone, get)
	a.Route("GET /v1/contracts/{id}/pdf", everyone, m.schedulePDF)
	a.Route("PATCH /v1/contracts/{id}", writers, update)
	a.Route("PUT /v1/contracts/{id}/clauses", writers, setClauses)
	a.Route("PUT /v1/contracts/{id}/sla", writers, setSLA)
	a.Route("POST /v1/contracts/{id}/links", writers, platform.Idempotent(addLink))
	a.Route("DELETE /v1/contracts/{id}/links/{kind}/{ref}", writers, removeLink)
	a.Route("POST /v1/contracts/{id}/activate", writers, platform.Idempotent(activate))
	a.Route("POST /v1/contracts/{id}/suspend", writers, platform.Idempotent(setState("suspended", "active", "suspend")))
	a.Route("POST /v1/contracts/{id}/resume", writers, platform.Idempotent(setState("active", "suspended", "resume")))
	a.Route("POST /v1/contracts/{id}/renew", writers, platform.Idempotent(renew))
	a.Route("POST /v1/contracts/{id}/terminate", writers, platform.Idempotent(terminate))
	a.Route("POST /v1/contracts/renewals/run", []string{"ceo", "manager"}, platform.Idempotent(runRenewals))
}

func nf() error { return platform.Err(http.StatusNotFound, "Not found", "contract not found") }
func bad(m string) error {
	return platform.Err(http.StatusUnprocessableEntity, "Invalid contract", m)
}
func isNF(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || (err != nil && strings.Contains(err.Error(), "invalid input syntax"))
}

type Contract struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	Client            string    `json:"client"`
	Number            string    `json:"number"`
	Title             string    `json:"title"`
	Kind              string    `json:"kind"`
	Status            string    `json:"status"`
	StartDate         time.Time `json:"start_date"`
	EndDate           time.Time `json:"end_date"`
	TermMonths        int       `json:"term_months"`
	DaysLeft          int       `json:"days_left"`
	Value             int64     `json:"value"`
	Billing           string    `json:"billing"`
	PeriodAmount      int64     `json:"period_amount"`
	AutoRenew         bool      `json:"auto_renew"`
	RenewalTermMonths int       `json:"renewal_term_months"`
	RenewalNoticeDays int       `json:"renewal_notice_days"`
	Notes             string    `json:"notes"`
	EndedReason       *string   `json:"ended_reason"`
	CreatedBy         string    `json:"created_by"`
	CreatedAt         time.Time `json:"created_at"`
	Clauses           *[]Clause `json:"clauses,omitempty"`
	SLA               *[]SLA    `json:"sla,omitempty"`
	Links             *[]Link   `json:"links,omitempty"`
	History           *[]Event  `json:"history,omitempty"`
}

type Clause struct {
	Position int    `json:"position"`
	Title    string `json:"title"`
	Body     string `json:"body"`
}
type SLA struct {
	Severity       string `json:"severity"`
	RespondMinutes int    `json:"respond_minutes"`
	ResolveMinutes int    `json:"resolve_minutes"`
}
type Link struct {
	Kind     string    `json:"kind"`
	RefID    string    `json:"ref_id"`
	Label    string    `json:"label"`
	LinkedBy string    `json:"linked_by"`
	LinkedAt time.Time `json:"linked_at"`
}
type Event struct {
	Action string         `json:"action"`
	Actor  string         `json:"actor"`
	Detail map[string]any `json:"detail"`
	At     time.Time      `json:"at"`
}

const cCols = `c.id, c.tenant_id, COALESCE(t.name,''), c.number, c.title, c.kind, c.status, c.start_date, c.end_date,
	((extract(year from age(c.end_date + 1, c.start_date)) * 12) + extract(month from age(c.end_date + 1, c.start_date)))::int,
	(c.end_date - current_date)::int, c.value, c.billing, c.period_amount, c.auto_renew, c.renewal_term_months, c.renewal_notice_days,
	c.notes, c.ended_reason, c.created_by, c.created_at`
const cFrom = ` FROM contracts.contracts c LEFT JOIN platform.tenants t ON t.id = c.tenant_id`

func scan(row pgx.Row) (*Contract, error) {
	var c Contract
	err := row.Scan(&c.ID, &c.TenantID, &c.Client, &c.Number, &c.Title, &c.Kind, &c.Status, &c.StartDate, &c.EndDate, &c.TermMonths, &c.DaysLeft,
		&c.Value, &c.Billing, &c.PeriodAmount, &c.AutoRenew, &c.RenewalTermMonths, &c.RenewalNoticeDays, &c.Notes, &c.EndedReason, &c.CreatedBy, &c.CreatedAt)
	if isNF(err) {
		return nil, nf()
	}
	return &c, err
}

func record(ctx context.Context, s *platform.Scope, id, tenant, action string, detail map[string]any) error {
	if detail == nil {
		detail = map[string]any{}
	}
	_, err := s.Tx.Exec(ctx, `INSERT INTO contracts.events (contract_id, tenant_id, action, actor, detail) VALUES ($1,$2,$3,$4,$5)`, id, tenant, action, s.Claims.Sub, detail)
	return err
}

func parseDate(v, field string) (time.Time, error) {
	d, err := time.Parse("2006-01-02", v)
	if err != nil {
		return d, bad(field + " must be YYYY-MM-DD")
	}
	return d, nil
}

var kinds = map[string]bool{"maintenance": true, "support": true, "supply": true, "service": true, "licence": true}
var billings = map[string]bool{"monthly": true, "quarterly": true, "annual": true, "one_off": true}

func create(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		TenantID          string `json:"tenant_id"`
		Title             string `json:"title"`
		Kind              string `json:"kind"`
		StartDate         string `json:"start_date"`
		EndDate           string `json:"end_date"`
		Value             int64  `json:"value"`
		Billing           string `json:"billing"`
		PeriodAmount      int64  `json:"period_amount"`
		AutoRenew         bool   `json:"auto_renew"`
		RenewalTermMonths int    `json:"renewal_term_months"`
		RenewalNoticeDays *int   `json:"renewal_notice_days"`
		Notes             string `json:"notes"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.TenantID == "" || strings.TrimSpace(in.Title) == "" {
		return nil, bad("tenant_id and title are required")
	}
	if in.Kind == "" {
		in.Kind = "maintenance"
	}
	if in.Billing == "" {
		in.Billing = "monthly"
	}
	if !kinds[in.Kind] || !billings[in.Billing] {
		return nil, bad("unknown kind or billing")
	}
	start, err := parseDate(in.StartDate, "start_date")
	if err != nil {
		return nil, err
	}
	end, err := parseDate(in.EndDate, "end_date")
	if err != nil {
		return nil, err
	}
	if end.Before(start) {
		return nil, bad("end_date cannot be before start_date")
	}
	if in.RenewalTermMonths == 0 {
		in.RenewalTermMonths = 12
	}
	notice := 60
	if in.RenewalNoticeDays != nil {
		notice = *in.RenewalNoticeDays
	}
	ctx := r.Context()
	var id string
	err = s.Tx.QueryRow(ctx, `INSERT INTO contracts.contracts (tenant_id, number, title, kind, start_date, end_date, value, billing, period_amount, auto_renew, renewal_term_months, renewal_notice_days, notes, created_by)
		VALUES ($1, 'CT-' || to_char(now(),'YYYY') || '-' || lpad(nextval('contracts.number_seq')::text, 4, '0'), $2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id::text`,
		in.TenantID, strings.TrimSpace(in.Title), in.Kind, start, end, in.Value, in.Billing, in.PeriodAmount, in.AutoRenew, in.RenewalTermMonths, notice, in.Notes, s.Claims.Sub).Scan(&id)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "foreign key"):
			return nil, bad("unknown tenant")
		case strings.Contains(err.Error(), "check constraint"):
			return nil, bad("check the amounts, the renewal term (1-120 months) and the notice period (0-365 days)")
		}
		return nil, err
	}
	if err := record(ctx, s, id, in.TenantID, "created", nil); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "create", "contract", id); err != nil {
		return nil, err
	}
	c, err := scan(s.Tx.QueryRow(ctx, `SELECT `+cCols+cFrom+` WHERE c.id::text=$1`, id))
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: c}, nil
}

func list(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	q := `SELECT ` + cCols + cFrom + ` WHERE true`
	var args []any
	if st := r.URL.Query().Get("status"); st != "" {
		args = append(args, st)
		q += fmt.Sprintf(` AND c.status = $%d`, len(args))
	}
	if t := r.URL.Query().Get("tenant_id"); t != "" && s.Claims.Scope() == "staff" {
		args = append(args, t)
		q += fmt.Sprintf(` AND c.tenant_id::text = $%d`, len(args))
	}
	rows, err := s.Tx.Query(r.Context(), q+` ORDER BY c.end_date, c.number LIMIT 500`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Contract{}
	for rows.Next() {
		c, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

// expiring lists active contracts that end within the window (default 90 days), soonest first.
func expiring(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	days := 90
	if v := r.URL.Query().Get("days"); v != "" {
		if _, err := fmt.Sscan(v, &days); err != nil || days < 1 || days > 730 {
			return nil, bad("days must be 1 to 730")
		}
	}
	rows, err := s.Tx.Query(r.Context(), `SELECT `+cCols+cFrom+` WHERE c.status = 'active' AND c.end_date <= current_date + $1::int ORDER BY c.end_date, c.number`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Contract{}
	for rows.Next() {
		c, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func load(ctx context.Context, s *platform.Scope, id string) (*Contract, error) {
	c, err := scan(s.Tx.QueryRow(ctx, `SELECT `+cCols+cFrom+` WHERE c.id::text=$1`, id))
	if err != nil {
		return nil, err
	}
	clauses, sla, links, hist := []Clause{}, []SLA{}, []Link{}, []Event{}
	rows, err := s.Tx.Query(ctx, `SELECT position, title, body FROM contracts.clauses WHERE contract_id=$1 ORDER BY position`, c.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x Clause
		if err := rows.Scan(&x.Position, &x.Title, &x.Body); err != nil {
			rows.Close()
			return nil, err
		}
		clauses = append(clauses, x)
	}
	rows.Close()
	rows, err = s.Tx.Query(ctx, `SELECT severity, respond_minutes, resolve_minutes FROM contracts.sla WHERE contract_id=$1 ORDER BY severity`, c.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x SLA
		if err := rows.Scan(&x.Severity, &x.RespondMinutes, &x.ResolveMinutes); err != nil {
			rows.Close()
			return nil, err
		}
		sla = append(sla, x)
	}
	rows.Close()
	c.Clauses, c.SLA, c.Links, c.History = &clauses, &sla, &links, &hist
	if s.Claims.Scope() != "staff" {
		return c, nil // clients see the agreement itself, not what is linked or the internal history
	}
	rows, err = s.Tx.Query(ctx, `SELECT kind, ref_id::text, label, linked_by, linked_at FROM contracts.links WHERE contract_id=$1 ORDER BY linked_at`, c.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x Link
		if err := rows.Scan(&x.Kind, &x.RefID, &x.Label, &x.LinkedBy, &x.LinkedAt); err != nil {
			rows.Close()
			return nil, err
		}
		links = append(links, x)
	}
	rows.Close()
	rows, err = s.Tx.Query(ctx, `SELECT action, actor, detail, at FROM contracts.events WHERE contract_id=$1 ORDER BY id`, c.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var x Event
		if err := rows.Scan(&x.Action, &x.Actor, &x.Detail, &x.At); err != nil {
			return nil, err
		}
		hist = append(hist, x)
	}
	return c, rows.Err()
}

func get(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	c, err := load(r.Context(), s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: c}, nil
}

// lock reads a contract for update and returns its current state.
func lock(ctx context.Context, s *platform.Scope, id string) (*Contract, error) {
	return scan(s.Tx.QueryRow(ctx, `SELECT `+cCols+cFrom+` WHERE c.id::text=$1 FOR UPDATE OF c`, id))
}

// update changes commercial terms. Dates, value and billing can change while the contract is a draft or active
// (each change is logged); an ended contract is closed for good.
func update(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Title             *string `json:"title"`
		Value             *int64  `json:"value"`
		Billing           *string `json:"billing"`
		PeriodAmount      *int64  `json:"period_amount"`
		AutoRenew         *bool   `json:"auto_renew"`
		RenewalTermMonths *int    `json:"renewal_term_months"`
		RenewalNoticeDays *int    `json:"renewal_notice_days"`
		Notes             *string `json:"notes"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	ctx := r.Context()
	c, err := lock(ctx, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if c.Status == "expired" || c.Status == "terminated" {
		return nil, platform.Err(http.StatusConflict, "Closed", "an ended contract cannot be changed")
	}
	if in.Billing != nil && !billings[*in.Billing] {
		return nil, bad("unknown billing")
	}
	_, err = s.Tx.Exec(ctx, `UPDATE contracts.contracts SET title=COALESCE($2,title), value=COALESCE($3,value), billing=COALESCE($4,billing), period_amount=COALESCE($5,period_amount),
		auto_renew=COALESCE($6,auto_renew), renewal_term_months=COALESCE($7,renewal_term_months), renewal_notice_days=COALESCE($8,renewal_notice_days), notes=COALESCE($9,notes) WHERE id=$1`,
		c.ID, in.Title, in.Value, in.Billing, in.PeriodAmount, in.AutoRenew, in.RenewalTermMonths, in.RenewalNoticeDays, in.Notes)
	if err != nil {
		if strings.Contains(err.Error(), "check constraint") {
			return nil, bad("check the amounts, the renewal term (1-120 months) and the notice period (0-365 days)")
		}
		return nil, err
	}
	if err := record(ctx, s, c.ID, c.TenantID, "updated", nil); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "update", "contract", c.ID); err != nil {
		return nil, err
	}
	out, err := load(ctx, s, c.ID)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}

func setClauses(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Clauses []Clause `json:"clauses"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if len(in.Clauses) > 100 {
		return nil, bad("at most 100 clauses")
	}
	ctx := r.Context()
	c, err := lock(ctx, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if c.Status != "draft" {
		return nil, platform.Err(http.StatusConflict, "Agreed text", "clauses can only change while the contract is a draft")
	}
	if _, err := s.Tx.Exec(ctx, `DELETE FROM contracts.clauses WHERE contract_id=$1`, c.ID); err != nil {
		return nil, err
	}
	for i, cl := range in.Clauses {
		if strings.TrimSpace(cl.Title) == "" || strings.TrimSpace(cl.Body) == "" {
			return nil, bad("every clause needs a title and text")
		}
		if _, err := s.Tx.Exec(ctx, `INSERT INTO contracts.clauses (contract_id, tenant_id, position, title, body) VALUES ($1,$2,$3,$4,$5)`, c.ID, c.TenantID, i+1, strings.TrimSpace(cl.Title), cl.Body); err != nil {
			return nil, err
		}
	}
	if err := record(ctx, s, c.ID, c.TenantID, "clauses_set", map[string]any{"count": len(in.Clauses)}); err != nil {
		return nil, err
	}
	out, err := load(ctx, s, c.ID)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}

// setSLA replaces the response and resolution times this contract promises. Tickets raised by the client use
// them in place of the standard targets while the contract is active.
func setSLA(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		SLA []SLA `json:"sla"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	ctx := r.Context()
	c, err := lock(ctx, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if c.Status == "expired" || c.Status == "terminated" {
		return nil, platform.Err(http.StatusConflict, "Closed", "an ended contract cannot be changed")
	}
	seen := map[string]bool{}
	for _, x := range in.SLA {
		if seen[x.Severity] {
			return nil, bad("one line per severity")
		}
		seen[x.Severity] = true
	}
	if _, err := s.Tx.Exec(ctx, `DELETE FROM contracts.sla WHERE contract_id=$1`, c.ID); err != nil {
		return nil, err
	}
	for _, x := range in.SLA {
		if _, err := s.Tx.Exec(ctx, `INSERT INTO contracts.sla (contract_id, tenant_id, severity, respond_minutes, resolve_minutes) VALUES ($1,$2,$3,$4,$5)`, c.ID, c.TenantID, x.Severity, x.RespondMinutes, x.ResolveMinutes); err != nil {
			if strings.Contains(err.Error(), "check constraint") {
				return nil, bad("severity must be P1 to P4, times must be above zero and resolution cannot be shorter than response")
			}
			return nil, err
		}
	}
	if err := record(ctx, s, c.ID, c.TenantID, "sla_set", map[string]any{"count": len(in.SLA)}); err != nil {
		return nil, err
	}
	out, err := load(ctx, s, c.ID)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}

// linkTargets maps a link kind to the table that must hold the id and carry the contract's client.
var linkTargets = map[string]string{
	"asset":   `SELECT tenant_id::text FROM inventory.serials WHERE id::text=$1`,
	"project": `SELECT tenant_id::text FROM projects.projects WHERE id::text=$1`,
	"invoice": `SELECT tenant_id::text FROM sales.invoices WHERE id::text=$1`,
	"ticket":  `SELECT tenant_id::text FROM support.tickets WHERE id::text=$1`,
}

func addLink(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Kind  string `json:"kind"`
		RefID string `json:"ref_id"`
		Label string `json:"label"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	q, ok := linkTargets[in.Kind]
	if !ok {
		return nil, bad("kind must be asset, project, invoice or ticket")
	}
	ctx := r.Context()
	c, err := lock(ctx, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	var tenant *string
	if err := s.Tx.QueryRow(ctx, q, in.RefID).Scan(&tenant); err != nil {
		if isNF(err) {
			return nil, bad("that " + in.Kind + " does not exist")
		}
		return nil, err
	}
	if tenant == nil || *tenant != c.TenantID {
		return nil, bad("that " + in.Kind + " belongs to a different client")
	}
	if _, err := s.Tx.Exec(ctx, `INSERT INTO contracts.links (contract_id, tenant_id, kind, ref_id, label, linked_by) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
		c.ID, c.TenantID, in.Kind, in.RefID, in.Label, s.Claims.Sub); err != nil {
		return nil, err
	}
	if in.Kind == "project" {
		if _, err := s.Tx.Exec(ctx, `UPDATE projects.projects SET contract_id=$1 WHERE id::text=$2`, c.ID, in.RefID); err != nil {
			return nil, err
		}
	}
	if err := record(ctx, s, c.ID, c.TenantID, "linked", map[string]any{"kind": in.Kind, "ref_id": in.RefID}); err != nil {
		return nil, err
	}
	out, err := load(ctx, s, c.ID)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: out}, nil
}

func removeLink(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	c, err := lock(ctx, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	tag, err := s.Tx.Exec(ctx, `DELETE FROM contracts.links WHERE contract_id=$1 AND kind=$2 AND ref_id::text=$3`, c.ID, r.PathValue("kind"), r.PathValue("ref"))
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, platform.Err(http.StatusNotFound, "Not found", "link not found")
	}
	if err := record(ctx, s, c.ID, c.TenantID, "unlinked", map[string]any{"kind": r.PathValue("kind"), "ref_id": r.PathValue("ref")}); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusNoContent}, nil
}

func activate(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	c, err := lock(ctx, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if c.Status != "draft" {
		return nil, platform.Err(http.StatusConflict, "Not a draft", "only a draft contract can be activated")
	}
	var n int
	if err := s.Tx.QueryRow(ctx, `SELECT count(*) FROM contracts.clauses WHERE contract_id=$1`, c.ID).Scan(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, bad("add the clauses before activating: a signed contract states what it covers")
	}
	if _, err := s.Tx.Exec(ctx, `UPDATE contracts.contracts SET status='active' WHERE id=$1`, c.ID); err != nil {
		return nil, err
	}
	if err := record(ctx, s, c.ID, c.TenantID, "activated", nil); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "activate", "contract", c.ID); err != nil {
		return nil, err
	}
	if err := s.Emit(ctx, "contract.activated", map[string]any{"id": c.ID, "tenant_id": c.TenantID, "number": c.Number, "actor": s.Claims.Sub}); err != nil {
		return nil, err
	}
	out, err := load(ctx, s, c.ID)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}

func setState(to, from, action string) platform.HandlerFunc {
	return func(r *http.Request, s *platform.Scope) (*platform.Result, error) {
		var in struct {
			Reason string `json:"reason"`
		}
		if err := platform.DecodeBody(r, &in); err != nil {
			return nil, err
		}
		if to == "suspended" && strings.TrimSpace(in.Reason) == "" {
			return nil, bad("a reason is required")
		}
		ctx := r.Context()
		c, err := lock(ctx, s, r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		if c.Status != from {
			return nil, platform.Err(http.StatusConflict, "Wrong state", fmt.Sprintf("only a %s contract can be %sd", from, action))
		}
		if _, err := s.Tx.Exec(ctx, `UPDATE contracts.contracts SET status=$2 WHERE id=$1`, c.ID, to); err != nil {
			return nil, err
		}
		if err := record(ctx, s, c.ID, c.TenantID, action, map[string]any{"reason": in.Reason}); err != nil {
			return nil, err
		}
		if err := s.Audit(ctx, action, "contract", c.ID); err != nil {
			return nil, err
		}
		out, err := load(ctx, s, c.ID)
		if err != nil {
			return nil, err
		}
		return &platform.Result{Status: http.StatusOK, Body: out}, nil
	}
}

func terminate(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Reason string `json:"reason"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Reason) == "" {
		return nil, bad("a reason is required")
	}
	ctx := r.Context()
	c, err := lock(ctx, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if c.Status != "active" && c.Status != "suspended" {
		return nil, platform.Err(http.StatusConflict, "Wrong state", "only an active or suspended contract can be terminated")
	}
	if _, err := s.Tx.Exec(ctx, `UPDATE contracts.contracts SET status='terminated', ended_reason=$2 WHERE id=$1`, c.ID, in.Reason); err != nil {
		return nil, err
	}
	if err := record(ctx, s, c.ID, c.TenantID, "terminated", map[string]any{"reason": in.Reason}); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "terminate", "contract", c.ID); err != nil {
		return nil, err
	}
	out, err := load(ctx, s, c.ID)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}

// renew extends the end date by whole months (default: the contract's renewal term) and optionally sets a new
// total value. It works on an active contract, or on one that has just expired.
func renew(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Months int    `json:"months"`
		Value  *int64 `json:"value"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	ctx := r.Context()
	c, err := lock(ctx, s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if c.Status != "active" && c.Status != "expired" {
		return nil, platform.Err(http.StatusConflict, "Wrong state", "only an active or expired contract can be renewed")
	}
	if in.Months == 0 {
		in.Months = c.RenewalTermMonths
	}
	if in.Months < 1 || in.Months > 120 {
		return nil, bad("months must be 1 to 120")
	}
	end := c.EndDate.AddDate(0, in.Months, 0)
	if _, err := s.Tx.Exec(ctx, `UPDATE contracts.contracts SET end_date=$2, status='active', value=COALESCE($3,value) WHERE id=$1`, c.ID, end, in.Value); err != nil {
		return nil, err
	}
	if err := record(ctx, s, c.ID, c.TenantID, "renewed", map[string]any{"months": in.Months, "from": c.EndDate.Format("2006-01-02"), "to": end.Format("2006-01-02")}); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "renew", "contract", c.ID); err != nil {
		return nil, err
	}
	out, err := load(ctx, s, c.ID)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}
