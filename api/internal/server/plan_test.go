package server

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// The row-level-security policies read "staff OR tenant_id = mine", which PostgreSQL cannot answer from an
// index. Client list queries therefore carry a redundant tenant predicate (Scope.TenantFilter). This test
// keeps both facts honest on a table big enough for the planner to care.
func TestClientListsUseTheTenantIndex(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	staff := token("plan-staff", "", "manager")
	sfx := fmt.Sprint(time.Now().UnixNano())
	var tenant string
	for i := 0; i < 25; i++ {
		tenant = e.newClient(staff, fmt.Sprintf("Plan %s %d", sfx, i))
		if _, err := conn.Exec(ctx, `INSERT INTO support.tickets (tenant_id, subject, description, severity, channel, reporter, sla_due_at)
			SELECT $1, 's' || g, 'd', 'P3', 'app', 'plan', now() + interval '1 day' FROM generate_series(1, 400) g`, tenant); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.Exec(ctx, `ANALYZE support.tickets`); err != nil {
		t.Fatal(err)
	}

	plan := func(q string) string {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		for _, s := range []string{"SET LOCAL ROLE hekate_app"} {
			if _, err := tx.Exec(ctx, s); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := tx.Exec(ctx, `SELECT set_config('app.role','client',true), set_config('app.tenant_id',$1,true), set_config('app.actor','p',true), set_config('app.roles','client_user',true)`, tenant); err != nil {
			t.Fatal(err)
		}
		rows, err := tx.Query(ctx, "EXPLAIN "+strings.ReplaceAll(q, "$1", "'"+tenant+"'"))
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var sb strings.Builder
		for rows.Next() {
			var l string
			_ = rows.Scan(&l)
			sb.WriteString(l + "\n")
		}
		return sb.String()
	}

	withFilter := plan(`SELECT id FROM support.tickets WHERE true AND tenant_id = $1::uuid ORDER BY created_at DESC, id DESC LIMIT 51`)
	if strings.Contains(withFilter, "Seq Scan") || !strings.Contains(withFilter, "Index") {
		t.Errorf("with the tenant predicate the planner must use an index:\n%s", withFilter)
	}
	without := plan(`SELECT id FROM support.tickets ORDER BY created_at DESC, id DESC LIMIT 51`)
	if !strings.Contains(without, "Seq Scan") {
		t.Logf("without the predicate the planner no longer scans the table (good news, but then TenantFilter can be retired):\n%s", without)
	}
}
