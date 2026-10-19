package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/modules/auditlog"
	"github.com/opeshealthsystems/heckate/api/internal/modules/notify"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/internal/server"
	"github.com/opeshealthsystems/heckate/api/migrations"
)

func main() {
	cfg := platform.LoadConfig()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "devtoken": // heckate devtoken <role[,role]> [tenant_id]
			devToken(cfg)
			return
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
	handler, hub := server.NewWithHub(cfg, pool)
	pub = platform.MultiPublisher{pub, hub} // live events to connected clients, beside the broker
	go platform.RelayOutbox(ctx, pool, pub, 2*time.Second)
	go auditlog.Seal(ctx, pool, 3*time.Second)

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
