package identity

import (
	"net/http"
	"strings"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// DemoLogin is one "try it as ..." account shown on the mobile sign-in screen.
type DemoLogin struct {
	Label    string `json:"label"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ParseDemoLogins reads HEKATE_DEMO_LOGINS: entries separated by ";", each "Label|email|password".
// Malformed entries are skipped. Empty turns the demo buttons off without an app update.
func ParseDemoLogins(raw string) []DemoLogin {
	out := []DemoLogin{}
	for _, e := range strings.Split(raw, ";") {
		p := strings.Split(strings.TrimSpace(e), "|")
		if len(p) != 3 || strings.TrimSpace(p[0]) == "" || !strings.Contains(p[1], "@") || p[2] == "" {
			continue
		}
		out = append(out, DemoLogin{Label: strings.TrimSpace(p[0]), Email: strings.TrimSpace(p[1]), Password: p[2]})
	}
	return out
}

// RegisterDemo serves the demo accounts to the sign-in screen. The list lives in the server's environment,
// not in the app, so the owner can change or withdraw it at any time.
func RegisterDemo(a *platform.App, logins []DemoLogin) {
	a.Public("GET /v1/public/demo-logins", func(r *http.Request, s *platform.Scope) (*platform.Result, error) {
		return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": logins}, Header: map[string]string{"Cache-Control": "no-store"}}, nil
	})
}
