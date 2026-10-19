// Package documents is the Documents & Trust context: the document register,
// reference numbers (HS-<CODE>-<YYYY>-<NNNNNN>) and public verification.
package documents

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

var (
	codeRe = regexp.MustCompile(`^[A-Z]+(-[A-Z0-9]+)*$`)
	refRe  = regexp.MustCompile(`^HS-([A-Z]+(?:-[A-Z0-9]+)*)-(\d{4})-(\d{6})(?:-([0-9A-Z]))?$`)
)

// IsCertificate reports whether a code carries a check character.
func IsCertificate(code string) bool {
	return strings.HasPrefix(code, "CRT") || strings.HasPrefix(code, "ASM")
}

// CheckChar is a position-weighted mod-36 checksum over the reference body.
func CheckChar(body string) byte {
	sum := 0
	for i, c := range body {
		if v := strings.IndexRune(alphabet, c); v >= 0 {
			sum += (i + 1) * (v + 1)
		}
	}
	return alphabet[sum%36]
}

// FormatRef builds the reference for a code, year and sequence number.
func FormatRef(code string, year, seq int) string {
	body := fmt.Sprintf("HS-%s-%d-%06d", code, year, seq)
	if IsCertificate(code) {
		return body + "-" + string(CheckChar(body))
	}
	return body
}

// ValidRef checks syntax and, for certificates, the check character.
func ValidRef(ref string) bool {
	m := refRe.FindStringSubmatch(ref)
	if m == nil {
		return false
	}
	if IsCertificate(m[1]) {
		if m[4] == "" {
			return false
		}
		body := strings.TrimSuffix(ref, "-"+m[4])
		return string(CheckChar(body)) == m[4]
	}
	return m[4] == ""
}

type Document struct {
	Ref      string    `json:"ref"`
	Code     string    `json:"code"`
	Title    string    `json:"title"`
	TenantID *string   `json:"tenant_id"`
	Status   string    `json:"status"`
	SHA256   *string   `json:"sha256"`
	IssuedBy string    `json:"issued_by"`
	IssuedAt time.Time `json:"issued_at"`
}

const cols = `ref, code, title, tenant_id, status, sha256, issued_by, issued_at`

func scan(row pgx.Row) (*Document, error) {
	var d Document
	err := row.Scan(&d.Ref, &d.Code, &d.Title, &d.TenantID, &d.Status, &d.SHA256, &d.IssuedBy, &d.IssuedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, platform.Err(http.StatusNotFound, "Not found", "document not found")
	}
	return &d, err
}

var issuers = []string{"ceo", "manager", "sales", "finance", "support", "engineer", "hr"}

func Register(a *platform.App) {
	a.Route("POST /v1/documents", issuers, platform.Idempotent(issue))
	a.Route("GET /v1/documents/{ref}", nil, get)
	a.Route("POST /v1/documents/{ref}/revoke", []string{"ceo", "manager"}, revoke)
	a.Public("GET /v1/public/verify/{ref}", verify)
}

// Issue allocates the next reference for code and registers the document. Other modules call it
// inside their own transaction so a document and its register entry commit together.
func Issue(ctx context.Context, s *platform.Scope, code, title, tenantID, sha string) (*Document, error) {
	year := time.Now().UTC().Year()
	var seq int
	if err := s.Tx.QueryRow(ctx, `INSERT INTO documents.counters (code, year, last) VALUES ($1,$2,1)
		ON CONFLICT (code, year) DO UPDATE SET last = documents.counters.last + 1 RETURNING last`, code, year).Scan(&seq); err != nil {
		return nil, err
	}
	ref := FormatRef(code, year, seq)
	d, err := scan(s.Tx.QueryRow(ctx, `INSERT INTO documents.register (ref, code, title, tenant_id, sha256, issued_by)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,''),$6) RETURNING `+cols, ref, code, title, tenantID, sha, s.Claims.Sub))
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") {
			return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid document", "unknown tenant")
		}
		return nil, err
	}
	return d, nil
}

func issue(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Code     string `json:"code"`
		Title    string `json:"title"`
		TenantID string `json:"tenant_id"`
		SHA256   string `json:"sha256"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	code := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(in.Code)), "HS-")
	if !codeRe.MatchString(code) || strings.TrimSpace(in.Title) == "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid document", "code (e.g. QT, FRM-23) and title are required")
	}
	if in.SHA256 != "" && !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(in.SHA256) {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid document", "sha256 must be 64 hex chars")
	}
	d, err := Issue(r.Context(), s, code, strings.TrimSpace(in.Title), in.TenantID, in.SHA256)
	if err != nil {
		return nil, err
	}
	ref := d.Ref
	if err := s.Audit(r.Context(), "issue", "document", ref); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: d}, nil
}

func get(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	d, err := scan(s.Tx.QueryRow(r.Context(), `SELECT `+cols+` FROM documents.register WHERE ref=$1`, r.PathValue("ref")))
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: d}, nil
}

func revoke(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	d, err := scan(s.Tx.QueryRow(r.Context(), `UPDATE documents.register SET status='revoked' WHERE ref=$1 RETURNING `+cols, r.PathValue("ref")))
	if err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "revoke", "document", d.Ref); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: d}, nil
}

// verify is public: it exposes only reference, type, title, status and issue date.
func verify(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ref := strings.ToUpper(r.PathValue("ref"))
	if !ValidRef(ref) {
		return &platform.Result{Status: http.StatusOK, Body: map[string]any{"valid": false, "reason": "malformed reference"}}, nil
	}
	var d struct {
		Ref, Code, Title, Status string
		IssuedAt                 time.Time
	}
	err := s.Tx.QueryRow(r.Context(), `SELECT ref, code, title, status, issued_at FROM documents.verify($1)`, ref).Scan(&d.Ref, &d.Code, &d.Title, &d.Status, &d.IssuedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return &platform.Result{Status: http.StatusOK, Body: map[string]any{"valid": false, "reason": "not found"}}, nil
	}
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{
		"valid": d.Status == "valid", "ref": d.Ref, "code": d.Code, "title": d.Title, "status": d.Status, "issued_at": d.IssuedAt,
	}}, nil
}
