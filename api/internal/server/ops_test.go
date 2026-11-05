package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

func TestProbesAndMetrics(t *testing.T) {
	e := setup(t)
	for _, p := range []string{"/livez", "/readyz", "/healthz"} {
		if c, b, _ := e.do("GET", p, "", nil); c != 200 || b["status"] == nil {
			t.Errorf("%s: %d %v", p, c, b)
		}
	}

	// A metrics registry sees real traffic, labelled by route pattern and never by raw URL.
	pool, err := platform.NewPool(t.Context(), setupDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	h, _, m := NewObserved(platform.Config{DevSecret: secret}, pool, "test-1")
	srv := httptest.NewServer(h)
	defer srv.Close()
	get := func(path, tok string) int {
		req, _ := http.NewRequest("GET", srv.URL+path, nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	get("/livez", "")
	get("/v1/me", "")                         // 401
	get("/v1/me", token("m1", "", "manager")) // 200
	get("/v1/tickets/not-a-real-id-xyz", token("m1", "", "manager"))
	get("/definitely/not/a/route/12345", "") // unmatched, must not become a label

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body, _ := io.ReadAll(rec.Body)
	text := string(body)
	for _, want := range []string{
		`hekate_http_requests_total{route="GET /livez",method="GET",class="2xx"} 1`,
		`hekate_http_requests_total{route="GET /v1/me",method="GET",class="4xx"} 1`,
		`hekate_http_requests_total{route="GET /v1/me",method="GET",class="2xx"} 1`,
		`hekate_http_requests_total{route="unmatched",method="GET",class="4xx"} 1`,
		`hekate_http_request_duration_seconds_count{route="GET /livez"} 1`,
		`hekate_build_info{version="test-1"`,
		"hekate_db_up 1",
		"hekate_outbox_pending ",
		"hekate_audit_unsealed ",
		"hekate_db_pool_connections{state=\"max\"}",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("metrics missing %q", want)
		}
	}
	if strings.Contains(text, "not-a-real-id-xyz") || strings.Contains(text, "definitely") {
		t.Error("raw URLs must never become metric labels")
	}
}
