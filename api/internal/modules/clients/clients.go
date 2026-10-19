// Package clients is the Clients bounded context: client accounts and contacts.
package clients

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

var staffWrite = []string{"ceo", "manager", "sales", "support"}

type Account struct {
	TenantID  string    `json:"tenant_id"`
	LegalName string    `json:"legal_name"`
	RCCM      *string   `json:"rccm"`
	Email     *string   `json:"email"`
	Phone     *string   `json:"phone"`
	KYCStatus string    `json:"kyc_status"`
	CreatedAt time.Time `json:"created_at"`
}

const cols = `tenant_id, legal_name, rccm, email, phone, kyc_status, created_at`

func scan(row pgx.Row) (*Account, error) {
	var a Account
	err := row.Scan(&a.TenantID, &a.LegalName, &a.RCCM, &a.Email, &a.Phone, &a.KYCStatus, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, platform.Err(http.StatusNotFound, "Not found", "client not found")
	}
	return &a, err
}

func Register(a *platform.App) {
	a.Route("GET /v1/me", nil, me)
	a.Route("GET /v1/clients", []string{"ceo", "manager", "sales", "support", "finance", "engineer"}, list)
	a.Route("POST /v1/clients", staffWrite, platform.Idempotent(create))
	a.Route("GET /v1/clients/{id}", nil, get)
	a.Route("PATCH /v1/clients/{id}", staffWrite, update)
}

func me(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	c := s.Claims
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{
		"sub": c.Sub, "email": c.Email, "tenant_id": c.TenantID, "roles": c.Roles, "scope": c.Scope(),
	}}, nil
}

func list(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+cols+` FROM clients.accounts ORDER BY legal_name LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Account{}
	for rows.Next() {
		a, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func create(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		LegalName string `json:"legal_name"`
		RCCM      string `json:"rccm"`
		Email     string `json:"email"`
		Phone     string `json:"phone"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	in.LegalName = strings.TrimSpace(in.LegalName)
	if in.LegalName == "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid client", "legal_name is required")
	}
	ctx := r.Context()
	var id string
	if err := s.Tx.QueryRow(ctx, `INSERT INTO platform.tenants (name) VALUES ($1) RETURNING id`, in.LegalName).Scan(&id); err != nil {
		return nil, err
	}
	a, err := scan(s.Tx.QueryRow(ctx, `INSERT INTO clients.accounts (tenant_id, legal_name, rccm, email, phone) VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,'')) RETURNING `+cols,
		id, in.LegalName, in.RCCM, in.Email, in.Phone))
	if err != nil {
		return nil, err
	}
	if err := s.Emit(ctx, "client.created", map[string]any{"tenant_id": id}); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "create", "client", id); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: a}, nil
}

func get(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	a, err := scan(s.Tx.QueryRow(r.Context(), `SELECT `+cols+` FROM clients.accounts WHERE tenant_id::text = $1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: a}, nil
}

func update(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		KYCStatus *string `json:"kyc_status"`
		Email     *string `json:"email"`
		Phone     *string `json:"phone"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.KYCStatus != nil {
		switch *in.KYCStatus {
		case "pending", "verified", "rejected":
		default:
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid client", "kyc_status must be pending, verified or rejected")
		}
		// KYC decisions are compliance-sensitive: managers only.
		if !s.Claims.HasAny("ceo", "manager") {
			return nil, platform.Err(http.StatusForbidden, "Forbidden", "only managers can change KYC status")
		}
	}
	a, err := scan(s.Tx.QueryRow(r.Context(), `UPDATE clients.accounts SET kyc_status=COALESCE($2,kyc_status), email=COALESCE($3,email), phone=COALESCE($4,phone)
		WHERE tenant_id::text=$1 RETURNING `+cols, r.PathValue("id"), in.KYCStatus, in.Email, in.Phone))
	if err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "update", "client", a.TenantID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: a}, nil
}
