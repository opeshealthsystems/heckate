package platform

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func appWithProxies(cidrs ...string) *App { return &App{Proxies: ParseProxies(cidrs)} }

func req(remote, xff string) *http.Request {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = remote
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	return r
}

func TestClientIPOnlyBelievesTrustedProxies(t *testing.T) {
	none := appWithProxies()
	if got := none.ClientIP(req("203.0.113.9:4444", "1.2.3.4")); got != "203.0.113.9" {
		t.Errorf("no trusted proxy: a forged header must be ignored, got %s", got)
	}
	a := appWithProxies("10.0.0.0/8")
	for _, c := range []struct{ remote, xff, want string }{
		{"203.0.113.9:1", "1.2.3.4", "203.0.113.9"},              // caller is not the proxy: header is a lie
		{"10.0.0.5:1", "198.51.100.7", "198.51.100.7"},           // the proxy vouches for one hop
		{"10.0.0.5:1", "6.6.6.6, 198.51.100.7", "198.51.100.7"},  // a forged left entry is ignored
		{"10.0.0.5:1", "198.51.100.7, 10.0.0.9", "198.51.100.7"}, // chained trusted proxies are skipped
		{"10.0.0.5:1", "not-an-ip", "10.0.0.5"},                  // garbage falls back to the proxy
		{"10.0.0.5:1", "", "10.0.0.5"},                           // no header
		{"[2001:db8::1]:5", "1.1.1.1", "2001:db8::1"},            // IPv6 caller, untrusted
	} {
		if got := a.ClientIP(req(c.remote, c.xff)); got != c.want {
			t.Errorf("remote %s xff %q: want %s, got %s", c.remote, c.xff, c.want, got)
		}
	}
}

func TestProductionRefusesUnsafeConfiguration(t *testing.T) {
	good := Config{Env: "production", JWKSURL: "https://auth/certs", Issuer: "https://auth/realms/h", Audience: "hekate-api",
		DatabaseURL: "postgres://u@db/h?sslmode=verify-full", CORSOrigins: "https://app.example.com"}
	if p := good.Problems(); len(p) != 0 {
		t.Fatalf("a sound production configuration was refused: %v", p)
	}
	for name, mod := range map[string]func(*Config){
		"dev secret":     func(c *Config) { c.DevSecret = "x" },
		"no jwks":        func(c *Config) { c.JWKSURL = "" },
		"no issuer":      func(c *Config) { c.Issuer = "" },
		"no audience":    func(c *Config) { c.Audience = "" },
		"sandbox pay":    func(c *Config) { c.PaymentsSandbox = true },
		"plain database": func(c *Config) { c.DatabaseURL = "postgres://u@db/h?sslmode=disable" },
		"wildcard cors":  func(c *Config) { c.CORSOrigins = "*" },
		"typo in env":    func(c *Config) { c.Env = "prod" },
		"bad proxy cidr": func(c *Config) { c.TrustedProxies = []string{"nope"} },
	} {
		c := good
		mod(&c)
		if len(c.Problems()) == 0 {
			t.Errorf("%s must be refused in production", name)
		}
	}
	dev := Config{Env: "development", DevSecret: "x", PaymentsSandbox: true, DatabaseURL: "postgres://u@localhost/h?sslmode=disable"}
	if p := dev.Problems(); len(p) != 0 {
		t.Errorf("development must run on a laptop: %v", p)
	}
	if (Config{}).Production() != true {
		t.Error("an empty environment must count as production")
	}
}

func TestSecureHeadersOnEveryResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	SecureHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	for h, want := range map[string]string{
		"X-Content-Type-Options": "nosniff", "Cache-Control": "no-store", "X-Frame-Options": "DENY", "Referrer-Policy": "no-referrer",
	} {
		if rec.Header().Get(h) != want {
			t.Errorf("%s = %q, want %q", h, rec.Header().Get(h), want)
		}
	}
	if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") || rec.Header().Get("Strict-Transport-Security") == "" {
		t.Error("CSP and HSTS must be set")
	}
}
