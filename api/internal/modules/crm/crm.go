// Package crm is the CRM bounded context: leads from the website and the sales pipeline.
package crm

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

var salesRoles = []string{"ceo", "manager", "sales"}

type Lead struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     *string   `json:"phone"`
	Company   *string   `json:"company"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
	Stage     string    `json:"stage"`
	Owner     *string   `json:"owner"`
	CreatedAt time.Time `json:"created_at"`
}

func Register(a *platform.App) {
	a.Public("POST /v1/public/enquiries", enquiry)
	a.Route("GET /v1/leads", salesRoles, list)
	a.Route("PATCH /v1/leads/{id}", salesRoles, update)
}

func enquiry(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Phone   string `json:"phone"`
		Company string `json:"company"`
		Message string `json:"message"`
		Website string `json:"website"` // honeypot: real users leave it empty
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.Website != "" { // bot: pretend success
		return &platform.Result{Status: http.StatusAccepted, Body: map[string]string{"status": "received"}}, nil
	}
	in.Name, in.Message = strings.TrimSpace(in.Name), strings.TrimSpace(in.Message)
	if _, err := mail.ParseAddress(in.Email); err != nil || in.Name == "" || len(in.Message) > 5000 || len(in.Name) > 200 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid enquiry", "name and a valid email are required")
	}
	var id string
	if err := s.Tx.QueryRow(r.Context(), `SELECT crm.submit_enquiry($1,$2,$3,$4,$5)`, in.Name, in.Email, in.Phone, in.Company, in.Message).Scan(&id); err != nil {
		return nil, err
	}
	if err := s.Emit(r.Context(), "lead.created", map[string]any{"id": id}); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusAccepted, Body: map[string]string{"status": "received"}}, nil
}

const cols = `id, name, email, phone, company, message, source, stage, owner, created_at`

func scan(row pgx.Row) (*Lead, error) {
	var l Lead
	err := row.Scan(&l.ID, &l.Name, &l.Email, &l.Phone, &l.Company, &l.Message, &l.Source, &l.Stage, &l.Owner, &l.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, platform.Err(http.StatusNotFound, "Not found", "lead not found")
	}
	return &l, err
}

func list(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+cols+` FROM crm.leads ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Lead{}
	for rows.Next() {
		l, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func update(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Stage *string `json:"stage"`
		Owner *string `json:"owner"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	l, err := scan(s.Tx.QueryRow(r.Context(), `UPDATE crm.leads SET stage=COALESCE($2,stage), owner=COALESCE($3,owner) WHERE id::text=$1 RETURNING `+cols,
		r.PathValue("id"), in.Stage, in.Owner))
	if err != nil {
		if strings.Contains(err.Error(), "leads_stage_check") {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid lead", "unknown stage")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "update", "lead", l.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: l}, nil
}
