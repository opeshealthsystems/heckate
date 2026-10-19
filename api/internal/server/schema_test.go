package server

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Tables that have row-level security enabled but not forced, because a SECURITY DEFINER function owned by
// the schema owner must still reach them (public verification, API-key lookup, payroll release, public
// enquiries, device push, audit sealing). The application role is neither owner nor superuser and has no
// BYPASSRLS, so the rules apply to it on every one of these. A new entry needs a reason here.
var notForced = map[string]string{
	"crm.leads":           "public enquiry form inserts through a definer function",
	"devapi.keys":         "API-key lookup by hash happens before any tenant is known",
	"devapi.plans":        "plan lookup during key authentication",
	"devapi.usage":        "metering through a definer function",
	"documents.artifacts": "policy denies everyone; stored and served only by definer functions",
	"documents.register":  "public verification through a definer function",
	"notify.devices":      "push fan-out reads devices across users through a definer function",
	"payroll.runs":        "payroll.run_released() lets an employee's payslip policy see the run's status",
	"platform.audit_log":  "read and sealed by definer functions; the application role may only INSERT",
}

func TestEveryTableHasRowLevelSecurity(t *testing.T) {
	e := setup(t)
	_ = e
	conn, err := pgx.Connect(context.Background(), adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(context.Background(), `SELECT n.nspname||'.'||c.relname, c.relrowsecurity, c.relforcerowsecurity,
		(SELECT count(*) FROM pg_policy p WHERE p.polrelid = c.oid)
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r' AND n.nspname NOT IN ('pg_catalog','information_schema','public') ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var name string
		var rls, force bool
		var policies int
		if err := rows.Scan(&name, &rls, &force, &policies); err != nil {
			t.Fatal(err)
		}
		seen++
		if !rls {
			t.Errorf("%s has no row-level security: enable it and write its policies in a migration", name)
		}
		if rls && policies == 0 {
			t.Errorf("%s has row-level security but no policy, so nobody can read it", name)
		}
		if _, ok := notForced[name]; rls && !force && !ok {
			t.Errorf("%s has row-level security that is not FORCEd: force it, or list it in notForced with a reason", name)
		}
	}
	if seen < 60 {
		t.Errorf("expected to inspect the whole schema, saw only %d tables", seen)
	}
}

// What the application role must never be able to do, even if a bug reached raw SQL.
func TestApplicationRoleCannotRewriteHistory(t *testing.T) {
	setup(t)
	conn, err := pgx.Connect(context.Background(), adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	appendOnly := []string{"accounting.entries", "accounting.entry_lines", "accounting.closed_periods", "inventory.moves", "platform.audit_log"}
	var bad []string
	for _, tbl := range appendOnly {
		for _, priv := range []string{"UPDATE", "DELETE", "TRUNCATE"} {
			if tbl == "platform.audit_log" && priv == "UPDATE" {
				continue // the sealer is a definer function; the application role has INSERT only, checked below
			}
			var ok bool
			if err := conn.QueryRow(context.Background(), `SELECT has_table_privilege('hekate_app', $1, $2)`, tbl, priv).Scan(&ok); err != nil {
				t.Fatal(err)
			}
			if ok {
				bad = append(bad, tbl+" "+priv)
			}
		}
	}
	var ok bool
	if err := conn.QueryRow(context.Background(), `SELECT has_table_privilege('hekate_app', 'platform.audit_log', 'SELECT') OR has_table_privilege('hekate_app', 'platform.audit_log', 'UPDATE')`).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	if ok {
		bad = append(bad, "platform.audit_log SELECT/UPDATE")
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("the application role must not hold these privileges: %s", strings.Join(bad, ", "))
	}
}

// Every foreign key needs an index that starts with its columns, or joins and the per-client row filters
// degrade into table scans as data grows. Tiny reference lookups are exempt.
func TestEveryForeignKeyIsIndexed(t *testing.T) {
	setup(t)
	conn, err := pgx.Connect(context.Background(), adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	exempt := map[string]bool{"accounting.entries|journal": true, "accounting.bank_statements|account": true, "devapi.keys|plan": true}
	rows, err := conn.Query(context.Background(), `SELECT c.conrelid::regclass::text||'|'||string_agg(a.attname, ',' ORDER BY a.attnum)
		FROM pg_constraint c JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
		WHERE c.contype = 'f' AND NOT EXISTS (SELECT 1 FROM pg_index i WHERE i.indrelid = c.conrelid AND (i.indkey::int2[])[0:array_length(c.conkey,1)-1] = c.conkey)
		GROUP BY c.oid, c.conrelid ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var fk string
		if err := rows.Scan(&fk); err != nil {
			t.Fatal(err)
		}
		if !exempt[fk] {
			t.Errorf("foreign key %s has no index: add one in a migration", strings.Replace(fk, "|", " (", 1)+")")
		}
	}
}
