package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func (e *env) callback(provider string, event map[string]any, signWith string) int {
	e.t.Helper()
	body, _ := json.Marshal(event)
	mac := hmac.New(sha256.New, []byte(signWith))
	mac.Write(body)
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/webhooks/payments/"+provider, bytes.NewReader(body))
	if signWith != "" {
		req.Header.Set("X-Hekate-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	var out bytes.Buffer
	_, _ = out.ReadFrom(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		e.t.Logf("callback %d: %s", resp.StatusCode, out.String())
	}
	return resp.StatusCode
}

func TestOnlinePaymentsNeedAConfirmedAnswer(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	fin := token("pay-fin", "", "finance", "sales")
	ta := e.newClient(token("m", "", "manager"), "Pay A "+sfx)
	tb := e.newClient(token("m", "", "manager"), "Pay B "+sfx)
	alice := token("pay-alice", ta, "client_admin")
	bob := token("pay-bob", tb, "client_admin")

	_, inv, _ := e.do("POST", "/v1/invoices", fin, map[string]any{"tenant_id": ta, "title": "Online " + sfx, "lines": lines(100_000)})
	id := inv["id"].(string)
	e.do("POST", "/v1/invoices/"+id+"/issue", fin, nil)
	total := int64(119_250)
	start := map[string]any{"provider": "momo", "msisdn": "237670000000"}

	if c, _, _ := e.do("POST", "/v1/invoices/"+id+"/online-payments", bob, start); c != 404 {
		t.Errorf("another company paying: want 404, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/invoices/"+id+"/online-payments", fin, start); c != 403 {
		t.Errorf("staff starting a client payment: want 403, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/invoices/"+id+"/online-payments", alice, map[string]any{"provider": "momo", "msisdn": "12"}); c != 422 {
		t.Errorf("bad number: want 422, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/invoices/"+id+"/online-payments", alice, map[string]any{"provider": "momo", "msisdn": "237670000000", "amount": total + 1}); c != 422 {
		t.Errorf("more than the balance: want 422, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/invoices/"+id+"/online-payments", alice, map[string]any{"provider": "paypal", "msisdn": "237670000000"}); c != 503 {
		t.Errorf("unknown provider: want 503, got %d", c)
	}
	code, it, _ := e.do("POST", "/v1/invoices/"+id+"/online-payments", alice, start)
	if code != 202 || it["status"] != "pending" || it["amount"] != float64(total) {
		t.Fatalf("start: %d %v", code, it)
	}
	first := it["id"].(string)
	if c, _, _ := e.do("POST", "/v1/invoices/"+id+"/online-payments", alice, start); c != 409 {
		t.Errorf("second request while one is pending: want 409, got %d", c)
	}
	// Asking the sandbox provider says "still waiting"; nothing is booked.
	if c, b, _ := e.do("POST", "/v1/online-payments/"+first+"/refresh", alice, nil); c != 200 || b["status"] != "pending" {
		t.Errorf("refresh: %d %v", c, b)
	}

	// Only a correctly signed callback counts.
	ev := func(eventID, ref, status string, amount int64) map[string]any {
		return map[string]any{"event_id": eventID, "reference": ref, "status": status, "amount": amount}
	}
	if c := e.callback("momo", ev("e-"+sfx+"-1", first, "succeeded", total), ""); c != 401 {
		t.Errorf("unsigned callback: want 401, got %d", c)
	}
	if c := e.callback("momo", ev("e-"+sfx+"-1", first, "succeeded", total), "wrong-secret"); c != 401 {
		t.Errorf("wrongly signed callback: want 401, got %d", c)
	}
	_, got, _ := e.do("GET", "/v1/invoices/"+id, alice, nil)
	if got["status"] != "issued" || got["paid"] != float64(0) {
		t.Fatalf("an unconfirmed payment must not touch the invoice: %v", got)
	}

	// A different amount than requested is parked for finance, not booked.
	if c := e.callback("momo", ev("e-"+sfx+"-2", first, "succeeded", total-1000), paySecret); c != 200 {
		t.Fatalf("callback with wrong amount: %d", c)
	}
	_, list, _ := e.do("GET", "/v1/invoices/"+id+"/online-payments", alice, nil)
	if list["data"].([]any)[0].(map[string]any)["status"] != "review" {
		t.Errorf("wrong amount should go to review: %v", list)
	}
	_, got, _ = e.do("GET", "/v1/invoices/"+id, alice, nil)
	if got["paid"] != float64(0) {
		t.Errorf("a mismatched amount must not be booked: %v", got["paid"])
	}

	// A failed attempt frees the invoice for another try; the next one succeeds.
	_, it2, _ := e.do("POST", "/v1/invoices/"+id+"/online-payments", alice, start)
	second := it2["id"].(string)
	if c := e.callback("momo", ev("e-"+sfx+"-3", second, "failed", 0), paySecret); c != 200 {
		t.Fatalf("failed callback: %d", c)
	}
	_, it3, _ := e.do("POST", "/v1/invoices/"+id+"/online-payments", alice, start)
	third := it3["id"].(string)
	if c := e.callback("momo", ev("e-"+sfx+"-4", third, "succeeded", total), paySecret); c != 200 {
		t.Fatalf("success callback: %d", c)
	}
	// A repeated delivery, and a later contradictory one, change nothing.
	if c := e.callback("momo", ev("e-"+sfx+"-4", third, "succeeded", total), paySecret); c != 200 {
		t.Errorf("repeated delivery: %d", c)
	}
	if c := e.callback("momo", ev("e-"+sfx+"-5", third, "failed", 0), paySecret); c != 200 {
		t.Errorf("late contradictory callback: %d", c)
	}
	_, got, _ = e.do("GET", "/v1/invoices/"+id, alice, nil)
	if got["status"] != "paid" || got["paid"] != float64(total) || got["balance"] != float64(0) {
		t.Fatalf("invoice should be paid exactly once: %v", got)
	}
	_, list, _ = e.do("GET", "/v1/invoices/"+id+"/online-payments", alice, nil)
	statuses := map[string]int{}
	for _, x := range list["data"].([]any) {
		statuses[x.(map[string]any)["status"].(string)]++
	}
	if statuses["succeeded"] != 1 || statuses["failed"] != 1 || statuses["review"] != 1 {
		t.Errorf("intent history: %v", statuses)
	}
	if c, _, _ := e.do("GET", "/v1/invoices/"+id+"/online-payments", bob, nil); c != 200 {
		t.Errorf("listing for another company should be empty, got %d", c)
	}
	if c := e.callback("momo", ev("e-"+sfx+"-6", "00000000-0000-0000-0000-000000000000", "succeeded", total), paySecret); c != 200 {
		t.Errorf("callback for an unknown reference is logged, not an error: %d", c)
	}
	if c := e.callback("nowhere", ev("e-"+sfx+"-7", third, "succeeded", total), paySecret); c != 404 {
		t.Errorf("unknown provider: want 404, got %d", c)
	}
	if c, b, _ := e.do("GET", "/v1/payment-providers", alice, nil); c != 200 || len(b["data"].([]any)) != 2 {
		t.Errorf("providers: %d %v", c, b)
	}
}
