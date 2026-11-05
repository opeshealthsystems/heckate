package server

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/migrations"
)

// Production should own the schema with an ordinary role, not a superuser. Superusers bypass row-level
// security, which can hide a SECURITY DEFINER function that only works because of that. This runs the
// public paths (website enquiry, document verification, push registration) against a database owned by
// a non-superuser.
func TestPublicPathsWorkWhenTheOwnerIsNotASuperuser(t *testing.T) {
	admin := adminDSN()
	if admin == "" {
		t.Skip("HEKATE_TEST_DB not set")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	sfx := fmt.Sprint(time.Now().UnixNano())
	role, db := "owner_"+sfx, "ownerdb_"+sfx
	for _, sql := range []string{
		fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD 'pw' CREATEROLE NOSUPERUSER NOBYPASSRLS`, role),
		fmt.Sprintf(`CREATE DATABASE %s OWNER %s`, db, role),
		// Roles are cluster-wide and hekate_app already exists from the other test databases. In a fresh
		// cluster the owner creates it and so holds this admin right; here we hand it over explicitly.
		fmt.Sprintf(`GRANT hekate_app TO %s WITH ADMIN OPTION`, role),
	} {
		if _, err := conn.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(ctx, admin)
		if err == nil {
			defer c.Close(ctx)
			_, _ = c.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s WITH (FORCE)`, db))
			_, _ = c.Exec(ctx, fmt.Sprintf(`DROP ROLE IF EXISTS %s`, role))
		}
	})
	u, _ := url.Parse(admin)
	u.User = url.UserPassword(role, "pw")
	u.Path = "/" + db
	dsn := u.String()
	if err := platform.Migrate(ctx, dsn, migrations.FS); err != nil {
		t.Fatalf("migrations must run as an ordinary owner: %v", err)
	}
	pool, err := platform.NewPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	e := &env{t, httptestServer(t, pool)}

	// Website enquiry (definer function inserting into a staff-only table).
	if c, b, _ := e.do("POST", "/v1/public/enquiries", "", map[string]any{"name": "Jo", "email": "jo@example.com", "message": "hi"}); c != 202 {
		t.Errorf("enquiry as non-superuser owner: want 202, got %d %v", c, b)
	}
	_, leads, _ := e.do("GET", "/v1/leads", token("s", "", "sales"), nil)
	if rows, _ := leads["data"].([]any); len(rows) != 1 {
		t.Errorf("the enquiry must be stored and visible to sales, got %v", leads)
	}
	// Document issue and public verification (definer function reading the register).
	_, d, _ := e.do("POST", "/v1/documents", token("f", "", "finance"), map[string]any{"code": "INV", "title": "Owner test"})
	ref, _ := d["ref"].(string)
	if _, v, _ := e.do("GET", "/v1/public/verify/"+ref, "", nil); v["valid"] != true {
		t.Errorf("public verification as non-superuser owner: %v", v)
	}
	// Push registration (definer function with an upsert).
	body := map[string]string{"token": "ExponentPushToken[owner-" + sfx + "]", "platform": "expo"}
	if c := putJSON(e, "/v1/devices", token("dev", "", "support"), body); c != 204 {
		t.Errorf("device registration as non-superuser owner: want 204, got %d", c)
	}
}
