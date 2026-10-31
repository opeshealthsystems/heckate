// Package stream pushes live events (ticket conversation, site visits) to connected web and mobile
// clients over WebSockets. It is a hint to refresh: the data itself is always fetched through the API,
// so row-level security still decides what anyone can read.
package stream

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

const (
	subprotocol  = "hekate.v1"
	bearerPrefix = "bearer."
	maxPerUser   = 5
	queueSize    = 64
)

// Allowed decides whether a connected user may be told about an event. Clients only hear about
// their own tenant. Management, support and engineers hear about every ticket and site visit; other staff
// hear only about tickets they raised themselves. This mirrors what row-level security lets each role read.
func Allowed(c *platform.Claims, subject string, payload map[string]any) bool {
	domain, _, _ := strings.Cut(subject, ".")
	if domain != "ticket" && domain != "intervention" {
		return false
	}
	switch c.Scope() {
	case "staff":
		if c.HasAny("ceo", "manager", "support", "engineer") {
			return true
		}
		reporter, _ := payload["reporter"].(string)
		return domain == "ticket" && reporter != "" && reporter == c.Sub
	case "client":
		tenant, _ := payload["tenant_id"].(string)
		return tenant != "" && tenant == c.TenantID
	}
	return false
}

type client struct {
	claims *platform.Claims
	out    chan []byte
}

// Hub fans events out to connected clients. It implements platform.Publisher so it sits beside the
// broker publisher in the outbox relay.
type Hub struct {
	mu    sync.Mutex
	conns map[*client]struct{}
}

func NewHub() *Hub { return &Hub{conns: map[*client]struct{}{}} }

func (h *Hub) Publish(_ context.Context, subject string, payload []byte, _ string) error {
	var p map[string]any
	if json.Unmarshal(payload, &p) != nil {
		return nil
	}
	msg, _ := json.Marshal(map[string]any{"subject": subject, "data": p})
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.conns {
		if !Allowed(c.claims, subject, p) {
			continue
		}
		select {
		case c.out <- msg:
		default: // a client that cannot keep up is dropped rather than slowing everyone down
			close(c.out)
			delete(h.conns, c)
		}
	}
	return nil
}

func (h *Hub) add(c *client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for o := range h.conns {
		if o.claims.Sub == c.claims.Sub {
			n++
		}
	}
	if n >= maxPerUser {
		return false
	}
	h.conns[c] = struct{}{}
	return true
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.conns[c]; ok {
		delete(h.conns, c)
		close(c.out)
	}
}

// Count is the number of open connections (used by tests and health checks).
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.conns)
}

// tokenFromProtocols finds "bearer.<token>" among the requested subprotocols. Browsers cannot set an
// Authorization header on a WebSocket, and a query string would end up in logs.
func tokenFromProtocols(r *http.Request) string {
	for _, line := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, p := range strings.Split(line, ",") {
			if t, ok := strings.CutPrefix(strings.TrimSpace(p), bearerPrefix); ok {
				return t
			}
		}
	}
	return ""
}

// Register mounts GET /v1/stream. origins are the browser origins allowed to connect (the CORS list).
func Register(a *platform.App, h *Hub, origins string) {
	var patterns []string
	for _, o := range strings.Split(origins, ",") {
		if u, err := url.Parse(strings.TrimSpace(o)); err == nil && u.Host != "" {
			patterns = append(patterns, u.Host)
		}
	}
	a.Handle("GET /v1/stream", func(w http.ResponseWriter, r *http.Request) {
		tok := tokenFromProtocols(r)
		claims, err := a.Verifier.Verify(tok)
		if tok == "" || err != nil {
			platform.WriteProblem(w, r, http.StatusUnauthorized, "Unauthorized", "Pass a valid token as the bearer.<token> subprotocol")
			return
		}
		if !a.Limit(w, r, claims.Sub) {
			return
		}
		c := &client{claims: claims, out: make(chan []byte, queueSize)}
		if !h.add(c) {
			platform.WriteProblem(w, r, http.StatusTooManyRequests, "Too many connections", "close another connection first")
			return
		}
		defer h.remove(c)
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{subprotocol}, OriginPatterns: patterns})
		if err != nil {
			slog.Debug("websocket accept", "err", err)
			return
		}
		defer conn.CloseNow()
		// The connection ends when the token does, so a revoked or expired session cannot linger.
		ctx, cancel := context.WithDeadline(r.Context(), claims.Exp)
		defer cancel()
		ctx = conn.CloseRead(ctx) // we never read client messages; this also notices disconnects
		ping := time.NewTicker(30 * time.Second)
		defer ping.Stop()
		for {
			select {
			case <-ctx.Done():
				conn.Close(websocket.StatusNormalClosure, "session ended")
				return
			case m, ok := <-c.out:
				if !ok {
					conn.Close(websocket.StatusPolicyViolation, "too slow")
					return
				}
				wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
				err := conn.Write(wctx, websocket.MessageText, m)
				wcancel()
				if err != nil {
					return
				}
			case <-ping.C:
				pctx, pcancel := context.WithTimeout(ctx, 5*time.Second)
				err := conn.Ping(pctx)
				pcancel()
				if err != nil {
					return
				}
			}
		}
	})
}
