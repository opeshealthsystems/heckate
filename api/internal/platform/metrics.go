package platform

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Metrics is a small Prometheus-compatible registry (text exposition format) so the platform needs no
// extra dependency to be observable. Labels are bounded: routes are the registered patterns, never raw URLs.
type Metrics struct {
	mu       sync.Mutex
	reqs     map[string]uint64
	buckets  map[string][]uint64 // route -> cumulative-ready bucket counts (one extra slot for +Inf)
	sums     map[string]float64
	counts   map[string]uint64
	inflight atomic.Int64
	start    time.Time
	Version  string
	pool     *pgxpool.Pool
}

var latencyBounds = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

func NewMetrics(version string, pool *pgxpool.Pool) *Metrics {
	return &Metrics{reqs: map[string]uint64{}, buckets: map[string][]uint64{}, sums: map[string]float64{}, counts: map[string]uint64{}, start: time.Now(), Version: version, pool: pool}
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(c int) {
	if w.code == 0 {
		w.code = c
	}
	w.ResponseWriter.WriteHeader(c)
}
func (w *statusWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer cannot be hijacked")
	}
	if w.code == 0 {
		w.code = http.StatusSwitchingProtocols
	}
	return h.Hijack()
}

// Middleware counts and times every request. It must sit inside Trace (which keeps the request pointer
// stable) so the matched route pattern can be read after the mux has run.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.inflight.Add(1)
		defer m.inflight.Add(-1)
		sw := &statusWriter{ResponseWriter: w}
		start := time.Now()
		next.ServeHTTP(sw, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		code := sw.code
		if code == 0 {
			code = http.StatusOK
		}
		m.observe(route, r.Method, code, time.Since(start).Seconds())
	})
}

func (m *Metrics) observe(route, method string, code int, secs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reqs[fmt.Sprintf("%s\x00%s\x00%dxx", route, method, code/100)]++
	b := m.buckets[route]
	if b == nil {
		b = make([]uint64, len(latencyBounds)+1)
		m.buckets[route] = b
	}
	i := sort.SearchFloat64s(latencyBounds, secs)
	b[i]++
	m.sums[route] += secs
	m.counts[route]++
}

// Handler serves the metrics. Run it on an internal-only listener.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		var sb strings.Builder
		m.mu.Lock()
		keys := make([]string, 0, len(m.reqs))
		for k := range m.reqs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sb.WriteString("# HELP hekate_http_requests_total HTTP requests by route, method and status class.\n# TYPE hekate_http_requests_total counter\n")
		for _, k := range keys {
			p := strings.Split(k, "\x00")
			fmt.Fprintf(&sb, "hekate_http_requests_total{route=%q,method=%q,class=%q} %d\n", p[0], p[1], p[2], m.reqs[k])
		}
		routes := make([]string, 0, len(m.buckets))
		for k := range m.buckets {
			routes = append(routes, k)
		}
		sort.Strings(routes)
		sb.WriteString("# HELP hekate_http_request_duration_seconds Request latency by route.\n# TYPE hekate_http_request_duration_seconds histogram\n")
		for _, route := range routes {
			var cum uint64
			for i, b := range m.buckets[route] {
				cum += b
				le := "+Inf"
				if i < len(latencyBounds) {
					le = fmt.Sprint(latencyBounds[i])
				}
				fmt.Fprintf(&sb, "hekate_http_request_duration_seconds_bucket{route=%q,le=%q} %d\n", route, le, cum)
			}
			fmt.Fprintf(&sb, "hekate_http_request_duration_seconds_sum{route=%q} %g\nhekate_http_request_duration_seconds_count{route=%q} %d\n", route, m.sums[route], route, m.counts[route])
		}
		m.mu.Unlock()

		fmt.Fprintf(&sb, "# TYPE hekate_http_requests_in_flight gauge\nhekate_http_requests_in_flight %d\n", m.inflight.Load())
		fmt.Fprintf(&sb, "# TYPE hekate_uptime_seconds gauge\nhekate_uptime_seconds %d\n", int(time.Since(m.start).Seconds()))
		fmt.Fprintf(&sb, "# TYPE hekate_build_info gauge\nhekate_build_info{version=%q,go=%q} 1\n", m.Version, runtime.Version())
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		fmt.Fprintf(&sb, "# TYPE hekate_go_goroutines gauge\nhekate_go_goroutines %d\n# TYPE hekate_go_heap_bytes gauge\nhekate_go_heap_bytes %d\n", runtime.NumGoroutine(), ms.HeapAlloc)
		if m.pool != nil {
			st := m.pool.Stat()
			sb.WriteString("# TYPE hekate_db_pool_connections gauge\n")
			fmt.Fprintf(&sb, "hekate_db_pool_connections{state=\"acquired\"} %d\nhekate_db_pool_connections{state=\"idle\"} %d\nhekate_db_pool_connections{state=\"max\"} %d\n", st.AcquiredConns(), st.IdleConns(), st.MaxConns())
			fmt.Fprintf(&sb, "# TYPE hekate_db_pool_empty_acquires_total counter\nhekate_db_pool_empty_acquires_total %d\n", st.EmptyAcquireCount())
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			var pending, oldest, unsealed int64
			if err := m.pool.QueryRow(ctx, `SELECT count(*), COALESCE(EXTRACT(EPOCH FROM now() - min(created_at))::bigint, 0) FROM platform.outbox WHERE published_at IS NULL`).Scan(&pending, &oldest); err == nil {
				fmt.Fprintf(&sb, "# HELP hekate_outbox_pending Events written but not yet published (should hover near 0).\n# TYPE hekate_outbox_pending gauge\nhekate_outbox_pending %d\n# TYPE hekate_outbox_oldest_pending_seconds gauge\nhekate_outbox_oldest_pending_seconds %d\n", pending, oldest)
			}
			if err := m.pool.QueryRow(ctx, `SELECT unsealed FROM platform.verify_audit_backlog()`).Scan(&unsealed); err == nil {
				fmt.Fprintf(&sb, "# HELP hekate_audit_unsealed Audit rows waiting to be linked into the hash chain.\n# TYPE hekate_audit_unsealed gauge\nhekate_audit_unsealed %d\n", unsealed)
			}
			fmt.Fprintf(&sb, "# TYPE hekate_db_up gauge\nhekate_db_up %d\n", boolInt(m.pool.Ping(ctx) == nil))
		}
		_, _ = w.Write([]byte(sb.String()))
	})
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
