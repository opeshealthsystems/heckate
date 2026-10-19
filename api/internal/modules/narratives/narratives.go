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
	"proposal":           {"PR", "Commercial and technical proposal"},
	"order_confirmation": {"OC", "Order confirmation"},
	"sow":                {"SOW", "Statement of work"},
	"letter":             {"LT", "Letter"},
	"certificate":        {"CRT-A", "Certificate"}, // CRT carries a check character in the reference
	"guarantee":          {"GP", "Guarantee policy"},
	"contract":           {"CTR", "Contract"},
	"pentest_report":     {"PTR", "Penetration test report"},
	"form":               {"", "Form"}, // code and name come from forms
}

// forms are the numbered HS-FRM forms of the specification that are filled in as titled fields.
// Forms with their own layout (27 acceptance, 38 asset issue) are served by their modules.
var forms = map[string]string{
	"FRM-06": "Transmittal and fax cover sheet", "FRM-07": "Client information and onboarding form",
	"FRM-08": "Service request and requirements questionnaire", "FRM-09": "Penetration test scoping form",
	"FRM-10": "Security assessment pre-engagement questionnaire", "FRM-11": "hecID e-KYC integration request",
	"FRM-12": "Background screening request", "FRM-13": "Client KYC and due-diligence form",
	"FRM-14": "Incident report and emergency response request", "FRM-15": "Customer feedback and satisfaction form",
	"FRM-23": "Change request form", "FRM-24": "Rate card", "FRM-26": "Project kick-off record",
	"FRM-31": "Project closure and sign-off", "FRM-32": "Job application form", "FRM-33": "Offer of employment",
	"FRM-37": "Visitor and laboratory access log", "FRM-39": "Supplier registration form", "FRM-41": "Supplier evaluation form",
}

type Section struct {
	Heading string `json:"heading"`
	Body    string `json:"body"`
}

type Narrative struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	FormCode  *string   `json:"form_code"`
	TenantID  string    `json:"tenant_id"`
	Title     string    `json:"title"`
	Sections  []Section `json:"sections"`
	Ref       *string   `json:"ref"`
	SignedBy  *string   `json:"signed_by"`
	CreatedBy string    `json:"created_by"`
	IssuedBy  *string   `json:"issued_by"`
	CreatedAt time.Time `json:"created_at"`
}

const cols = `id::text, kind, form_code, tenant_id::text, title, sections, ref, signed_by, created_by, issued_by, created_at`

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
	err := row.Scan(&n.ID, &n.Kind, &n.FormCode, &n.TenantID, &n.Title, &sec, &n.Ref, &n.SignedBy, &n.CreatedBy, &n.IssuedBy, &n.CreatedAt)
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
		Form     string    `json:"form"`
		TenantID string    `json:"tenant_id"`
		Title    string    `json:"title"`
		Sections []Section `json:"sections"`
		SignedBy string    `json:"signed_by"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if _, ok := kinds[in.Kind]; !ok {
		return nil, bad("unknown kind")
	}
	if in.Kind == "form" {
		if _, ok := forms[in.Form]; !ok {
			return nil, bad("form must be one of the supported HS-FRM codes, for example FRM-23")
		}
	} else if in.Form != "" {
		return nil, bad("form is only for kind form")
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.TenantID == "" || in.Title == "" || len(in.Title) > 200 {
		return nil, bad("tenant_id and a title of up to 200 characters are required")
	}
	if err := validSections(in.Sections); err != nil {
		return nil, err
	}
	sec, _ := json.Marshal(in.Sections)
	n, err := scan(s.Tx.QueryRow(r.Context(), `INSERT INTO documents.narratives (kind, form_code, tenant_id, title, sections, signed_by, created_by)
		VALUES ($1,NULLIF($2,''),$3,$4,$5,NULLIF($6,''),$7) RETURNING `+cols, in.Kind, in.Form, in.TenantID, in.Title, sec, strings.TrimSpace(in.SignedBy), s.Claims.Sub))
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
		Form     string    `json:"form"`      // likewise
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
	code, label := cur.codeAndLabel()
	doc, err := documents.Issue(ctx, s, code, label+": "+cur.Title, cur.TenantID, "")
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
		_, label := n.codeAndLabel()
		secs := make([]docs.Section, len(n.Sections))
		for i, x := range n.Sections {
			secs[i] = docs.Section{Heading: x.Heading, Body: x.Body}
		}
		signed := ""
		if n.SignedBy != nil {
			signed = *n.SignedBy
		}
		return docs.RenderNarrative(m.docs, *n.Ref, label, entry.IssuedAt, n.Title, client, secs, signed)
	})
}

func (n *Narrative) codeAndLabel() (code, label string) {
	if n.Kind == "form" && n.FormCode != nil {
		return *n.FormCode, forms[*n.FormCode]
	}
	return kinds[n.Kind].code, kinds[n.Kind].label
}
