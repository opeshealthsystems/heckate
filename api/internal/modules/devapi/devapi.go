// Package devapi is the API-as-a-product context: plans, tenant-scoped read-only API keys and usage.
package devapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

var planAdmin = []string{"ceo", "manager"}

// keyViewers see API keys and usage: management and support for any client, and the client's own people for their own.
var keyViewers = []string{"ceo", "manager", "support", "client_admin", "client_user"}

type Plan struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	RatePerMin   int    `json:"rate_per_min"`
	MonthlyQuota int64  `json:"monthly_quota"`
	PriceXAF     int64  `json:"price_xaf"`
	Active       bool   `json:"active"`
}

type Key struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	Plan       string     `json:"plan"`
	CreatedBy  string     `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

const keyCols = `id::text, tenant_id::text, name, prefix, scopes, plan, created_by, created_at, expires_at, revoked_at, last_used_at`

func scanKey(row pgx.Row) (*Key, error) {
	var k Key
	err := row.Scan(&k.ID, &k.TenantID, &k.Name, &k.Prefix, &k.Scopes, &k.Plan, &k.CreatedBy, &k.CreatedAt, &k.ExpiresAt, &k.RevokedAt, &k.LastUsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, platform.Err(http.StatusNotFound, "Not found", "api key not found")
	}
	return &k, err
}

func Register(a *platform.App) {
	a.Route("GET /v1/api-plans", keyViewers, listPlans)
	a.Route("PUT /v1/api-plans/{code}", planAdmin, putPlan)
	a.Route("GET /v1/api-keys", keyViewers, listKeys)
	a.Route("POST /v1/api-keys", []string{"client_admin"}, platform.Idempotent(createKey))
	a.Route("DELETE /v1/api-keys/{id}", []string{"client_admin", "ceo", "manager"}, revokeKey)
	a.Route("PATCH /v1/api-keys/{id}", planAdmin, setKeyPlan)
	a.Route("GET /v1/api-keys/{id}/usage", keyViewers, usage)
}

func listPlans(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT code, name, rate_per_min, monthly_quota, price_xaf, active FROM devapi.plans ORDER BY price_xaf, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Plan{}
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.Code, &p.Name, &p.RatePerMin, &p.MonthlyQuota, &p.PriceXAF, &p.Active); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func putPlan(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in Plan
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	in.Code = r.PathValue("code")
	if in.Name == "" || in.RatePerMin <= 0 || in.MonthlyQuota <= 0 || in.PriceXAF < 0 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid plan", "name, rate_per_min and monthly_quota are required and positive")
	}
	_, err := s.Tx.Exec(r.Context(), `INSERT INTO devapi.plans (code, name, rate_per_min, monthly_quota, price_xaf, active) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name, rate_per_min=EXCLUDED.rate_per_min, monthly_quota=EXCLUDED.monthly_quota,
		price_xaf=EXCLUDED.price_xaf, active=EXCLUDED.active`, in.Code, in.Name, in.RatePerMin, in.MonthlyQuota, in.PriceXAF, in.Active)
	if err != nil {
		if strings.Contains(err.Error(), "plans_code_check") {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid plan", "code must be lowercase letters, digits, - or _")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "put", "api_plan", in.Code); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: in}, nil
}

func listKeys(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	tc, ta := s.TenantFilter("tenant_id", 1)
	rows, err := s.Tx.Query(r.Context(), `SELECT `+keyCols+` FROM devapi.keys WHERE true`+tc+` ORDER BY created_at DESC LIMIT 200`, ta...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Key{}
	for rows.Next() {
		k, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func validScope(sc string) bool {
	seg, ok := strings.CutPrefix(sc, "read:")
	if !ok || seg == "" || len(seg) > 40 {
		return false
	}
	if seg == "*" {
		return true
	}
	for _, c := range seg {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return !map[string]bool{"api-keys": true, "api-plans": true, "devices": true, "stream": true}[seg]
}

func createKey(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Name          string   `json:"name"`
		Scopes        []string `json:"scopes"`
		Plan          string   `json:"plan"`
		ExpiresInDays int      `json:"expires_in_days"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Plan == "" {
		in.Plan = "free"
	}
	if in.Name == "" || len(in.Name) > 80 || len(in.Scopes) == 0 || len(in.Scopes) > 20 || in.ExpiresInDays < 0 || in.ExpiresInDays > 3650 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid key", "name and at least one scope are required")
	}
	for _, sc := range in.Scopes {
		if !validScope(sc) {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid scope", "scopes look like read:tickets or read:*; "+sc+" is not allowed")
		}
	}
	// Clients may only pick plans that cost nothing; paid plans are assigned by staff after a contract.
	var price int64
	var active bool
	if err := s.Tx.QueryRow(r.Context(), `SELECT price_xaf, active FROM devapi.plans WHERE code=$1`, in.Plan).Scan(&price, &active); err != nil || !active {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid plan", "unknown or inactive plan")
	}
	if price > 0 {
		return nil, platform.Err(http.StatusForbidden, "Plan not available", "paid plans are assigned by HEKATE staff")
	}
	var exp any
	if in.ExpiresInDays > 0 {
		exp = time.Now().AddDate(0, 0, in.ExpiresInDays)
	}
	secret, prefix, hash := platform.NewAPIKey()
	k, err := scanKey(s.Tx.QueryRow(r.Context(), `INSERT INTO devapi.keys (tenant_id, name, prefix, hash, scopes, plan, created_by, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+keyCols, s.Claims.TenantID, in.Name, prefix, hash, in.Scopes, in.Plan, s.Claims.Sub, exp))
	if err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "create", "api_key", k.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: map[string]any{"key": k, "secret": secret}}, nil
}

func revokeKey(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	tag, err := s.Tx.Exec(r.Context(), `UPDATE devapi.keys SET revoked_at = now() WHERE id=$1 AND revoked_at IS NULL`, r.PathValue("id"))
	if err != nil {
		if strings.Contains(err.Error(), "invalid input syntax") {
			return nil, platform.Err(http.StatusNotFound, "Not found", "api key not found")
		}
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, platform.Err(http.StatusNotFound, "Not found", "api key not found or already revoked")
	}
	if err := s.Audit(r.Context(), "revoke", "api_key", r.PathValue("id")); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusNoContent}, nil
}

func setKeyPlan(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Plan string `json:"plan"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	k, err := scanKey(s.Tx.QueryRow(r.Context(), `UPDATE devapi.keys SET plan=$2 WHERE id=$1 RETURNING `+keyCols, r.PathValue("id"), in.Plan))
	if err != nil {
		if strings.Contains(err.Error(), "violates foreign key") {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid plan", "unknown plan")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "set_plan", "api_key", k.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: k}, nil
}

func usage(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT to_char(month,'YYYY-MM'), calls FROM devapi.usage WHERE key_id=$1 ORDER BY month DESC LIMIT 24`, r.PathValue("id"))
	if err != nil {
		if strings.Contains(err.Error(), "invalid input syntax") {
			return nil, platform.Err(http.StatusNotFound, "Not found", "api key not found")
		}
		return nil, err
	}
	defer rows.Close()
	type row struct {
		Month string `json:"month"`
		Calls int64  `json:"calls"`
	}
	out := []row{}
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.Month, &x.Calls); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}
