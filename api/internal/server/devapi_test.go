package server

import (
	"testing"
	"time"
)

func TestAPIKeysScopesPlansAndMetering(t *testing.T) {
	e := setup(t)
	staff := token("agent-k", "", "support", "manager")
	ta := e.newClient(staff, "KeyCo A "+t.Name())
	tb := e.newClient(staff, "KeyCo B "+t.Name())
	adminA := token("admin-a", ta, "client_admin")
	userA := token("user-a", ta, "client_user")
	adminB := token("admin-b", tb, "client_admin")

	if code, tk, _ := e.do("POST", "/v1/tickets", userA, map[string]any{"subject": "Printer", "description": "jam", "severity": "P3", "channel": "app"}); code != 201 {
		t.Fatalf("ticket: %d %v", code, tk)
	}

	// Only a client admin of the company creates keys.
	body := map[string]any{"name": "ERP sync", "scopes": []string{"read:tickets"}}
	if code, _, _ := e.do("POST", "/v1/api-keys", userA, body); code != 403 {
		t.Errorf("client_user create key: want 403, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/api-keys", staff, body); code != 403 {
		t.Errorf("staff create key: want 403, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/api-keys", adminA, map[string]any{"name": "x", "scopes": []string{"write:tickets"}}); code != 422 {
		t.Errorf("bad scope: want 422, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/api-keys", adminA, map[string]any{"name": "x", "scopes": []string{"read:api-keys"}}); code != 422 {
		t.Errorf("key-management scope: want 422, got %d", code)
	}
	code, created, _ := e.do("POST", "/v1/api-keys", adminA, body)
	if code != 201 {
		t.Fatalf("create key: %d %v", code, created)
	}
	secretKey := created["secret"].(string)
	keyID := created["key"].(map[string]any)["id"].(string)
	if len(secretKey) < 40 || secretKey[:3] != "hk_" {
		t.Fatalf("odd secret %q", secretKey)
	}

	// The secret is never listed again, and other companies cannot see the key.
	_, list, _ := e.do("GET", "/v1/api-keys", adminA, nil)
	if len(list["data"].([]any)) != 1 {
		t.Errorf("admin A should see 1 key, got %v", list["data"])
	}
	if _, has := list["data"].([]any)[0].(map[string]any)["secret"]; has {
		t.Error("secret must not be listed")
	}
	_, other, _ := e.do("GET", "/v1/api-keys", adminB, nil)
	if len(other["data"].([]any)) != 0 {
		t.Errorf("admin B must not see A's keys: %v", other["data"])
	}
	if code, _, _ := e.do("DELETE", "/v1/api-keys/"+keyID, adminB, nil); code != 404 {
		t.Errorf("cross-tenant revoke: want 404, got %d", code)
	}

	// The key reads its own company's tickets, and only those.
	code, tickets, hdr := e.do("GET", "/v1/tickets", secretKey, nil)
	if code != 200 || len(tickets["data"].([]any)) != 1 {
		t.Fatalf("key list tickets: %d %v", code, tickets)
	}
	if hdr.Get("RateLimit-Limit") != "60" {
		t.Errorf("free plan rate limit 60, got %q", hdr.Get("RateLimit-Limit"))
	}
	// Outside its scopes, writing, and managing keys are all refused.
	if code, _, _ := e.do("GET", "/v1/invoices", secretKey, nil); code != 403 {
		t.Errorf("out-of-scope read: want 403, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/tickets", secretKey, map[string]any{"subject": "s", "description": "d", "severity": "P3", "channel": "app"}); code != 403 {
		t.Errorf("key write: want 403, got %d", code)
	}
	if code, _, _ := e.do("GET", "/v1/api-keys", secretKey, nil); code != 403 {
		t.Errorf("key managing keys: want 403, got %d", code)
	}
	if code, _, _ := e.do("GET", "/v1/clients", secretKey, nil); code != 403 {
		t.Errorf("staff endpoint with key: want 403, got %d", code)
	}
	if code, _, _ := e.do("GET", "/v1/tickets", "hk_deadbeef_"+secretKey[12:], nil); code != 401 {
		t.Errorf("forged key: want 401, got %d", code)
	}

	// Plans: clients cannot pick paid plans; staff move a key to a plan with a tiny quota.
	if code, _, _ := e.do("PUT", "/v1/api-plans/paid", staff, map[string]any{"code": "paid", "name": "Paid", "rate_per_min": 600, "monthly_quota": 1000000, "price_xaf": 50000, "active": true}); code != 200 {
		t.Fatalf("put plan: %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/api-keys", adminA, map[string]any{"name": "p", "scopes": []string{"read:*"}, "plan": "paid"}); code != 403 {
		t.Errorf("client picking paid plan: want 403, got %d", code)
	}
	if code, _, _ := e.do("PUT", "/v1/api-plans/tiny", adminA, map[string]any{"name": "Tiny", "rate_per_min": 100, "monthly_quota": 2}); code != 403 {
		t.Errorf("client editing plans: want 403, got %d", code)
	}
	suffix := time.Now().UnixNano() % 100000
	planCode := "tiny" + string(rune('a'+suffix%26)) + string(rune('a'+suffix/26%26)) + string(rune('a'+suffix/676%26))
	if code, p, _ := e.do("PUT", "/v1/api-plans/"+planCode, staff, map[string]any{"name": "Tiny", "rate_per_min": 100, "monthly_quota": 2, "active": true}); code != 200 {
		t.Fatalf("tiny plan: %d %v", code, p)
	}
	if code, _, _ := e.do("PATCH", "/v1/api-keys/"+keyID, adminA, map[string]any{"plan": planCode}); code != 403 {
		t.Errorf("client changing own plan: want 403, got %d", code)
	}
	if code, k, _ := e.do("PATCH", "/v1/api-keys/"+keyID, staff, map[string]any{"plan": planCode}); code != 200 || k["plan"] != planCode {
		t.Fatalf("staff set plan: %d %v", code, k)
	}
	// Two calls were already made this month (list + out-of-scope are not metered... only authenticated in-scope ones are).
	_, usage, _ := e.do("GET", "/v1/api-keys/"+keyID+"/usage", adminA, nil)
	used := int(usage["data"].([]any)[0].(map[string]any)["calls"].(float64))
	if used < 1 {
		t.Fatalf("usage should count calls, got %v", usage)
	}
	var last int
	for i := 0; i < 3; i++ {
		last, _, _ = e.do("GET", "/v1/tickets", secretKey, nil)
		if last == 429 {
			break
		}
	}
	if last != 429 {
		t.Errorf("quota of 2 must end in 429, got %d", last)
	}

	// Revocation is immediate, and permanent.
	if code, _, _ := e.do("DELETE", "/v1/api-keys/"+keyID, adminA, nil); code != 204 {
		t.Fatalf("revoke: %d", code)
	}
	if code, _, _ := e.do("GET", "/v1/tickets", secretKey, nil); code != 401 {
		t.Errorf("revoked key: want 401, got %d", code)
	}
	if code, _, _ := e.do("DELETE", "/v1/api-keys/"+keyID, adminA, nil); code != 404 {
		t.Errorf("double revoke: want 404, got %d", code)
	}
}
