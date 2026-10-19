// Package narratives holds text documents for clients (proposals, order confirmations, statements of
// work, letters, certificates) as editable drafts that are frozen and numbered when issued.
package narratives

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
	"github.com/opeshealthsystems/heckate/api/internal/modules/clients"
	"github.com/opeshealthsystems/heckate/api/internal/modules/documents"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

var writers = []string{"ceo", "manager", "sales"}

// kinds maps each kind to its reference code and the heading printed on the PDF.
var kinds = map[string]struct{ code, label string }{
	"proposal":           {"PRP", "Proposal"},
	"order_confirmation": {"OC", "Order confirmation"},
	"sow":                {"SOW", "Statement of work"},
	"letter":             {"LTR", "Letter"},
	"certificate":        {"CRT", "Certificate"},
}

type Section struct {
	Heading string `json:"heading"`
	Body    string `json:"body"`
}

type Narrative struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	TenantID  string    `json:"tenant_id"`
	Title     string    `json:"title"`
	Sections  []Section `json:"sections"`
	Ref       *string   `json:"ref"`
	SignedBy  *string   `json:"signed_by"`
	CreatedBy string    `json:"created_by"`
	IssuedBy  *string   `json:"issued_by"`
	CreatedAt time.Time `json:"created_at"`
}

const cols = `id::text, kind, tenant_id::text, title, sections, ref, signed_by, created_by, issued_by, created_at`

type module struct{ docs docs.Options }

func Register(a *platform.App, o docs.Options) {
	m := &module{docs: o}
	a.Route("POST /v1/narratives", writers, platform.Idempotent(create))
	a.Route("GET /v1/narratives", nil, list)
	a.Route("GET /v1/narratives/{id}", nil, get)
	a.Route("PUT /v1/narratives/{id}", writers, update)
	a.Route("POST /v1/narratives/{id}/issue", writers, platform.Idempotent(issue))
	a.Route("GET /v1/narratives/{id}/pdf", nil, m.pdf)
}

func bad(msg string) error {
	return platform.Err(http.StatusUnprocessableEntity, "Invalid request", msg)
}

func scan(row pgx.Row) (*Narrative, error) {
	var n Narrative
	var sec []byte
	err := row.Scan(&n.ID, &n.Kind, &n.TenantID, &n.Title, &sec, &n.Ref, &n.SignedBy, &n.CreatedBy, &n.IssuedBy, &n.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, platform.Err(http.StatusNotFound, "Not found", "document not found")
	}
	if err != nil {
		return nil, err
	}
	return &n, json.Unmarshal(sec, &n.Sections)
}

func validSections(in []Section) error {
	if len(in) == 0 || len(in) > 60 {
		return bad("between 1 and 60 sections are required")
	}
	total := 0
	for _, s := range in {
		if strings.TrimSpace(s.Body) == "" {
			return bad("every section needs text")
		}
		total += len(s.Body) + len(s.Heading)
	}
	if total > 200_000 {
		return bad("the document is too long")
	}
	return nil
}

func create(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Kind     string    `json:"kind"`
		TenantID string    `json:"tenant_id"`
		Title    string    `json:"title"`
		Sections []Section `json:"sections"`
		SignedBy string    `json:"signed_by"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if _, ok := kinds[in.Kind]; !ok {
		return nil, bad("kind must be proposal, order_confirmation, sow, letter or certificate")
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.TenantID == "" || in.Title == "" || len(in.Title) > 200 {
		return nil, bad("tenant_id and a title of up to 200 characters are required")
	}
	if err := validSections(in.Sections); err != nil {
		return nil, err
	}
	sec, _ := json.Marshal(in.Sections)
	n, err := scan(s.Tx.QueryRow(r.Context(), `INSERT INTO documents.narratives (kind, tenant_id, title, sections, signed_by, created_by)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6) RETURNING `+cols, in.Kind, in.TenantID, in.Title, sec, strings.TrimSpace(in.SignedBy), s.Claims.Sub))
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "invalid input syntax") {
			return nil, bad("unknown client")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "create", "narrative", n.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: n}, nil
}

func list(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	tc, ta := s.TenantFilter("tenant_id", 1)
	rows, err := s.Tx.Query(r.Context(), `SELECT `+cols+` FROM documents.narratives WHERE true`+tc+` ORDER BY created_at DESC LIMIT 200`, ta...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Narrative{}
	for rows.Next() {
		n, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func get(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	n, err := scan(s.Tx.QueryRow(r.Context(), `SELECT `+cols+` FROM documents.narratives WHERE id::text=$1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: n}, nil
}

func update(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Kind     string    `json:"kind"`      // accepted so a client can send back what it read; never applied
		TenantID string    `json:"tenant_id"` // likewise
		Title    string    `json:"title"`
		Sections []Section `json:"sections"`
		SignedBy string    `json:"signed_by"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" || len(in.Title) > 200 {
		return nil, bad("a title of up to 200 characters is required")
	}
	if err := validSections(in.Sections); err != nil {
		return nil, err
	}
	ctx := r.Context()
	cur, err := scan(s.Tx.QueryRow(ctx, `SELECT `+cols+` FROM documents.narratives WHERE id::text=$1 FOR UPDATE`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	if cur.Ref != nil {
		return nil, platform.Err(http.StatusConflict, "Already issued", "an issued document cannot be edited")
	}
	sec, _ := json.Marshal(in.Sections)
	n, err := scan(s.Tx.QueryRow(ctx, `UPDATE documents.narratives SET title=$2, sections=$3, signed_by=NULLIF($4,'') WHERE id=$1 RETURNING `+cols,
		cur.ID, in.Title, sec, strings.TrimSpace(in.SignedBy)))
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: n}, nil
}

func issue(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	cur, err := scan(s.Tx.QueryRow(ctx, `SELECT `+cols+` FROM documents.narratives WHERE id::text=$1 FOR UPDATE`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	if cur.Ref != nil {
		return nil, platform.Err(http.StatusConflict, "Already issued", "this document has already been issued")
	}
	doc, err := documents.Issue(ctx, s, kinds[cur.Kind].code, kinds[cur.Kind].label+": "+cur.Title, cur.TenantID, "")
	if err != nil {
		return nil, err
	}
	n, err := scan(s.Tx.QueryRow(ctx, `UPDATE documents.narratives SET ref=$2, issued_by=$3 WHERE id=$1 RETURNING `+cols, cur.ID, doc.Ref, s.Claims.Sub))
	if err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "issue", "narrative", n.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: n}, nil
}

func (m *module) pdf(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	n, err := scan(s.Tx.QueryRow(ctx, `SELECT `+cols+` FROM documents.narratives WHERE id::text=$1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	if n.Ref == nil {
		return nil, platform.Err(http.StatusConflict, "Not issued", "only an issued document has a PDF")
	}
	return documents.ServePDF(ctx, s, *n.Ref, func() ([]byte, error) {
		client, err := clients.Party(ctx, s, n.TenantID)
		if err != nil {
			return nil, err
		}
		entry, err := documents.Lookup(ctx, s, *n.Ref)
		if err != nil {
			return nil, err
		}
		secs := make([]docs.Section, len(n.Sections))
		for i, x := range n.Sections {
			secs[i] = docs.Section{Heading: x.Heading, Body: x.Body}
		}
		signed := ""
		if n.SignedBy != nil {
			signed = *n.SignedBy
		}
		return docs.RenderNarrative(m.docs, *n.Ref, kinds[n.Kind].label, entry.IssuedAt, n.Title, client, secs, signed)
	})
}
