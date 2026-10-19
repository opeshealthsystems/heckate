// Package server assembles the HTTP handler from the platform layer and every module.
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
	"github.com/opeshealthsystems/heckate/api/internal/modules/accounting"
	"github.com/opeshealthsystems/heckate/api/internal/modules/auditlog"
	"github.com/opeshealthsystems/heckate/api/internal/modules/clients"
	"github.com/opeshealthsystems/heckate/api/internal/modules/contracts"
	"github.com/opeshealthsystems/heckate/api/internal/modules/crm"
	"github.com/opeshealthsystems/heckate/api/internal/modules/devapi"
	"github.com/opeshealthsystems/heckate/api/internal/modules/documents"
	"github.com/opeshealthsystems/heckate/api/internal/modules/files"
	"github.com/opeshealthsystems/heckate/api/internal/modules/forms"
	"github.com/opeshealthsystems/heckate/api/internal/modules/hr"
	"github.com/opeshealthsystems/heckate/api/internal/modules/inventory"
	"github.com/opeshealthsystems/heckate/api/internal/modules/narratives"
	"github.com/opeshealthsystems/heckate/api/internal/modules/notify"
	"github.com/opeshealthsystems/heckate/api/internal/modules/payments"
	"github.com/opeshealthsystems/heckate/api/internal/modules/payroll"
	"github.com/opeshealthsystems/heckate/api/internal/modules/procurement"
	"github.com/opeshealthsystems/heckate/api/internal/modules/projects"
	"github.com/opeshealthsystems/heckate/api/internal/modules/sales"
	"github.com/opeshealthsystems/heckate/api/internal/modules/stream"
	"github.com/opeshealthsystems/heckate/api/internal/modules/support"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// New returns the full API handler.
func New(cfg platform.Config, pool *pgxpool.Pool) http.Handler {
	h, _ := NewWithHub(cfg, pool)
	return h
}

// NewWithHub also returns the live-event hub, which the outbox relay must feed.
func NewWithHub(cfg platform.Config, pool *pgxpool.Pool) (http.Handler, *stream.Hub) {
	h, hub, _ := NewObserved(cfg, pool, "dev")
	return h, hub
}

// NewObserved is NewWithHub plus the metrics registry, which main serves on an internal listener.
func NewObserved(cfg platform.Config, pool *pgxpool.Pool, version string) (http.Handler, *stream.Hub, *platform.Metrics) {
	hub := stream.NewHub()
	app := build(cfg, pool, hub)
	m := platform.NewMetrics(version, pool)
	return platform.Trace(m.Middleware(platform.SecureHeaders(platform.CORS(cfg.CORSOrigins, app.Mux)))), hub, m
}

// Patterns lists every route the server registers ("METHOD /path"), without needing a database.
func Patterns() []string { return build(platform.Config{}, nil, stream.NewHub()).Patterns }

func build(cfg platform.Config, pool *pgxpool.Pool, hub *stream.Hub) *platform.App {
	app := &platform.App{
		Pool:     pool,
		Verifier: platform.NewVerifier(cfg),
		Limiter:  platform.NewLimiter(rateLimit(cfg), time.Minute),
		Mux:      http.NewServeMux(),
		Proxies:  platform.ParseProxies(cfg.TrustedProxies),
	}
	// Liveness: the process answers. Never touches the database, so a database outage does not make an
	// orchestrator restart healthy application servers.
	app.Patterns = append(app.Patterns, "GET /livez")
	app.Mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		platform.WriteJSON(w, http.StatusOK, map[string]string{"status": "alive"})
	})
	// Readiness: safe to send traffic (the database answers within two seconds). /healthz is kept as an alias.
	ready := func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			platform.WriteProblem(w, r, http.StatusServiceUnavailable, "Database unavailable", "")
			return
		}
		platform.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
	app.Patterns = append(app.Patterns, "GET /readyz", "GET /healthz")
	app.Mux.HandleFunc("GET /readyz", ready)
	app.Mux.HandleFunc("GET /healthz", ready)
	clients.Register(app)
	crm.Register(app)
	support.Register(app)
	documents.Register(app, cfg.PortalURL)
	notify.Register(app)
	devapi.Register(app)
	auditlog.Register(app)
	docOpts := docs.Options{PortalURL: cfg.PortalURL, Letterhead: cfg.Letterhead, Compress: cfg.PDFCompress}
	if signer, err := docs.LoadQRSigner(cfg.QRSigningKeyFile); err != nil {
		panic("HEKATE_QR_SIGNING_KEY_FILE: " + err.Error()) // a configured but unusable key must stop start-up, not quietly print unsigned codes
	} else {
		docOpts.QRSigner = signer
	}
	projects.Register(app, docOpts)
	hr.Register(app)
	contracts.Register(app, docOpts)
	forms.Register(app)
	files.Register(app)
	inventory.Register(app, docOpts)
	narratives.Register(app, docOpts)
	payroll.Register(app, accounting.PayrollPoster{}, docOpts)
	procurement.Register(app, accounting.ProcurementPoster{}, docOpts)
	payments.Register(app, sales.Register(app, accounting.SalesPoster{}, docOpts), cfg)
	accounting.Register(app, docOpts)
	stream.Register(app, hub, cfg.CORSOrigins)
	return app
}

func rateLimit(cfg platform.Config) int {
	if cfg.RateLimit > 0 {
		return cfg.RateLimit
	}
	return 300
}
