package documents

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// deciders may see access requests and grant or refuse them.
var deciders = []string{"ceo", "manager"}

// signature is what the verification says about the QR signature, when a signed code was scanned.
type signature struct {
	Checked bool   `json:"checked"`
	Valid   bool   `json:"valid"`
	Scheme  string `json:"scheme,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func notFoundBody(reason string) map[string]any {
	return map[string]any{"valid": false, "result": "not_found", "reason": reason}
}

// answer builds the public verification answer for a reference and records the lookup. It exposes only what a
// stranger holding the document may know: what it is, when it was issued, its version and classification, whether
// it is still current, and its fingerprint. Never who it was issued to.
func (m *module) answer(r *http.Request, s *platform.Scope, ref, channel string, sig *signature, signed *docs.SignedQR) (map[string]any, error) {
	ctx := r.Context()
	var d struct {
		Ref, Code, Title, Status string
		IssuedAt                 time.Time
		SHA256                   *string
	}
	err := s.Tx.QueryRow(ctx, `SELECT ref, code, title, status, issued_at, sha256 FROM documents.verify($1)`, ref).Scan(&d.Ref, &d.Code, &d.Title, &d.Status, &d.IssuedAt, &d.SHA256)
	if errors.Is(err, pgx.ErrNoRows) {
		return notFoundBody("not found"), nil
	}
	if err != nil {
		return nil, err
	}
	var info struct {
		Revision  int
		Newer     *string
		Count     int64
		LastCheck *time.Time
	}
	// A signed code must agree with the register: same date, same classification, same version.
	if signed != nil {
		class := map[string]string{"PUBLIC": "P", "INTERNAL": "I", "CONFIDENTIAL": "C", "RESTRICTED": "R"}[docs.Classification(d.Ref)]
		if err := s.Tx.QueryRow(ctx, `SELECT revision FROM documents.public_info($1)`, ref).Scan(&info.Revision); err != nil {
			return nil, err
		}
		if signed.Date != d.IssuedAt.UTC().Format("20060102") || signed.Class != class || signed.Version != fmt.Sprintf("%d.0", info.Revision) {
			if _, err := s.Tx.Exec(ctx, `SELECT documents.log_verification($1,$2,'signature_invalid')`, ref, channel); err != nil {
				return nil, err
			}
			b := notFoundBody("the signed code does not match this document's record")
			b["result"] = "signature_invalid"
			b["signature"] = signature{Checked: true, Valid: false, Scheme: "ed25519", Reason: "does not match the record"}
			return b, nil
		}
	}
	if err := s.Tx.QueryRow(ctx, `SELECT revision, superseded_by FROM documents.public_info($1)`, ref).Scan(&info.Revision, &info.Newer); err != nil {
		return nil, err
	}
	result := "authentic"
	switch {
	case d.Status != "valid":
		result = "revoked"
	case info.Newer != nil:
		result = "replaced"
	}
	if _, err := s.Tx.Exec(ctx, `SELECT documents.log_verification($1,$2,$3)`, ref, channel, result); err != nil {
		return nil, err
	}
	if err := s.Tx.QueryRow(ctx, `SELECT verified_count, last_verified FROM documents.public_info($1)`, ref).Scan(&info.Count, &info.LastCheck); err != nil {
		return nil, err
	}
	body := map[string]any{
		"valid": d.Status == "valid", "result": result, "ref": d.Ref, "code": d.Code, "type": TypeName(d.Code), "title": d.Title,
		"status": d.Status, "issued_at": d.IssuedAt, "version": fmt.Sprintf("%d.0", info.Revision), "revision": info.Revision,
		"classification": strings.ToLower(docs.Classification(d.Ref)), "issued_by": "HEKATE SYSTEMS SARL",
		"verified_count": info.Count, "last_verified": info.LastCheck,
	}
	if d.SHA256 != nil {
		body["sha256"] = *d.SHA256 // lets anyone check a file they hold against the original
	}
	if info.Newer != nil {
		body["superseded_by"] = *info.Newer // a replaced document stays authentic; say so and point to the newer one
	}
	if sig != nil {
		body["signature"] = sig
	}
	return body, nil
}

func (m *module) verify(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ref := strings.ToUpper(r.PathValue("ref"))
	if !ValidRef(ref) {
		return &platform.Result{Status: http.StatusOK, Body: map[string]any{"valid": false, "result": "malformed", "reason": "malformed reference"}}, nil
	}
	b, err := m.answer(r, s, ref, "reference", nil, nil)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: b}, nil
}

// verifyQR checks a scanned signed address: the signature first (against HEKATE's published public key), then the
// register. A code that is not signed by HEKATE is refused however convincing the rest looks.
func (m *module) verifyQR(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	q, err := docs.ParseSignedQR(r.PathValue("token"))
	if err != nil {
		body := map[string]any{"valid": false, "result": "signature_invalid", "reason": err.Error(), "signature": signature{Checked: true, Valid: false, Scheme: "ed25519", Reason: err.Error()}}
		// If the reference part is real, note that someone presented a forged code for it.
		if parts := strings.SplitN(strings.TrimPrefix(strings.TrimSpace(r.PathValue("token")), "/"), ".", 2); len(parts) > 0 && ValidRef(strings.ToUpper(parts[0])) {
			if _, e := s.Tx.Exec(r.Context(), `SELECT documents.log_verification($1,'qr','signature_invalid')`, strings.ToUpper(parts[0])); e != nil {
				return nil, e
			}
		}
		return &platform.Result{Status: http.StatusOK, Body: body}, nil
	}
	if !ValidRef(q.Ref) {
		return &platform.Result{Status: http.StatusOK, Body: map[string]any{"valid": false, "result": "malformed", "reason": "malformed reference"}}, nil
	}
	b, err := m.answer(r, s, q.Ref, "qr", &signature{Checked: true, Valid: true, Scheme: "ed25519"}, &q)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: b}, nil
}

// ----- access requests -----

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func (m *module) requestAccess(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Ref          string `json:"ref"`
		Name         string `json:"name"`
		Email        string `json:"email"`
		Organisation string `json:"organisation"`
		Reason       string `json:"reason"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	in.Ref = strings.ToUpper(strings.TrimSpace(in.Ref))
	in.Name, in.Email, in.Organisation, in.Reason = strings.TrimSpace(in.Name), strings.TrimSpace(in.Email), strings.TrimSpace(in.Organisation), strings.TrimSpace(in.Reason)
	bad := func(d string) error { return platform.Err(http.StatusUnprocessableEntity, "Invalid request", d) }
	switch {
	case !ValidRef(in.Ref):
		return nil, bad("a valid document reference is required")
	case len(in.Name) < 2 || len(in.Name) > 120:
		return nil, bad("your name is required")
	case len(in.Email) > 200 || !emailRe.MatchString(in.Email):
		return nil, bad("a valid email address is required")
	case len(in.Organisation) > 200:
		return nil, bad("organisation is too long")
	case len([]rune(in.Reason)) < 20 || len([]rune(in.Reason)) > 1000:
		return nil, bad("please explain in 20 to 1000 characters why you need to see this document")
	}
	var id string
	err := s.Tx.QueryRow(r.Context(), `SELECT documents.request_access($1,$2,$3,$4,$5)::text`, in.Ref, in.Name, in.Email, in.Organisation, in.Reason).Scan(&id)
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch pe.Code {
		case "P0002":
			return nil, platform.Err(http.StatusNotFound, "Not found", "no such document")
		case "P0003":
			return nil, platform.Err(http.StatusTooManyRequests, "Too many requests", "you already have requests waiting for this document; HEKATE will reply by email")
		}
	}
	if err != nil {
		return nil, err
	}
	if err := s.Emit(r.Context(), "document.access_requested", map[string]any{"id": id, "ref": in.Ref}); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: map[string]any{"id": id, "status": "pending"}}, nil
}

type AccessRequest struct {
	ID           string     `json:"id"`
	Ref          string     `json:"ref"`
	Title        string     `json:"title"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	Organisation string     `json:"organisation"`
	Reason       string     `json:"reason"`
	Status       string     `json:"status"`
	RequestedAt  time.Time  `json:"requested_at"`
	DecidedBy    *string    `json:"decided_by"`
	DecidedAt    *time.Time `json:"decided_at"`
	DecisionNote string     `json:"decision_note"`
}

func (m *module) listRequests(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	status := r.URL.Query().Get("status")
	rows, err := s.Tx.Query(r.Context(), `
		SELECT a.id::text, a.ref, d.title, a.name, a.email, a.organisation, a.reason, a.status, a.requested_at, a.decided_by, a.decided_at, a.decision_note
		FROM documents.access_requests a JOIN documents.register d ON d.ref = a.ref
		WHERE ($1 = '' OR a.status = $1) ORDER BY (a.status = 'pending') DESC, a.requested_at DESC LIMIT 200`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AccessRequest{}
	for rows.Next() {
		var x AccessRequest
		if err := rows.Scan(&x.ID, &x.Ref, &x.Title, &x.Name, &x.Email, &x.Organisation, &x.Reason, &x.Status, &x.RequestedAt, &x.DecidedBy, &x.DecidedAt, &x.DecisionNote); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func hashToken(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

// approve grants a time-limited download link to the stored original of the document. HEKATE sends the link to the
// requester (the platform has no email channel yet); only a hash of the token is kept, so the link is shown once.
func (m *module) approve(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Days         int    `json:"days"`
		MaxDownloads int    `json:"max_downloads"`
		Note         string `json:"note"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.Days == 0 {
		in.Days = 7
	}
	if in.MaxDownloads == 0 {
		in.MaxDownloads = 3
	}
	in.Note = strings.TrimSpace(in.Note)
	if in.Days < 1 || in.Days > 30 || in.MaxDownloads < 1 || in.MaxDownloads > 10 || len(in.Note) > 500 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid request", "days must be 1 to 30, downloads 1 to 10, and the note at most 500 characters")
	}
	ctx := r.Context()
	var ref, status string
	if err := s.Tx.QueryRow(ctx, `SELECT ref, status FROM documents.access_requests WHERE id::text = $1 FOR UPDATE`, r.PathValue("id")).Scan(&ref, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, platform.Err(http.StatusNotFound, "Not found", "request not found")
		}
		return nil, err
	}
	if status != "pending" {
		return nil, platform.Err(http.StatusConflict, "Already decided", "this request has already been decided")
	}
	var have bool
	if err := s.Tx.QueryRow(ctx, `SELECT documents.get_artifact($1) IS NOT NULL`, ref).Scan(&have); err != nil {
		return nil, err
	}
	if !have {
		return nil, platform.Err(http.StatusConflict, "No original yet", "the PDF of this document has not been produced yet; open or download it once, then approve")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expires := time.Now().UTC().Add(time.Duration(in.Days) * 24 * time.Hour)
	if _, err := s.Tx.Exec(ctx, `UPDATE documents.access_requests SET status='approved', decided_by=$2, decided_at=now(), decision_note=$3 WHERE id::text=$1`, r.PathValue("id"), s.Claims.Sub, in.Note); err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `INSERT INTO documents.access_grants (request_id, ref, token_hash, expires_at, max_downloads, created_by) VALUES ($1::uuid,$2,$3,$4,$5,$6)`,
		r.PathValue("id"), ref, hashToken(token), expires, in.MaxDownloads, s.Claims.Sub); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "approve", "access_request", r.PathValue("id")); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{
		"status": "approved", "link": m.portal + "/access/" + token, "expires_at": expires, "max_downloads": in.MaxDownloads,
	}}, nil
}

func (m *module) deny(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Note string `json:"note"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	in.Note = strings.TrimSpace(in.Note)
	if len(in.Note) > 500 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid request", "the note can be at most 500 characters")
	}
	tag, err := s.Tx.Exec(r.Context(), `UPDATE documents.access_requests SET status='denied', decided_by=$2, decided_at=now(), decision_note=$3 WHERE id::text=$1 AND status='pending'`, r.PathValue("id"), s.Claims.Sub, in.Note)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, platform.Err(http.StatusConflict, "Not pending", "no pending request with this id")
	}
	if err := s.Audit(r.Context(), "deny", "access_request", r.PathValue("id")); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"status": "denied"}}, nil
}

// download serves the stored original to the holder of a granted link, and counts the download.
func (m *module) download(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var ref *string
	if err := s.Tx.QueryRow(r.Context(), `SELECT documents.redeem_grant($1)`, hashToken(r.PathValue("token"))).Scan(&ref); err != nil {
		return nil, err
	}
	if ref == nil {
		return nil, platform.Err(http.StatusNotFound, "Link not valid", "this link does not exist, has expired, has been used up, or the document was withdrawn")
	}
	var content []byte
	if err := s.Tx.QueryRow(r.Context(), `SELECT documents.get_artifact($1)`, *ref).Scan(&content); err != nil || len(content) == 0 {
		return nil, platform.Err(http.StatusNotFound, "Link not valid", "the document is not available")
	}
	sum := sha256.Sum256(content)
	return &platform.Result{Status: http.StatusOK, Bytes: content, ContentType: "application/pdf", Header: map[string]string{
		"Content-Disposition": `attachment; filename="` + *ref + `.pdf"`, "X-Document-SHA256": hex.EncodeToString(sum[:]),
		"X-Content-Type-Options": "nosniff", "Cache-Control": "private, no-store",
	}}, nil
}

// verifications summarises how often a document has been checked, for the people who issued it.
func (m *module) verifications(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ref := strings.ToUpper(r.PathValue("ref"))
	rows, err := s.Tx.Query(r.Context(), `SELECT at, channel, result FROM documents.verifications WHERE ref = $1 ORDER BY at DESC LIMIT 20`, ref)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type ev struct {
		At      time.Time `json:"at"`
		Channel string    `json:"channel"`
		Result  string    `json:"result"`
	}
	recent := []ev{}
	for rows.Next() {
		var e ev
		if err := rows.Scan(&e.At, &e.Channel, &e.Result); err != nil {
			return nil, err
		}
		recent = append(recent, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var total int64
	var first, last *time.Time
	if err := s.Tx.QueryRow(r.Context(), `SELECT count(*), min(at), max(at) FROM documents.verifications WHERE ref = $1`, ref).Scan(&total, &first, &last); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"ref": ref, "total": total, "first": first, "last": last, "recent": recent}}, nil
}
