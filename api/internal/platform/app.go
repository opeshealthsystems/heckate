package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HTTPError is returned by handlers to produce a Problem response.
type HTTPError struct {
	Status int
	Title  string
	Detail string
}

func (e *HTTPError) Error() string { return e.Title + ": " + e.Detail }

func Err(status int, title, detail string) error { return &HTTPError{status, title, detail} }

// Scope is the per-request database transaction with tenant/role settings applied for row-level security.
type Scope struct {
	Tx      pgx.Tx
	Claims  *Claims
	TraceID string
}

// Audit appends to the audit log inside the request transaction.
func (s *Scope) Audit(ctx context.Context, action, entity, entityID string) error {
	var tenant any
	if s.Claims != nil && s.Claims.TenantID != "" {
		tenant = s.Claims.TenantID
	}
	actor := "anonymous"
	if s.Claims != nil {
		actor = s.Claims.Sub
	}
	_, err := s.Tx.Exec(ctx, `INSERT INTO platform.audit_log (tenant_id, actor, action, entity, entity_id, trace_id) VALUES ($1,$2,$3,$4,$5,$6)`,
		tenant, actor, action, entity, entityID, s.TraceID)
	return err
}

// Emit writes a domain event to the transactional outbox.
func (s *Scope) Emit(ctx context.Context, subject string, payload any) error {
	b, _ := json.Marshal(payload)
	_, err := s.Tx.Exec(ctx, `INSERT INTO platform.outbox (subject, payload) VALUES ($1,$2)`, subject, b)
	return err
}

// Result is what a handler returns; the transaction commits before it is written.
type Result struct {
	Status int
	Body   any
	Header map[string]string
	// Bytes, when set, is written as-is with ContentType instead of Body as JSON (file downloads).
	Bytes       []byte
	ContentType string
}

type HandlerFunc func(r *http.Request, s *Scope) (*Result, error)

type App struct {
	Pool     *pgxpool.Pool
	Verifier *Verifier
	Limiter  *Limiter
	Mux      *http.ServeMux
	Patterns []string // every registered "METHOD /path", used to check the code against the OpenAPI contract
}

// NewPool connects with the owner credentials, then drops to hekate_app so RLS applies.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, err := c.Exec(ctx, "SET ROLE hekate_app")
		return err
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

func (a *App) begin(ctx context.Context, c *Claims, trace string) (*Scope, error) {
	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	var tenant, role, actor, roles string
	if c != nil {
		tenant, role, actor, roles = c.TenantID, c.Scope(), c.Sub, strings.Join(c.Roles, ",")
	} else {
		role = "public"
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true), set_config('app.role',$2,true), set_config('app.actor',$3,true), set_config('app.roles',$4,true)`, tenant, role, actor, roles); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return &Scope{Tx: tx, Claims: c, TraceID: TraceID(ctx)}, nil
}

// Route registers an authenticated route. roles is an allow-list (empty = any authenticated user).
func (a *App) Route(pattern string, roles []string, h HandlerFunc) {
	a.Patterns = append(a.Patterns, pattern)
	a.Mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			w.Header().Set("WWW-Authenticate", "Bearer")
			WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "Bearer token required")
			return
		}
		claims, err := a.Verifier.Verify(tok)
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "Invalid or expired token")
			return
		}
		if len(roles) > 0 && !claims.HasAny(roles...) {
			WriteProblem(w, r, http.StatusForbidden, "Forbidden", "Your role cannot perform this action")
			return
		}
		if !a.limit(w, r, claims.Sub) {
			return
		}
		a.run(w, r, claims, h)
	})
}

// Public registers an unauthenticated route (rate limited per client IP).
func (a *App) Public(pattern string, h HandlerFunc) {
	a.Patterns = append(a.Patterns, pattern)
	a.Mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if !a.limit(w, r, clientIP(r)) {
			return
		}
		a.run(w, r, nil, h)
	})
}

func (a *App) limit(w http.ResponseWriter, r *http.Request, key string) bool {
	ok, limit, remaining, reset := a.Limiter.Allow(key)
	w.Header().Set("RateLimit-Limit", strconv.Itoa(limit))
	w.Header().Set("RateLimit-Remaining", strconv.Itoa(remaining))
	w.Header().Set("RateLimit-Reset", strconv.Itoa(reset))
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(reset))
		WriteProblem(w, r, http.StatusTooManyRequests, "Too many requests", "Rate limit exceeded")
	}
	return ok
}

func clientIP(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		return strings.TrimSpace(strings.Split(f, ",")[0])
	}
	return r.RemoteAddr
}

func (a *App) run(w http.ResponseWriter, r *http.Request, claims *Claims, h HandlerFunc) {
	ctx := r.Context()
	s, err := a.begin(ctx, claims, TraceID(ctx))
	if err != nil {
		slog.Error("begin", "err", err)
		WriteProblem(w, r, http.StatusServiceUnavailable, "Service unavailable", "")
		return
	}
	res, err := h(r, s)
	if err != nil {
		_ = s.Tx.Rollback(ctx)
		var he *HTTPError
		if errors.As(err, &he) {
			WriteProblem(w, r, he.Status, he.Title, he.Detail)
			return
		}
		slog.Error("handler", "trace_id", TraceID(ctx), "err", err)
		WriteProblem(w, r, http.StatusInternalServerError, "Internal error", "")
		return
	}
	if err := s.Tx.Commit(ctx); err != nil {
		// Deferred database constraints (balanced ledger entries, non-negative stock) fail at commit.
		switch msg := err.Error(); {
		case strings.Contains(msg, "insufficient stock"):
			WriteProblem(w, r, http.StatusConflict, "Insufficient stock", "not enough stock for this movement")
		case strings.Contains(msg, "is not balanced"):
			WriteProblem(w, r, http.StatusUnprocessableEntity, "Entry does not balance", "debits and credits must be equal")
		default:
			slog.Error("commit", "err", err)
			WriteProblem(w, r, http.StatusInternalServerError, "Internal error", "")
		}
		return
	}
	for k, v := range res.Header {
		w.Header().Set(k, v)
	}
	if res.Bytes != nil {
		w.Header().Set("Content-Type", res.ContentType)
		w.WriteHeader(res.Status)
		_, _ = w.Write(res.Bytes)
		return
	}
	if res.Body == nil {
		w.WriteHeader(res.Status)
		return
	}
	WriteJSON(w, res.Status, res.Body)
}

// Idempotent wraps a POST handler: a repeated Idempotency-Key replays the stored response.
func Idempotent(h HandlerFunc) HandlerFunc {
	return func(r *http.Request, s *Scope) (*Result, error) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			return nil, Err(http.StatusBadRequest, "Missing header", "Idempotency-Key is required on POST")
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return nil, Err(http.StatusBadRequest, "Bad request", "unreadable body")
		}
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		sum := sha256.Sum256(body)
		hash := hex.EncodeToString(sum[:])
		actor := "anonymous"
		if s.Claims != nil {
			actor = s.Claims.Sub
		}
		var status int
		var stored []byte
		var prevHash string
		err = s.Tx.QueryRow(r.Context(), `SELECT status, body, request_hash FROM platform.idempotency_keys WHERE key=$1 AND actor=$2`, key, actor).Scan(&status, &stored, &prevHash)
		if err == nil {
			if prevHash != hash {
				return nil, Err(http.StatusUnprocessableEntity, "Idempotency conflict", "key reused with a different request")
			}
			var v any
			_ = json.Unmarshal(stored, &v)
			return &Result{Status: status, Body: v, Header: map[string]string{"Idempotent-Replay": "true"}}, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		res, err := h(r, s)
		if err != nil {
			return nil, err
		}
		b, _ := json.Marshal(res.Body)
		if _, err := s.Tx.Exec(r.Context(), `INSERT INTO platform.idempotency_keys (key, actor, request_hash, status, body) VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, key, actor, hash, res.Status, b); err != nil {
			return nil, err
		}
		return res, nil
	}
}

// DecodeBody parses a JSON request body strictly.
func DecodeBody(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return Err(http.StatusBadRequest, "Invalid body", err.Error())
	}
	return nil
}

// Limiter is a fixed-window in-memory rate limiter.
type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]*bucket
}

type bucket struct {
	n     int
	start time.Time
}

func NewLimiter(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, hits: map[string]*bucket{}}
}

func (l *Limiter) Allow(key string) (ok bool, limit, remaining, resetSecs int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b := l.hits[key]
	if b == nil || now.Sub(b.start) >= l.window {
		if len(l.hits) > 10000 {
			l.hits = map[string]*bucket{}
		}
		b = &bucket{start: now}
		l.hits[key] = b
	}
	b.n++
	reset := int(l.window.Seconds() - now.Sub(b.start).Seconds())
	rem := l.limit - b.n
	if rem < 0 {
		rem = 0
	}
	return b.n <= l.limit, l.limit, rem, reset
}

// CORS allows the configured browser origins.
func CORS(origins string, next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			allowed[o] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" && allowed[o] {
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, If-Match")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Expose-Headers", "ETag, RateLimit-Limit, RateLimit-Remaining, RateLimit-Reset, X-Trace-Id")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
