// Command loadtest drives a running HEKATE API with a mix of reads and writes and reports throughput,
// latency percentiles and errors. It exists so capacity claims are measured, not guessed.
//
//	HEKATE_DEV_JWT_SECRET=... loadtest -url http://localhost:8080 -c 50 -d 30s -writes 10
//
// The API must run with HEKATE_ENV=development and the same dev secret. Never point this at production.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

type result struct {
	op   string
	d    time.Duration
	code int
}

func main() {
	url := flag.String("url", "http://localhost:8080", "API base URL")
	conc := flag.Int("c", 50, "concurrent clients")
	dur := flag.Duration("d", 20*time.Second, "test duration")
	writes := flag.Int("writes", 10, "percent of requests that create a ticket")
	tenants := flag.Int("tenants", 20, "client companies to spread load over")
	only := flag.String("only", "", "run a single operation: me, list or create (default: the mix)")
	flag.Parse()
	secret := os.Getenv("HEKATE_DEV_JWT_SECRET")
	if secret == "" {
		fmt.Fprintln(os.Stderr, "set HEKATE_DEV_JWT_SECRET (same value as the API)")
		os.Exit(2)
	}
	tok := func(sub, tenant string, roles ...string) string {
		return platform.SignDevToken(secret, map[string]any{"sub": sub, "tenant_id": tenant, "roles": roles, "exp": time.Now().Add(time.Hour).Unix()})
	}
	hc := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: *conc * 2}}
	call := func(method, path, token string, body any) (int, []byte) {
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, *url+path, rd)
		req.Header.Set("Authorization", "Bearer "+token)
		if method == "POST" {
			k := make([]byte, 12)
			_, _ = rand.Read(k)
			req.Header.Set("Idempotency-Key", hex.EncodeToString(k))
		}
		resp, err := hc.Do(req)
		if err != nil {
			return 0, nil
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b
	}

	staff := tok("load-staff", "", "manager", "support")
	var tids []string
	for i := 0; i < *tenants; i++ {
		code, b := call("POST", "/v1/clients", staff, map[string]any{"legal_name": fmt.Sprintf("Load %d %d", time.Now().UnixNano(), i)})
		var c struct {
			TenantID string `json:"tenant_id"`
		}
		if code != 201 || json.Unmarshal(b, &c) != nil {
			fmt.Fprintf(os.Stderr, "setup failed: %d %s\n", code, b)
			os.Exit(2)
		}
		tids = append(tids, c.TenantID)
		// A little data so list endpoints return rows.
		ct := tok("load-seed", c.TenantID, "client_user")
		for j := 0; j < 5; j++ {
			call("POST", "/v1/tickets", ct, map[string]any{"subject": "seed", "description": "seed", "severity": "P3", "channel": "app"})
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var all []result
	var n atomic.Int64
	stop := time.Now().Add(*dur)
	start := time.Now()
	for i := 0; i < *conc; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tenant := tids[i%len(tids)]
			ct := tok(fmt.Sprintf("load-user-%d", i), tenant, "client_user")
			var local []result
			for time.Now().Before(stop) {
				k := int(n.Add(1))
				var op string
				var code int
				t0 := time.Now()
				switch {
				case *only == "me":
					op = "who am i"
					code, _ = call("GET", "/v1/me", ct, nil)
				case *only == "list":
					op = "list tickets"
					code, _ = call("GET", "/v1/tickets", ct, nil)
				case *only == "create" || (*only == "" && k%100 < *writes):
					op = "create ticket"
					code, _ = call("POST", "/v1/tickets", ct, map[string]any{"subject": "load", "description": "load", "severity": "P3", "channel": "app"})
				case *only == "" && k%2 == 0:
					op = "list tickets"
					code, _ = call("GET", "/v1/tickets", ct, nil)
				default:
					op = "who am i"
					code, _ = call("GET", "/v1/me", ct, nil)
				}
				local = append(local, result{op, time.Since(t0), code})
			}
			mu.Lock()
			all = append(all, local...)
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	by := map[string][]result{}
	for _, r := range all {
		by[r.op] = append(by[r.op], r)
	}
	fmt.Printf("%d clients, %s, %d%% writes, %d companies\n", *conc, elapsed.Round(time.Second), *writes, *tenants)
	fmt.Printf("total %d requests, %.0f req/s\n\n", len(all), float64(len(all))/elapsed.Seconds())
	fmt.Printf("%-14s %8s %8s %8s %8s %8s %8s\n", "operation", "count", "errors", "p50", "p95", "p99", "max")
	ops := make([]string, 0, len(by))
	for k := range by {
		ops = append(ops, k)
	}
	sort.Strings(ops)
	failed := 0
	for _, op := range ops {
		rs := by[op]
		sort.Slice(rs, func(i, j int) bool { return rs[i].d < rs[j].d })
		errs := 0
		for _, r := range rs {
			if r.code < 200 || r.code >= 300 {
				errs++
			}
		}
		failed += errs
		pct := func(p float64) time.Duration { return rs[int(float64(len(rs)-1)*p)].d.Round(100 * time.Microsecond) }
		fmt.Printf("%-14s %8d %8d %8s %8s %8s %8s\n", op, len(rs), errs, pct(0.5), pct(0.95), pct(0.99), rs[len(rs)-1].d.Round(100*time.Microsecond))
	}
	if failed > 0 {
		os.Exit(1)
	}
}
