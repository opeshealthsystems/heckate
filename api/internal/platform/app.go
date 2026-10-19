package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
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

// AsSystem runs fn with the transaction acting as a trusted internal actor (staff, finance) instead of
// the caller, then restores the caller. It is for steps whose authority comes from something other
// than the caller's token, such as a verified payment-provider callback, and must be called only after
// that verification. The audit log and ledger record the actor name given here.
func (s *Scope) AsSystem(ctx context.Context, actor string, fn func() error) error {
	prev := s.Claims
	var tenant, role, who, roles string
	if err := s.Tx.QueryRow(ctx, `SELECT current_setting('app.tenant_id',true), current_setting('app.role',true), current_setting('app.actor',true), current_setting('app.roles',true)`).Scan(&tenant, &role, &who, &roles); err != nil {
		return err
	}
	if _, err := s.Tx.Exec(ctx, `SELECT set_config('app.role','staff',true), set_config('app.actor',$1,true), set_config('app.roles','finance',true)`, actor); err != nil {
		return err
	}
	s.Claims = &Claims{Sub: actor, Roles: []string{"finance"}}
	err := fn()
	if err == nil {
		// Deferred checks (a balanced ledger entry, non-negative stock) read tables under row-level
		// security, so they must run now, while the privileges that wrote the rows still apply.
		if _, err = s.Tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err == nil {
			_, err = s.Tx.Exec(ctx, `SET CONSTRAINTS ALL DEFERRED`)
		}
		err = deferredCheckError(err)
	}
	s.Claims = prev
	if _, e2 := s.Tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true), set_config('app.role',$2,true), set_config('app.actor',$3,true), set_config('app.roles',$4,true)`, tenant, role, who, roles); e2 != nil && err == nil {
		err = e2
	}
	return err
}

// deferredCheckError turns a failed deferred-constraint check into a problem response; other errors pass through.
func deferredCheckError(err error) error {
	if err == nil {
		return nil
	}
	switch msg := err.Error(); {
	case strings.Contains(msg, "insufficient stock"):
		return Err(http.StatusConflict, "Insufficient stock", "not enough stock for this movement")
	case strings.Contains(msg, "is not balanced"):
		return Err(http.StatusUnprocessableEntity, "Entry does not balance", "debits and credits must be equal")
	}
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
	keyLim   keyLimiters
	Proxies  []*net.IPNet // load balancers whose X-Forwarded-For is believed
	Patterns []string     // every registered "METHOD /path", used to check the code against the OpenAPI contract
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
	// pgx would otherwise default to max(4, CPUs) connections, which throttles a busy API to a trickle.
	// Set pool_max_conns in the URL, or HEKATE_DB_MAX_CONNS, to override the default of 25 per instance;
	// instances x this value must stay below PostgreSQL's max_connections.
	if !strings.Contains(dsn, "pool_max_conns") {
		cfg.MaxConns = int32(atoiOr(os.Getenv("HEKATE_DB_MAX_CONNS"), 25))
	}
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
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
		if strings.HasPrefix(tok, KeyPrefix) {
			a.runKey(w, r, pattern, tok, roles, h)
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

// runKey authenticates an API key, enforces its scopes, plan rate limit and monthly quota, and meters the call.
func (a *App) runKey(w http.ResponseWriter, r *http.Request, pattern, tok string, roles []string, h HandlerFunc) {
	ctx := r.Context()
	var k APIKey
	var tenant string
	err := a.Pool.QueryRow(ctx, `SELECT key_id::text, tenant_id::text, scopes, rate_per_min, monthly_quota FROM devapi.authenticate($1)`, HashKey(tok)).
		Scan(&k.ID, &tenant, &k.Scopes, &k.Rate, &k.Quota)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "Invalid, expired or revoked API key")
		return
	}
	if !KeyAllows(k.Scopes, pattern) || len(roles) > 0 {
		WriteProblem(w, r, http.StatusForbidden, "Forbidden", "This API key cannot call this endpoint")
		return
	}
	if !a.keyLim.get(k.Rate).allowWrite(w, r, k.ID) {
		return
	}
	var calls int64
	if err := a.Pool.QueryRow(ctx, `SELECT devapi.meter($1::uuid)`, k.ID).Scan(&calls); err != nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, "Service unavailable", "")
		return
	}
	if calls > k.Quota {
		w.Header().Set("Retry-After", "86400")
		WriteProblem(w, r, http.StatusTooManyRequests, "Quota exceeded", "Monthly API quota for this key is used up")
		return
	}
	claims := &Claims{Sub: "apikey:" + k.ID, TenantID: tenant, Roles: []string{"client_user"}, APIKey: &k}
	a.run(w, r, claims, h)
}

// Public registers an unauthenticated route (rate limited per client IP).
func (a *App) Public(pattern string, h HandlerFunc) {
	a.Patterns = append(a.Patterns, pattern)
	a.Mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if !a.limit(w, r, a.ClientIP(r)) {
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

func (l *Limiter) allowWrite(w http.ResponseWriter, r *http.Request, key string) bool {
	ok, limit, remaining, reset := l.Allow(key)
	w.Header().Set("RateLimit-Limit", strconv.Itoa(limit))
	w.Header().Set("RateLimit-Remaining", strconv.Itoa(remaining))
	w.Header().Set("RateLimit-Reset", strconv.Itoa(reset))
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(reset))
		WriteProblem(w, r, http.StatusTooManyRequests, "Too many requests", "Rate limit exceeded")
	}
	return ok
}

// Limit applies the shared rate limit for a key and writes the standard headers; false means the
// request was rejected with 429 and the caller must stop.
func (a *App) Limit(w http.ResponseWriter, r *http.Request, key string) bool {
	return a.limit(w, r, key)
}

// Handle registers a raw handler (used for WebSocket upgrades, which need the ResponseWriter itself).
func (a *App) Handle(pattern string, h http.HandlerFunc) {
	a.Patterns = append(a.Patterns, pattern)
	a.Mux.HandleFunc(pattern, h)
}

// ClientIP is the address of the real caller. X-Forwarded-For is believed only when the request came
// from a configured trusted proxy, and then the right-most entry that is not itself a trusted proxy
// is used: anything to its left was supplied by the caller and can be forged.
func (a *App) ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if !a.trusted(host) {
		return host
	}
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if net.ParseIP(ip) == nil {
			return host // malformed: fall back to the proxy itself rather than trust it
		}
		if !a.trusted(ip) {
			return ip
		}
	}
	return host
}

func (a *App) trusted(ip string) bool {
	p := net.ParseIP(ip)
	for _, n := range a.Proxies {
		if p != nil && n.Contains(p) {
			return true
		}
	}
	return false
}

// ParseProxies turns CIDR strings into networks; invalid entries are reported by Config.Problems.
func ParseProxies(cidrs []string) []*net.IPNet {
	var out []*net.IPNet
	for _, c := range cidrs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// SecureHeaders sets the response headers an API should always send. Handlers that serve files override
// Cache-Control and Content-Type themselves.
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "no-store")
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		h.Set("Cross-Origin-Resource-Policy", "same-site")
		next.ServeHTTP(w, r)
	})
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
		var he *HTTPError
		if e := deferredCheckError(err); errors.As(e, &he) {
			WriteProblem(w, r, he.Status, he.Title, he.Detail)
		} else {
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
