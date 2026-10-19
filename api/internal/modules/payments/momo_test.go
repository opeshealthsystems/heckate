package payments

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

func fakeMoMo(t *testing.T, status string) (*httptest.Server, *[]string) {
	var seen []string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /collection/token/", func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, "token")
		if u, p, ok := r.BasicAuth(); !ok || u != "user" || p != "key" || r.Header.Get("Ocp-Apim-Subscription-Key") != "sub" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok-1", "expires_in": 3600})
	})
	mux.HandleFunc("POST /collection/v1_0/requesttopay", func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, "charge")
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		if r.Header.Get("Authorization") != "Bearer tok-1" || r.Header.Get("X-Reference-Id") != "ref-1" || r.Header.Get("X-Target-Environment") != "mtncameroon" ||
			b["amount"] != "119250" || b["currency"] != "XAF" || b["payer"].(map[string]any)["partyId"] != "237670000000" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("GET /collection/v1_0/requesttopay/{ref}", func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, "status")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "amount": "119250", "reason": nil})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &seen
}

func TestMoMoChargeAndStatus(t *testing.T) {
	for in, want := range map[string]string{"SUCCESSFUL": "succeeded", "PENDING": "pending", "FAILED": "failed", "REJECTED": "failed", "WHATEVER": "pending"} {
		srv, seen := fakeMoMo(t, in)
		g := NewMoMo(platform.MoMoConfig{BaseURL: srv.URL, SubscriptionKey: "sub", APIUser: "user", APIKey: "key", Environment: "mtncameroon"}, nil)
		ref, err := g.Charge(context.Background(), ChargeRequest{Reference: "ref-1", Amount: 119250, MSISDN: "237670000000", Message: "x"})
		if err != nil || ref != "ref-1" {
			t.Fatalf("%s charge: %q %v", in, ref, err)
		}
		st, err := g.Status(context.Background(), ref)
		if err != nil || st.Status != want || st.Amount != 119250 {
			t.Fatalf("%s status: %+v %v (want %s)", in, st, err, want)
		}
		tokens := 0
		for _, s := range *seen {
			if s == "token" {
				tokens++
			}
		}
		if tokens != 1 {
			t.Errorf("the access token should be fetched once and reused, got %d", tokens)
		}
	}
}

func TestMoMoRefusesWhenCredentialsOrRequestAreWrong(t *testing.T) {
	srv, _ := fakeMoMo(t, "PENDING")
	bad := NewMoMo(platform.MoMoConfig{BaseURL: srv.URL, SubscriptionKey: "sub", APIUser: "user", APIKey: "WRONG", Environment: "mtncameroon"}, nil)
	if _, err := bad.Charge(context.Background(), ChargeRequest{Reference: "ref-1", Amount: 1, MSISDN: "237670000000"}); err == nil {
		t.Error("a rejected token request must fail the charge")
	}
	good := NewMoMo(platform.MoMoConfig{BaseURL: srv.URL, SubscriptionKey: "sub", APIUser: "user", APIKey: "key", Environment: "mtncameroon"}, nil)
	if _, err := good.Charge(context.Background(), ChargeRequest{Reference: "ref-1", Amount: 5, MSISDN: "237670000000"}); err == nil {
		t.Error("a request the provider answers 400 must fail")
	}
	srv.Close()
	if _, err := good.Status(context.Background(), "ref-1"); err == nil {
		t.Error("an unreachable provider must be an error, never a guess")
	}
}

func TestGatewaysFollowConfiguration(t *testing.T) {
	if len(Gateways(platform.Config{})) != 0 {
		t.Error("no configuration, no gateways")
	}
	if g := Gateways(platform.Config{PaymentsSandbox: true}); len(g) != 2 {
		t.Errorf("sandbox gives both: %v", g)
	}
	cfg := platform.Config{MoMo: platform.MoMoConfig{BaseURL: "https://x", SubscriptionKey: "s", APIUser: "u", APIKey: "k"}}
	if _, ok := Gateways(cfg)["momo"].(*MoMo); !ok {
		t.Error("full MoMo configuration enables MoMo")
	}
	cfg.MoMo.APIKey = ""
	if len(Gateways(cfg)) != 0 {
		t.Error("partial MoMo configuration must not enable it")
	}
}
