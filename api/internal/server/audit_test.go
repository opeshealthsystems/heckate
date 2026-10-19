package server

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestAuditTrailIsSealedReadableAndTamperEvident(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	seal := func() {
		t.Helper()
		if _, err := conn.Exec(ctx, `SELECT platform.seal_audit()`); err != nil {
			t.Fatal(err)
		}
	}

	sfx := fmt.Sprint(time.Now().UnixNano())
	staff := token("audit-staff", "", "manager")
	tid := e.newClient(staff, "Audit Co "+sfx)
	admin := token("audit-admin-"+sfx, tid, "client_admin")
	if c, b, _ := e.do("POST", "/v1/api-keys", admin, map[string]any{"name": "audited", "scopes": []string{"read:tickets"}}); c != 201 {
		t.Fatalf("create key: %d %v", c, b)
	}
	seal()

	// Only management and the auditor can read the trail.
	if c, _, _ := e.do("GET", "/v1/audit", admin, nil); c != 403 {
		t.Errorf("client reading the audit trail: want 403, got %d", c)
	}
	if c, _, _ := e.do("GET", "/v1/audit/verify", token("sup", "", "support"), nil); c != 403 {
		t.Errorf("support verifying: want 403, got %d", c)
	}
	aud := token("auditor-1", "", "auditor")
	_, list, _ := e.do("GET", "/v1/audit?limit=50&actor=audit-admin-"+sfx, aud, nil)
	rows := list["data"].([]any)
	if len(rows) == 0 || rows[0].(map[string]any)["action"] != "create" || rows[0].(map[string]any)["entity"] != "api_key" {
		t.Fatalf("the key creation should be in the trail: %v", list)
	}
	if rows[0].(map[string]any)["hash"] == nil {
		t.Error("a sealed row carries its hash")
	}
	_, v, _ := e.do("GET", "/v1/audit/verify", aud, nil)
	if v["intact"] != true || v["checked"].(float64) < 1 {
		t.Fatalf("a fresh chain must verify: %v", v)
	}

	// The application role cannot change history, and neither can the table's owner by accident.
	for name, q := range map[string]string{
		"update": `UPDATE platform.audit_log SET action = 'forged' WHERE actor = $1`,
		"delete": `DELETE FROM platform.audit_log WHERE actor = $1`,
	} {
		if _, err := conn.Exec(ctx, q, "audit-admin-"+sfx); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%s must be refused as append-only, got %v", name, err)
		}
	}
	if _, err := conn.Exec(ctx, `TRUNCATE platform.audit_log CASCADE`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Errorf("truncate must be refused, got %v", err)
	}

	// Someone with full control who disables the guard and rewrites a sealed row is caught by the chain.
	var id int64
	if err := conn.QueryRow(ctx, `SELECT id FROM platform.audit_log WHERE actor = $1 AND hash IS NOT NULL ORDER BY id LIMIT 1`, "audit-admin-"+sfx).Scan(&id); err != nil {
		t.Fatal(err)
	}
	restore := func() {
		_, _ = conn.Exec(ctx, `ALTER TABLE platform.audit_log DISABLE TRIGGER audit_guard`)
		_, _ = conn.Exec(ctx, `UPDATE platform.audit_log SET action = 'create' WHERE id = $1`, id)
		_, _ = conn.Exec(ctx, `ALTER TABLE platform.audit_log ENABLE TRIGGER audit_guard`)
	}
	defer restore()
	if _, err := conn.Exec(ctx, `ALTER TABLE platform.audit_log DISABLE TRIGGER audit_guard`); err != nil {
		t.Skip("cannot disable triggers on this database: ", err)
	}
	if _, err := conn.Exec(ctx, `UPDATE platform.audit_log SET action = 'forged' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	_, v, _ = e.do("GET", "/v1/audit/verify", aud, nil)
	if v["intact"] != false || v["first_broken_seq"] == nil {
		t.Fatalf("a rewritten row must break the chain: %v", v)
	}
	restore()
	_, v, _ = e.do("GET", "/v1/audit/verify", aud, nil)
	if v["intact"] != true {
		t.Fatalf("after the row is put back the chain verifies again: %v", v)
	}
}
