package platform

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

type ctxKey struct{}

// TraceID returns the trace ID attached to ctx, or "".
func TraceID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// validTraceID accepts only what a tracing system would send: 8 to 64 letters, digits, dashes or
// underscores. Anything else is replaced, because the value ends up in logs, headers and the audit trail.
func validTraceID(s string) bool {
	if len(s) < 8 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// Trace ensures every request carries a trace ID (X-Trace-Id) and logs it. A caller's trace ID is kept
// only if it is well formed. The log line names the matched route, not the raw URL, so nothing the
// caller typed is written to the log except the validated ID.
func Trace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Trace-Id")
		if !validTraceID(id) {
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Trace-Id", id)
		start := time.Now()
		r2 := r.WithContext(context.WithValue(r.Context(), ctxKey{}, id))
		next.ServeHTTP(w, r2)
		slog.Info("request", "trace_id", id, "route", r2.Pattern, "dur", time.Since(start))
	})
}
