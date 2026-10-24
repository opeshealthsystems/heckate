package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/modules/accounting"
	"github.com/opeshealthsystems/heckate/api/internal/modules/auditlog"
	"github.com/opeshealthsystems/heckate/api/internal/modules/contracts"
	"github.com/opeshealthsystems/heckate/api/internal/modules/notify"
	"github.com/opeshealthsystems/heckate/api/internal/modules/sales"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/internal/server"
	"github.com/opeshealthsystems/heckate/api/migrations"
)

// version is stamped at build time: -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cfg := platform.LoadConfig()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "devtoken": // heckate devtoken <role[,role]> [tenant_id]
			devToken(cfg)
			return
		case "verify-data": // heckate verify-data: integrity report for a database (used by backup restore drills)
			os.Exit(verifyData(cfg))
		case "migrate":
			if err := platform.Migrate(context.Background(), cfg.DatabaseURL, migrations.FS); err != nil {
				slog.Error("migrate", "err", err)
				os.Exit(1)
			}
			return
		}
	}

	if problems := cfg.Problems(); len(problems) > 0 {
		for _, m := range problems {
			slog.Error("unsafe configuration", "problem", m)
		}
		slog.Error("refusing to start; fix the settings above, or set HEKATE_ENV=development on a developer machine")
		os.Exit(1)
	}
	if !cfg.Production() {
		slog.Warn("running in DEVELOPMENT mode: weaker authentication and fake payments may be enabled")
	}

	if cfg.Production() && cfg.QRSigningKeyFile == "" {
		slog.Warn("HEKATE_QR_SIGNING_KEY_FILE is not set: document QR codes are plain, unsigned verification links")
	}

	if cfg.Production() && cfg.PDFSigningCertFile == "" {
		slog.Warn("HEKATE_PDF_SIGNING_CERT_FILE is not set: issued PDFs are hash-registered and QR-verifiable but not cryptographically signed")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := platform.Migrate(ctx, cfg.DatabaseURL, migrations.FS); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}
	pool, err := platform.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	var pub platform.Publisher = platform.LogPublisher{}
	if cfg.NATSURL != "" {
		np, err := platform.NewNATSPublisher(ctx, cfg.NATSURL)
		if err != nil {
			slog.Error("nats", "err", err)
			os.Exit(1)
		}
		defer np.Close()
		pub = np
		slog.Info("publishing events to NATS JetStream", "url", cfg.NATSURL)
	}
	if cfg.PushEnabled {
		pub = platform.MultiPublisher{pub, platform.BestEffort{Inner: notify.NewNotifier(pool, notify.NewExpoSender(cfg.ExpoToken))}}
		slog.Info("push notifications enabled")
	}
	handler, hub, metrics := server.NewObserved(cfg, pool, version)
	pub = platform.MultiPublisher{pub, hub} // live events to connected clients, beside the broker
	go platform.RelayOutbox(ctx, pool, pub, 2*time.Second)
	go auditlog.Seal(ctx, pool, 3*time.Second)
	if cfg.RecurringEvery > 0 {
		go sales.RunScheduler(ctx, pool, cfg.RecurringEvery)
	}
	if cfg.RemindersEvery > 0 {
		go sales.RunReminderScheduler(ctx, pool, cfg.RemindersEvery)
		go contracts.RunScheduler(ctx, pool, cfg.RemindersEvery) // contract renewals and warnings follow the same timer
	}

	if cfg.MetricsAddr != "" {
		// Internal only: metrics name every route and show load, so they must not face the internet.
		ms := &http.Server{Addr: cfg.MetricsAddr, Handler: metrics.Handler(), ReadHeaderTimeout: 5 * time.Second}
		go func() {
			<-ctx.Done()
			_ = ms.Close()
		}()
		go func() {
			if err := ms.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("metrics listener", "err", err)
			}
		}()
		slog.Info("metrics on", "addr", cfg.MetricsAddr)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second, // PDFs and downloads are the slowest responses
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	slog.Info("listening", "addr", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// devToken prints a signed development token. Requires HEKATE_DEV_JWT_SECRET.
func devToken(cfg platform.Config) {
	if cfg.DevSecret == "" || len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: HEKATE_DEV_JWT_SECRET=... api devtoken <role[,role]> [tenant_id]")
		os.Exit(2)
	}
	claims := map[string]any{
		"sub": "dev-" + os.Args[2], "email": os.Args[2] + "@dev.local",
		"roles": strings.Split(os.Args[2], ","), "exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	if len(os.Args) > 3 {
		claims["tenant_id"] = os.Args[3]
	}
	fmt.Println(platform.SignDevToken(cfg.DevSecret, claims))
}

// verifyData proves a database is whole: both hash chains recompute, nothing waits to be sealed that
// should not, and every table's row count is printed so a restore can be compared with its source.
// Exit status 0 means intact; the report is JSON on stdout.
func verifyData(cfg platform.Config) int {
	ctx := context.Background()
	pool, err := platform.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		return 2
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.role','staff',true), set_config('app.roles','auditor,ceo',true), set_config('app.actor','verify-data',true), set_config('app.tenant_id','',true)`); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	entries, brokenLedger, err := accounting.VerifyChain(ctx, tx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ledger:", err)
		return 2
	}
	var auditChecked, unsealed int64
	var brokenAudit *int64
	if err := tx.QueryRow(ctx, `SELECT checked, first_broken, unsealed FROM platform.verify_audit()`).Scan(&auditChecked, &brokenAudit, &unsealed); err != nil {
		fmt.Fprintln(os.Stderr, "audit:", err)
		return 2
	}
	// Row counts need to see every row, so they use the owner connection, not the restricted role.
	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		return 2
	}
	defer conn.Close(ctx)
	rows, err := conn.Query(ctx, `SELECT table_schema, table_name FROM information_schema.tables
		WHERE table_type='BASE TABLE' AND table_schema NOT IN ('pg_catalog','information_schema') ORDER BY 1,2`)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	type tn struct{ s, t string }
	var tables []tn
	for rows.Next() {
		var x tn
		_ = rows.Scan(&x.s, &x.t)
		tables = append(tables, x)
	}
	rows.Close()
	counts := map[string]int64{}
	for _, x := range tables {
		var n int64
		if err := conn.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.%s`, pgx.Identifier{x.s}.Sanitize(), pgx.Identifier{x.t}.Sanitize())).Scan(&n); err != nil {
			fmt.Fprintln(os.Stderr, x.s+"."+x.t+":", err)
			return 2
		}
		counts[x.s+"."+x.t] = n
	}
	intact := brokenLedger == 0 && brokenAudit == nil
	out, _ := json.MarshalIndent(map[string]any{
		"intact": intact, "ledger_entries": entries, "ledger_first_broken": brokenLedger,
		"audit_rows_checked": auditChecked, "audit_first_broken": brokenAudit, "audit_unsealed": unsealed, "row_counts": counts,
	}, "", "  ")
	fmt.Println(string(out))
	if !intact {
		return 1
	}
	return 0
}
