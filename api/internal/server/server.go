// Package server assembles the HTTP handler from the platform layer and every module.
package server

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opeshealthsystems/heckate/api/internal/modules/accounting"
	"github.com/opeshealthsystems/heckate/api/internal/modules/clients"
	"github.com/opeshealthsystems/heckate/api/internal/modules/crm"
	"github.com/opeshealthsystems/heckate/api/internal/modules/documents"
	"github.com/opeshealthsystems/heckate/api/internal/modules/hr"
	"github.com/opeshealthsystems/heckate/api/internal/modules/inventory"
	"github.com/opeshealthsystems/heckate/api/internal/modules/notify"
	"github.com/opeshealthsystems/heckate/api/internal/modules/procurement"
	"github.com/opeshealthsystems/heckate/api/internal/modules/projects"
	"github.com/opeshealthsystems/heckate/api/internal/modules/sales"
	"github.com/opeshealthsystems/heckate/api/internal/modules/support"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// New returns the full API handler.
func New(cfg platform.Config, pool *pgxpool.Pool) http.Handler {
	app := build(cfg, pool)
	return platform.Trace(platform.CORS(cfg.CORSOrigins, app.Mux))
}

// Patterns lists every route the server registers ("METHOD /path"), without needing a database.
func Patterns() []string { return build(platform.Config{}, nil).Patterns }

func build(cfg platform.Config, pool *pgxpool.Pool) *platform.App {
	app := &platform.App{
		Pool:     pool,
		Verifier: platform.NewVerifier(cfg),
		Limiter:  platform.NewLimiter(300, time.Minute),
		Mux:      http.NewServeMux(),
	}
	app.Patterns = append(app.Patterns, "GET /healthz")
	app.Mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			platform.WriteProblem(w, r, http.StatusServiceUnavailable, "Database unavailable", "")
			return
		}
		platform.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	clients.Register(app)
	crm.Register(app)
	support.Register(app)
	documents.Register(app)
	notify.Register(app)
	projects.Register(app)
	hr.Register(app)
	inventory.Register(app)
	procurement.Register(app, accounting.ProcurementPoster{})
	sales.Register(app, accounting.SalesPoster{})
	accounting.Register(app)
	return app
}
