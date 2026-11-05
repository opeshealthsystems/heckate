// Package notify manages push-notification device registrations.
// Delivery (FCM/APNs) consumes outbox events and needs provider credentials; see docs/BLOCKERS.md.
package notify

import (
	"net/http"
	"strings"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

func Register(a *platform.App) {
	a.Route("PUT /v1/devices", platform.Portal, register)
	a.Route("DELETE /v1/devices/{token}", platform.Portal, unregister)
}

func register(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Token    string `json:"token"`
		Platform string `json:"platform"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	in.Token = strings.TrimSpace(in.Token)
	if in.Token == "" || len(in.Token) > 512 || (in.Platform != "ios" && in.Platform != "android" && in.Platform != "expo") {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid device", "token and platform (ios, android, expo) are required")
	}
	var tenant any
	if s.Claims.TenantID != "" {
		tenant = s.Claims.TenantID
	}
	// A token is reassigned to whoever registers it last (device handed to another user after sign-out).
	if _, err := s.Tx.Exec(r.Context(), `SELECT notify.register_device($1,$2,$3,$4,$5)`, in.Token, s.Claims.Sub, tenant, in.Platform, s.Claims.Roles); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusNoContent}, nil
}

func unregister(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	if _, err := s.Tx.Exec(r.Context(), `DELETE FROM notify.devices WHERE token = $1`, r.PathValue("token")); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusNoContent}, nil
}
