// Package support is the Support & Incidents bounded context.
// It owns the "support" schema; other modules talk to it through events or this API only.
package support

import (
	"net/http"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// Severity is the incident severity, P1 (critical) to P4 (low).
type Severity string

const (
	P1 Severity = "P1"
	P2 Severity = "P2"
	P3 Severity = "P3"
	P4 Severity = "P4"
)

// Valid reports whether s is one of P1..P4.
func (s Severity) Valid() bool {
	switch s {
	case P1, P2, P3, P4:
		return true
	}
	return false
}

// Register mounts the module's routes under /v1.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/tickets", notImplemented)
	mux.HandleFunc("POST /v1/tickets", notImplemented)
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
	platform.WriteProblem(w, r, http.StatusNotImplemented, "Not implemented", "Phase 1 deliverable")
}
