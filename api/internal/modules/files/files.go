// Package files stores attachments: incident photos, intervention photos and expense receipts.
package files

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/modules/hr"
	"github.com/opeshealthsystems/heckate/api/internal/modules/projects"
	"github.com/opeshealthsystems/heckate/api/internal/modules/support"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

const MaxSize = 10 << 20 // 10 MB

// allowed lists what we store. The type is detected from the bytes, never taken from the client,
// and nothing executable or renderable as a page (SVG, HTML) is accepted.
var allowed = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true, "application/pdf": true}

// Detect returns the content type of data if it is an allowed attachment type.
func Detect(data []byte) (string, bool) {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	t := http.DetectContentType(head)
	return t, allowed[t]
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._ -]`)

// SafeName makes a client-supplied file name safe for storage and for a Content-Disposition header.
func SafeName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Trim(unsafeName.ReplaceAllString(name, "_"), " .")
	if name == "" {
		name = "file"
	}
	if len(name) > 100 {
		name = name[len(name)-100:]
	}
	return name
}

type Meta struct {
	ID          string    `json:"id"`
	EntityType  string    `json:"entity_type"`
	EntityID    string    `json:"entity_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Size        int       `json:"size"`
	SHA256      string    `json:"sha256"`
	Owner       string    `json:"owner"`
	CreatedAt   time.Time `json:"created_at"`
}

// Store keeps the bytes. The Postgres implementation can be replaced by an object store (MinIO)
// without changing the HTTP API.
type Store interface {
	Put(ctx context.Context, s *platform.Scope, tenant *string, m Meta, data []byte) (*Meta, error)
	Get(ctx context.Context, s *platform.Scope, id string) (*Meta, []byte, error)
	List(ctx context.Context, s *platform.Scope, entityType, entityID string) ([]Meta, error)
}

type pgStore struct{}

const metaCols = `id, entity_type, entity_id, filename, content_type, size, sha256, owner, created_at`

func scanMeta(row pgx.Row) (*Meta, error) {
	var m Meta
	err := row.Scan(&m.ID, &m.EntityType, &m.EntityID, &m.Filename, &m.ContentType, &m.Size, &m.SHA256, &m.Owner, &m.CreatedAt)
	return &m, err
}

func (pgStore) Put(ctx context.Context, s *platform.Scope, tenant *string, m Meta, data []byte) (*Meta, error) {
	return scanMeta(s.Tx.QueryRow(ctx, `INSERT INTO files.objects (tenant_id, owner, entity_type, entity_id, filename, content_type, size, sha256, data)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+metaCols, tenant, m.Owner, m.EntityType, m.EntityID, m.Filename, m.ContentType, len(data), m.SHA256, data))
}

func (pgStore) Get(ctx context.Context, s *platform.Scope, id string) (*Meta, []byte, error) {
	var data []byte
	var m Meta
	err := s.Tx.QueryRow(ctx, `SELECT `+metaCols+`, data FROM files.objects WHERE id::text = $1`, id).
		Scan(&m.ID, &m.EntityType, &m.EntityID, &m.Filename, &m.ContentType, &m.Size, &m.SHA256, &m.Owner, &m.CreatedAt, &data)
	if errors.Is(err, pgx.ErrNoRows) || (err != nil && strings.Contains(err.Error(), "invalid input syntax")) {
		return nil, nil, platform.Err(http.StatusNotFound, "Not found", "attachment not found")
	}
	return &m, data, err
}

func (pgStore) List(ctx context.Context, s *platform.Scope, entityType, entityID string) ([]Meta, error) {
	rows, err := s.Tx.Query(ctx, `SELECT `+metaCols+` FROM files.objects WHERE entity_type=$1 AND entity_id::text=$2 ORDER BY created_at, id`, entityType, entityID)
	if err != nil {
		if strings.Contains(err.Error(), "invalid input syntax") {
			return []Meta{}, nil
		}
		return nil, err
	}
	defer rows.Close()
	out := []Meta{}
	for rows.Next() {
		m, err := scanMeta(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

type module struct{ store Store }

func Register(a *platform.App) {
	m := &module{store: pgStore{}}
	a.Route("POST /v1/attachments", platform.Portal, m.upload)
	a.Route("GET /v1/attachments", platform.Portal, m.list)
	a.Route("GET /v1/attachments/{id}", platform.Portal, m.download)
}

// authorise checks the caller may attach to or read the entity, using each owning module's own
// row-level security, and returns the tenant to store with the file.
func authorise(ctx context.Context, s *platform.Scope, entityType, entityID string, writing bool) (*string, error) {
	switch entityType {
	case "ticket":
		t, err := support.TicketTenant(ctx, s, entityID)
		return &t, err
	case "intervention":
		t, err := projects.InterventionTenant(ctx, s, entityID)
		if err == nil && writing && s.Claims.Scope() != "staff" {
			return nil, platform.Err(http.StatusForbidden, "Forbidden", "only staff attach files to site visits")
		}
		return &t, err
	case "expense":
		owner, err := hr.ClaimOwner(ctx, s, entityID)
		if err != nil {
			return nil, err
		}
		if writing && owner != s.Claims.Sub {
			return nil, platform.Err(http.StatusForbidden, "Forbidden", "only the claimant attaches receipts to a claim")
		}
		return nil, nil
	}
	return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid request", "entity_type must be ticket, intervention or expense")
}

func (m *module) upload(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	if s.Claims.Scope() == "other" {
		return nil, platform.Err(http.StatusForbidden, "Forbidden", "your role cannot upload files")
	}
	r.Body = http.MaxBytesReader(nil, r.Body, MaxSize+1<<20) // the file plus form overhead
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return nil, platform.Err(http.StatusRequestEntityTooLarge, "File too large", "attachments are limited to 10 MB")
		}
		return nil, platform.Err(http.StatusBadRequest, "Invalid upload", "send multipart/form-data with entity_type, entity_id and file")
	}
	entityType, entityID := r.FormValue("entity_type"), r.FormValue("entity_id")
	fhs := r.MultipartForm.File["file"]
	if len(fhs) != 1 {
		return nil, platform.Err(http.StatusBadRequest, "Invalid upload", "exactly one file is required")
	}
	f, err := fhs[0].Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Empty file", "the file is empty")
	}
	if len(data) > MaxSize {
		return nil, platform.Err(http.StatusRequestEntityTooLarge, "File too large", "attachments are limited to 10 MB")
	}
	ctype, ok := Detect(data)
	if !ok {
		return nil, platform.Err(http.StatusUnsupportedMediaType, "File type not allowed", "attach a JPEG, PNG or WebP image, or a PDF")
	}
	ctx := r.Context()
	tenant, err := authorise(ctx, s, entityType, entityID, true)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	meta, err := m.store.Put(ctx, s, tenant, Meta{Owner: s.Claims.Sub, EntityType: entityType, EntityID: entityID,
		Filename: SafeName(fhs[0].Filename), ContentType: ctype, SHA256: hex.EncodeToString(sum[:])}, data)
	if err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "upload", "attachment", meta.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: meta}, nil
}

func (m *module) list(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	et, eid := r.URL.Query().Get("entity_type"), r.URL.Query().Get("entity_id")
	if _, err := authorise(r.Context(), s, et, eid, false); err != nil {
		return nil, err
	}
	out, err := m.store.List(r.Context(), s, et, eid)
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, nil
}

func (m *module) download(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	meta, data, err := m.store.Get(r.Context(), s, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	return &platform.Result{
		Status: http.StatusOK, Bytes: data, ContentType: meta.ContentType,
		Header: map[string]string{
			"Content-Disposition":     `attachment; filename="` + meta.Filename + `"`,
			"X-Content-Type-Options":  "nosniff",
			"Content-Security-Policy": "sandbox; default-src 'none'",
			"Cache-Control":           "private, no-store",
		},
	}, nil
}
