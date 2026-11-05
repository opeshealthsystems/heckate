// Package forms lets staff send a form to a client to fill in online, takes in the answers (typed by the client
// or keyed in from a paper form), and records the review. Every change is kept in an append-only history.
package forms

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/internal/reports"
)

var (
	senders = []string{"ceo", "manager", "sales", "support", "engineer"}
	filers  = platform.Join(senders, platform.ClientAll)
)

func Register(a *platform.App) {
	a.Route("POST /v1/form-requests", senders, platform.Idempotent(send))
	a.Route("GET /v1/form-requests", filers, list)
	a.Route("GET /v1/form-requests/{id}", filers, get)
	a.Route("PUT /v1/form-requests/{id}/answers", filers, saveAnswers)
	a.Route("POST /v1/form-requests/{id}/submit", filers, platform.Idempotent(submit))
	a.Route("POST /v1/form-requests/{id}/enter", senders, platform.Idempotent(enter))
	a.Route("POST /v1/form-requests/{id}/accept", senders, platform.Idempotent(review("accepted")))
	a.Route("POST /v1/form-requests/{id}/return", senders, platform.Idempotent(review("returned")))
	a.Route("POST /v1/form-requests/{id}/cancel", senders, platform.Idempotent(cancel))
	a.Route("GET /v1/exports/form-requests", senders, export)
}

func nf() error { return platform.Err(http.StatusNotFound, "Not found", "form not found") }
func bad(m string) error {
	return platform.Err(http.StatusUnprocessableEntity, "Invalid form", m)
}
func isNF(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || (err != nil && strings.Contains(err.Error(), "invalid input syntax"))
}

// Field is one question. Types: text, textarea, number, date, select, checkbox.
type Field struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
}

var keyRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
var fieldTypes = map[string]bool{"text": true, "textarea": true, "number": true, "date": true, "select": true, "checkbox": true}

// Validate checks the question list.
func Validate(fs []Field) error {
	if len(fs) == 0 || len(fs) > 100 {
		return bad("a form has 1 to 100 questions")
	}
	seen := map[string]bool{}
	for _, f := range fs {
		switch {
		case !keyRE.MatchString(f.Key):
			return bad("each question needs a key of lowercase letters, digits and underscores, starting with a letter")
		case seen[f.Key]:
			return bad("duplicate question key " + f.Key)
		case strings.TrimSpace(f.Label) == "" || len(f.Label) > 300:
			return bad("each question needs a label of up to 300 characters")
		case !fieldTypes[f.Type]:
			return bad("unknown question type " + f.Type)
		case f.Type == "select" && len(f.Options) == 0:
			return bad("a select question needs options")
		}
		seen[f.Key] = true
	}
	return nil
}

// CheckAnswers validates answers against the questions. Unknown keys are refused; required ones must be present
// when the form is submitted (final), but a draft may be partial.
func CheckAnswers(fs []Field, ans map[string]any, final bool) error {
	byKey := map[string]Field{}
	for _, f := range fs {
		byKey[f.Key] = f
	}
	for k, v := range ans {
		f, ok := byKey[k]
		if !ok {
			return bad("unknown answer " + k)
		}
		if v == nil || v == "" {
			continue
		}
		switch f.Type {
		case "text", "textarea":
			s, ok := v.(string)
			if !ok || len(s) > 5000 {
				return bad(f.Label + " must be text of up to 5000 characters")
			}
		case "number":
			switch x := v.(type) {
			case float64:
			case string:
				if _, err := strconv.ParseFloat(x, 64); err != nil {
					return bad(f.Label + " must be a number")
				}
			default:
				return bad(f.Label + " must be a number")
			}
		case "date":
			s, ok := v.(string)
			if _, err := time.Parse("2006-01-02", s); !ok || err != nil {
				return bad(f.Label + " must be a date (YYYY-MM-DD)")
			}
		case "select":
			s, _ := v.(string)
			okOpt := false
			for _, o := range f.Options {
				okOpt = okOpt || o == s
			}
			if !okOpt {
				return bad(f.Label + " must be one of the listed options")
			}
		case "checkbox":
			if _, ok := v.(bool); !ok {
				return bad(f.Label + " must be yes or no")
			}
		}
	}
	if final {
		for _, f := range fs {
			v, has := ans[f.Key]
			empty := !has || v == nil || v == ""
			if f.Type == "checkbox" {
				empty = !has || v == nil // an unticked box is still an answer
			}
			if f.Required && empty {
				return bad("please answer: " + f.Label)
			}
		}
	}
	return nil
}

type Request struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	Client      string         `json:"client"`
	FormCode    string         `json:"form_code"`
	Title       string         `json:"title"`
	Message     string         `json:"message"`
	Fields      []Field        `json:"fields"`
	Answers     map[string]any `json:"answers"`
	Status      string         `json:"status"`
	DueDate     *time.Time     `json:"due_date"`
	ProjectID   *string        `json:"project_id"`
	ContractID  *string        `json:"contract_id"`
	SentBy      string         `json:"sent_by"`
	SentAt      time.Time      `json:"sent_at"`
	SubmittedBy *string        `json:"submitted_by"`
	SubmittedAt *time.Time     `json:"submitted_at"`
	OnBehalf    bool           `json:"on_behalf"`
	ReviewedBy  *string        `json:"reviewed_by"`
	ReviewedAt  *time.Time     `json:"reviewed_at"`
	ReviewNote  *string        `json:"review_note"`
	History     *[]Event       `json:"history,omitempty"`
}

type Event struct {
	Action string    `json:"action"`
	Actor  string    `json:"actor"`
	Note   *string   `json:"note"`
	At     time.Time `json:"at"`
}

const cols = `r.id, r.tenant_id, COALESCE(t.name,''), r.form_code, r.title, r.message, r.fields, r.answers, r.status, r.due_date, r.project_id::text, r.contract_id::text,
	r.sent_by, r.sent_at, r.submitted_by, r.submitted_at, r.on_behalf, r.reviewed_by, r.reviewed_at, r.review_note`
const from = ` FROM forms.requests r LEFT JOIN platform.tenants t ON t.id = r.tenant_id`

func scan(row pgx.Row) (*Request, error) {
	var x Request
	var fields, answers []byte
	err := row.Scan(&x.ID, &x.TenantID, &x.Client, &x.FormCode, &x.Title, &x.Message, &fields, &answers, &x.Status, &x.DueDate, &x.ProjectID, &x.ContractID,
		&x.SentBy, &x.SentAt, &x.SubmittedBy, &x.SubmittedAt, &x.OnBehalf, &x.ReviewedBy, &x.ReviewedAt, &x.ReviewNote)
	if isNF(err) {
		return nil, nf()
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(fields, &x.Fields); err != nil {
		return nil, err
	}
	x.Answers = map[string]any{}
	_ = json.Unmarshal(answers, &x.Answers)
	return &x, nil
}

func log(r *http.Request, s *platform.Scope, id, tenant, action string, answers map[string]any, note string) error {
	var a any
	if answers != nil {
		b, _ := json.Marshal(answers)
		a = b
	}
	_, err := s.Tx.Exec(r.Context(), `INSERT INTO forms.events (request_id, tenant_id, action, actor, answers, note) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''))`, id, tenant, action, s.Claims.Sub, a, note)
	return err
}

func send(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		TenantID   string  `json:"tenant_id"`
		FormCode   string  `json:"form_code"`
		Title      string  `json:"title"`
		Message    string  `json:"message"`
		Fields     []Field `json:"fields"`
		DueDate    string  `json:"due_date"`
		ProjectID  string  `json:"project_id"`
		ContractID string  `json:"contract_id"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.TenantID == "" || strings.TrimSpace(in.Title) == "" {
		return nil, bad("tenant_id and title are required")
	}
	if err := Validate(in.Fields); err != nil {
		return nil, err
	}
	var due any
	if in.DueDate != "" {
		d, err := time.Parse("2006-01-02", in.DueDate)
		if err != nil {
			return nil, bad("due_date must be YYYY-MM-DD")
		}
		due = d
	}
	fb, _ := json.Marshal(in.Fields)
	var id string
	err := s.Tx.QueryRow(r.Context(), `INSERT INTO forms.requests (tenant_id, form_code, title, message, fields, due_date, project_id, contract_id, sent_by)
		VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,'')::uuid,NULLIF($8,'')::uuid,$9) RETURNING id::text`,
		in.TenantID, in.FormCode, strings.TrimSpace(in.Title), in.Message, fb, due, in.ProjectID, in.ContractID, s.Claims.Sub).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") {
			return nil, bad("unknown tenant")
		}
		if strings.Contains(err.Error(), "invalid input syntax") {
			return nil, bad("project_id and contract_id must be ids")
		}
		return nil, err
	}
	if err := log(r, s, id, in.TenantID, "sent", nil, ""); err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "create", "form_request", id); err != nil {
		return nil, err
	}
	if err := s.Emit(r.Context(), "form.sent", map[string]any{"id": id, "tenant_id": in.TenantID, "title": in.Title}); err != nil {
		return nil, err
	}
	x, err := scan(s.Tx.QueryRow(r.Context(), `SELECT `+cols+from+` WHERE r.id::text=$1`, id))
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: x}, nil
}

func list(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	q := `SELECT ` + cols + from + ` WHERE true`
	var args []any
	if st := r.URL.Query().Get("status"); st != "" {
		args = append(args, st)
		q += fmt.Sprintf(` AND r.status = $%d`, len(args))
	}
	rows, err := s.Tx.Query(r.Context(), q+` ORDER BY r.sent_at DESC LIMIT 300`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Request{}
	for rows.Next() {
		x, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func load(r *http.Request, s *platform.Scope, id string, forUpdate bool) (*Request, error) {
	q := `SELECT ` + cols + from + ` WHERE r.id::text=$1`
	if forUpdate {
		q += ` FOR UPDATE OF r`
	}
	return scan(s.Tx.QueryRow(r.Context(), q, id))
}

func get(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	x, err := load(r, s, r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	rows, err := s.Tx.Query(r.Context(), `SELECT action, actor, note, at FROM forms.events WHERE request_id=$1 ORDER BY id`, x.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	h := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Action, &e.Actor, &e.Note, &e.At); err != nil {
			return nil, err
		}
		h = append(h, e)
	}
	x.History = &h
	return &platform.Result{Status: http.StatusOK, Body: x}, rows.Err()
}

// writeAnswers stores answers on a request that is open for filling.
func writeAnswers(r *http.Request, s *platform.Scope, x *Request, ans map[string]any, final bool) error {
	if x.Status != "sent" && x.Status != "returned" {
		return platform.Err(http.StatusConflict, "Closed", "this form is not open for answers")
	}
	if err := CheckAnswers(x.Fields, ans, final); err != nil {
		return err
	}
	b, _ := json.Marshal(ans)
	_, err := s.Tx.Exec(r.Context(), `UPDATE forms.requests SET answers=$2 WHERE id=$1`, x.ID, b)
	return err
}

func answersBody(r *http.Request) (map[string]any, error) {
	var in struct {
		Answers map[string]any `json:"answers"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.Answers == nil {
		in.Answers = map[string]any{}
	}
	return in.Answers, nil
}

// saveAnswers keeps a partly filled form so the client can come back to it.
func saveAnswers(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ans, err := answersBody(r)
	if err != nil {
		return nil, err
	}
	x, err := load(r, s, r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if err := writeAnswers(r, s, x, ans, false); err != nil {
		return nil, err
	}
	return get(r, s)
}

func finish(r *http.Request, s *platform.Scope, onBehalf bool) (*platform.Result, error) {
	ans, err := answersBody(r)
	if err != nil {
		return nil, err
	}
	x, err := load(r, s, r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if err := writeAnswers(r, s, x, ans, true); err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(r.Context(), `UPDATE forms.requests SET status='submitted', submitted_by=$2, submitted_at=now(), on_behalf=$3, reviewed_by=NULL, reviewed_at=NULL WHERE id=$1`, x.ID, s.Claims.Sub, onBehalf); err != nil {
		return nil, err
	}
	action := "submitted"
	if onBehalf {
		action = "entered_from_paper"
	}
	if err := log(r, s, x.ID, x.TenantID, action, ans, ""); err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), action, "form_request", x.ID); err != nil {
		return nil, err
	}
	if err := s.Emit(r.Context(), "form.submitted", map[string]any{"id": x.ID, "tenant_id": x.TenantID, "title": x.Title, "actor": s.Claims.Sub}); err != nil {
		return nil, err
	}
	return get(r, s)
}

// submit is the client sending the form back. Staff use it too if they fill in a form with the client on the phone.
func submit(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	return finish(r, s, false)
}

// enter records a paper form the client filled in by hand: staff key in the answers and mark it as such.
func enter(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	return finish(r, s, true)
}

func review(to string) platform.HandlerFunc {
	return func(r *http.Request, s *platform.Scope) (*platform.Result, error) {
		var in struct {
			Note string `json:"note"`
		}
		if err := platform.DecodeBody(r, &in); err != nil {
			return nil, err
		}
		if to == "returned" && strings.TrimSpace(in.Note) == "" {
			return nil, bad("say what needs to change")
		}
		x, err := load(r, s, r.PathValue("id"), true)
		if err != nil {
			return nil, err
		}
		if x.Status != "submitted" {
			return nil, platform.Err(http.StatusConflict, "Not submitted", "only a submitted form can be reviewed")
		}
		if _, err := s.Tx.Exec(r.Context(), `UPDATE forms.requests SET status=$2, reviewed_by=$3, reviewed_at=now(), review_note=NULLIF($4,'') WHERE id=$1`, x.ID, to, s.Claims.Sub, in.Note); err != nil {
			return nil, err
		}
		if err := log(r, s, x.ID, x.TenantID, to, nil, in.Note); err != nil {
			return nil, err
		}
		if err := s.Audit(r.Context(), to, "form_request", x.ID); err != nil {
			return nil, err
		}
		if to == "returned" {
			if err := s.Emit(r.Context(), "form.returned", map[string]any{"id": x.ID, "tenant_id": x.TenantID, "title": x.Title}); err != nil {
				return nil, err
			}
		}
		return get(r, s)
	}
}

func cancel(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	x, err := load(r, s, r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if x.Status == "accepted" || x.Status == "cancelled" {
		return nil, platform.Err(http.StatusConflict, "Closed", "this form is already closed")
	}
	if _, err := s.Tx.Exec(r.Context(), `UPDATE forms.requests SET status='cancelled' WHERE id=$1`, x.ID); err != nil {
		return nil, err
	}
	if err := log(r, s, x.ID, x.TenantID, "cancelled", nil, ""); err != nil {
		return nil, err
	}
	return get(r, s)
}

func export(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	res, err := list(r, s)
	if err != nil {
		return nil, err
	}
	data, err := reports.Rows(res, "data")
	if err != nil {
		return nil, err
	}
	return reports.CSV("form-requests", []reports.Col{reports.C("Client", "client"), reports.C("Form", "title"), reports.C("Code", "form_code"), reports.C("Status", "status"), reports.C("Sent", "sent_at"),
		reports.C("Sent by", "sent_by"), reports.C("Submitted", "submitted_at"), reports.C("Submitted by", "submitted_by"), reports.C("Keyed from paper", "on_behalf"), reports.C("Reviewed by", "reviewed_by")}, data)
}
