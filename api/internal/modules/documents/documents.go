// Package documents is the Documents & Trust context: the document register,
// reference numbers (HS-<CODE>-<YYYY>-<NNNNNN>) and public verification.
package documents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
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

// CheckChar is the ISO/IEC 7064 MOD 37,36 check character over the alphanumerics of the reference body, the same
// calculation as the brand pack's certificate generators (HS-ASM-2026-000148 gets P, as on the approved design).
func CheckChar(body string) byte {
	const m = 36
	p := m
	for _, c := range body {
		v := strings.IndexRune(alphabet, c)
		if v < 0 {
			continue // hyphens are not part of the calculation
		}
		p = (p + v) % m
		if p == 0 {
			p = m
		}
		p = p * 2 % (m + 1)
	}
	return alphabet[(m+1-p)%m]
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

func Register(a *platform.App, portalURL string) {
	a.Route("POST /v1/documents", issuers, platform.Idempotent(issue))
	a.Route("GET /v1/documents", nil, list)
	a.Route("GET /v1/documents/{ref}", nil, get)
	a.Route("POST /v1/documents/{ref}/revoke", []string{"ceo", "manager"}, revoke)
	m := &module{portal: strings.TrimRight(portalURL, "/")}
	a.Public("GET /v1/public/verify/{ref}", m.verify)
	a.Public("GET /v1/public/verify-qr/{token}", m.verifyQR)
	a.Public("POST /v1/public/access-requests", m.requestAccess)
	a.Public("GET /v1/public/access/{token}", m.download)
	a.Route("GET /v1/documents/{ref}/verifications", issuers, m.verifications)
	a.Route("GET /v1/access-requests", deciders, m.listRequests)
	a.Route("POST /v1/access-requests/{id}/approve", deciders, platform.Idempotent(m.approve))
	a.Route("POST /v1/access-requests/{id}/deny", deciders, platform.Idempotent(m.deny))
}

type module struct{ portal string }

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

// listLimit reads ?limit=, defaulting to 50 and never returning more than 200.
func listLimit(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 50
	}
	return min(n, 200)
}

// list is the register, newest first. Row-level security gives staff everything and clients their own tenant.
func list(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+cols+` FROM documents.register ORDER BY issued_at DESC, ref DESC LIMIT $1`, listLimit(r.URL.Query().Get("limit")))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Document{}
	for rows.Next() {
		d, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
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

// Lookup returns a register entry the caller is allowed to see.
func Lookup(ctx context.Context, s *platform.Scope, ref string) (*Document, error) {
	return scan(s.Tx.QueryRow(ctx, `SELECT `+cols+` FROM documents.register WHERE ref=$1`, ref))
}

var pdfSigner atomic.Pointer[docs.PDFSigner]

// SetPDFSigner turns PDF signing on for originals rendered from now on; nil turns it off. Originals already
// stored are never re-signed: one reference stays one file.
func SetPDFSigner(s *docs.PDFSigner) { pdfSigner.Store(s) }

// ServePDF returns the canonical PDF for a document the caller has already been authorised to see.
// The first rendering is stored with its hash (and the hash goes into the public register); every
// later request gets exactly those bytes, so one reference always means one file.
func ServePDF(ctx context.Context, s *platform.Scope, ref string, render func() ([]byte, error)) (*platform.Result, error) {
	var stored []byte
	if err := s.Tx.QueryRow(ctx, `SELECT documents.get_artifact($1)`, ref).Scan(&stored); err != nil {
		return nil, err
	}
	if stored == nil {
		fresh, err := render()
		if err != nil {
			return nil, err
		}
		if ps := pdfSigner.Load(); ps != nil {
			// Sign before hashing so the register, the stored original and every download are the same signed bytes.
			// The signing time is the register's issue time, not the clock, so the signature is reproducible.
			d, err := Lookup(ctx, s, ref)
			if err != nil {
				return nil, err
			}
			if fresh, err = ps.SignPDF(fresh, d.IssuedAt); err != nil {
				return nil, fmt.Errorf("sign %s: %w", ref, err) // never store or serve an unsigned original while signing is on
			}
		}
		sum := sha256.Sum256(fresh)
		if err := s.Tx.QueryRow(ctx, `SELECT documents.put_artifact($1,$2,$3)`, ref, hex.EncodeToString(sum[:]), fresh).Scan(&stored); err != nil {
			return nil, err
		}
	}
	sum := sha256.Sum256(stored)
	return &platform.Result{
		Status: http.StatusOK, Bytes: stored, ContentType: "application/pdf",
		Header: map[string]string{
			"Content-Disposition":    `attachment; filename="` + ref + `.pdf"`,
			"X-Document-SHA256":      hex.EncodeToString(sum[:]),
			"X-Content-Type-Options": "nosniff",
			"Cache-Control":          "private, no-store",
		},
	}, nil
}
