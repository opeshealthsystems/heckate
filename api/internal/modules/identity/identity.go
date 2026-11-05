// Package identity handles account requests: people ask for access from the sign-in screen,
// a manager approves (creating the Keycloak account) or rejects.
package identity

import (
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

var approverRoles = []string{"ceo", "manager"}

// Roles an approver may give. ceo is left out so a manager cannot create a superior account.
var grantable = []string{"manager", "finance", "sales", "support", "engineer", "hr", "employee", "logistics", "procurement"}

type Request struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Phone     *string    `json:"phone"`
	Company   *string    `json:"company"`
	Reason    string     `json:"reason"`
	Status    string     `json:"status"`
	Role      *string    `json:"role"`
	DecidedBy *string    `json:"decided_by"`
	DecidedAt *time.Time `json:"decided_at"`
	Note      *string    `json:"note"`
	CreatedAt time.Time  `json:"created_at"`
}

func Register(a *platform.App, kc *Keycloak) {
	a.Public("POST /v1/public/account-requests", submit)
	a.Route("GET /v1/account-requests", approverRoles, list)
	a.Route("POST /v1/account-requests/{id}/approve", approverRoles, func(r *http.Request, s *platform.Scope) (*platform.Result, error) { return approve(r, s, kc) })
	a.Route("POST /v1/account-requests/{id}/reject", approverRoles, reject)
}

func submit(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Phone   string `json:"phone"`
		Company string `json:"company"`
		Reason  string `json:"reason"`
		Website string `json:"website"` // honeypot
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	ok := &platform.Result{Status: http.StatusAccepted, Body: map[string]string{"status": "received"}}
	if in.Website != "" {
		return ok, nil
	}
	in.Name, in.Email, in.Reason = strings.TrimSpace(in.Name), strings.TrimSpace(in.Email), strings.TrimSpace(in.Reason)
	addr, err := mail.ParseAddress(in.Email)
	if err != nil || addr.Address != in.Email || in.Name == "" || len(in.Name) > 200 || len(in.Email) > 254 ||
		len(in.Phone) > 40 || len(in.Company) > 200 || len(in.Reason) > 1000 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid request", "name and a valid email are required")
	}
	// The same answer whether or not a request was already pending, so the form cannot be used to probe addresses.
	if _, err := s.Tx.Exec(r.Context(), `SELECT platform.submit_account_request($1,$2,$3,$4,$5)`,
		in.Name, in.Email, strings.TrimSpace(in.Phone), strings.TrimSpace(in.Company), in.Reason); err != nil {
		return nil, err
	}
	if err := s.Emit(r.Context(), "account_request.created", map[string]any{"email": strings.ToLower(in.Email)}); err != nil {
		return nil, err
	}
	return ok, nil
}

const cols = `id::text, name, email, phone, company, reason, status, role, decided_by, decided_at, note, created_at`

func scan(row pgx.Row) (*Request, error) {
	var q Request
	err := row.Scan(&q.ID, &q.Name, &q.Email, &q.Phone, &q.Company, &q.Reason, &q.Status, &q.Role, &q.DecidedBy, &q.DecidedAt, &q.Note, &q.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, platform.Err(http.StatusNotFound, "Not found", "request not found or already decided")
	}
	return &q, err
}

func list(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	rows, err := s.Tx.Query(r.Context(), `SELECT `+cols+` FROM platform.account_requests WHERE ($1='all' OR status=$1) ORDER BY created_at DESC LIMIT 200`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Request{}
	for rows.Next() {
		q, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out, "roles": grantable}}, rows.Err()
}

func pending(r *http.Request, s *platform.Scope) (*Request, error) {
	return scan(s.Tx.QueryRow(r.Context(), `SELECT `+cols+` FROM platform.account_requests WHERE id::text=$1 AND status='pending' FOR UPDATE`, r.PathValue("id")))
}

func approve(r *http.Request, s *platform.Scope, kc *Keycloak) (*platform.Result, error) {
	var in struct {
		Role string `json:"role"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if !slices.Contains(grantable, in.Role) {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid role", "choose one of: "+strings.Join(grantable, ", "))
	}
	if !kc.Enabled() {
		return nil, platform.Err(http.StatusServiceUnavailable, "Account creation is not configured", "set HEKATE_KEYCLOAK_ADMIN_URL, _CLIENT_ID and _CLIENT_SECRET")
	}
	q, err := pending(r, s)
	if err != nil {
		return nil, err
	}
	first, last, _ := strings.Cut(q.Name, " ")
	// The new account joins the approver's own organisation.
	if _, err := kc.CreateUser(r.Context(), q.Email, first, strings.TrimSpace(last), in.Role, s.Claims.TenantID); err != nil {
		if errors.Is(err, ErrUserExists) {
			return nil, platform.Err(http.StatusConflict, "Account exists", "an account with this email already exists")
		}
		slog.Error("keycloak create user", "err", err, "request", q.ID)
		return nil, platform.Err(http.StatusBadGateway, "Could not create the account", "the identity server refused the request; try again or check its logs")
	}
	q, err = scan(s.Tx.QueryRow(r.Context(), `UPDATE platform.account_requests SET status='approved', role=$2, decided_by=$3, decided_at=now() WHERE id::text=$1 RETURNING `+cols,
		q.ID, in.Role, s.Claims.Sub))
	if err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "approve", "account_request", q.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: q}, nil
}

func reject(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Note string `json:"note"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if _, err := pending(r, s); err != nil {
		return nil, err
	}
	q, err := scan(s.Tx.QueryRow(r.Context(), `UPDATE platform.account_requests SET status='rejected', note=NULLIF($2,''), decided_by=$3, decided_at=now() WHERE id::text=$1 RETURNING `+cols,
		r.PathValue("id"), strings.TrimSpace(in.Note), s.Claims.Sub))
	if err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "reject", "account_request", q.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: q}, nil
}
