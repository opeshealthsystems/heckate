package payments

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

func TestOrangeChargeAndStatus(t *testing.T) {
	for in, want := range map[string]string{"SUCCESS": "succeeded", "INITIATED": "pending", "PENDING": "pending", "FAILED": "failed", "EXPIRED": "failed"} {
		mux := http.NewServeMux()
		mux.HandleFunc("POST /oauth/v3/token", func(w http.ResponseWriter, r *http.Request) {
			if u, p, ok := r.BasicAuth(); !ok || u != "cid" || p != "sec" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 3600})
		})
		mux.HandleFunc("POST /orange-money-webpay/cm/v1/webpayment", func(w http.ResponseWriter, r *http.Request) {
			var b map[string]any
			_ = json.NewDecoder(r.Body).Decode(&b)
			if r.Header.Get("Authorization") != "Bearer tok" || b["merchant_key"] != "mk" || b["order_id"] != "o-1" || b["amount"] != float64(5000) || b["currency"] != "XAF" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"pay_token": "pt-9", "payment_url": "https://pay.example/x"})
		})
		mux.HandleFunc("POST /orange-money-webpay/cm/v1/transactionstatus", func(w http.ResponseWriter, r *http.Request) {
			var b map[string]any
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b["order_id"] != "o-1" || b["pay_token"] != "pt-9" || b["amount"] != float64(5000) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": in})
		})
		srv := httptest.NewServer(mux)
		g := NewOrange(platform.OrangeConfig{BaseURL: srv.URL, ClientID: "cid", ClientSecret: "sec", MerchantKey: "mk"}, nil)
		ref, err := g.Charge(context.Background(), ChargeRequest{Reference: "o-1", Amount: 5000, Message: "inv"})
		if err != nil || ref != "o-1|pt-9|5000" {
			t.Fatalf("%s charge: %q %v", in, ref, err)
		}
		st, err := g.Status(context.Background(), ref)
		if err != nil || st.Status != want {
			t.Fatalf("%s status: %+v %v (want %s)", in, st, err, want)
		}
		srv.Close()
	}
	g := NewOrange(platform.OrangeConfig{BaseURL: "http://127.0.0.1:1", ClientID: "a", ClientSecret: "b", MerchantKey: "c"}, nil)
	if _, err := g.Charge(context.Background(), ChargeRequest{Reference: "x", Amount: 1}); err == nil {
		t.Fatal("an unreachable provider must be an error")
	}
	if _, err := g.Status(context.Background(), "garbage"); err == nil {
		t.Fatal("a malformed reference must be an error")
	}
}

func TestOrangeIsOffWithoutCredentials(t *testing.T) {
	if _, ok := Gateways(platform.Config{})["orange_money"]; ok {
		t.Fatal("orange_money must be disabled without credentials")
	}
	cfg := platform.Config{Orange: platform.OrangeConfig{BaseURL: "https://x", ClientID: "a", ClientSecret: "b", MerchantKey: "c"}}
	if _, ok := Gateways(cfg)["orange_money"]; !ok {
		t.Fatal("orange_money must be enabled once configured")
	}
}
