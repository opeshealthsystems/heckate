package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/modules/support"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		platform.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	support.Register(mux)

	addr := os.Getenv("HEKATE_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           platform.Trace(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
