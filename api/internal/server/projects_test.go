package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func pngURL() string {
	b := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, 64)...)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
}

func TestFieldInterventionFlow(t *testing.T) {
	e := setup(t)
	mgr := token("mgr-f", "", "manager")
	disp := token("dispatch", "", "support")
	eng1 := token("eng-1", "", "engineer")
	eng2 := token("eng-2", "", "engineer")
	ta := e.newClient(mgr, "Field A "+t.Name())
	tb := e.newClient(mgr, "Field B "+t.Name())
	clientA := token("fa", ta, "client_user")
	clientB := token("fb", tb, "client_user")

	if code, _, _ := e.do("POST", "/v1/interventions", eng1, map[string]any{"tenant_id": ta, "engineer": "eng-1"}); code != 403 {
		t.Errorf("engineer cannot dispatch, got %d", code)
	}
	code, iv, _ := e.do("POST", "/v1/interventions", disp, map[string]any{"tenant_id": ta, "engineer": "eng-1", "summary": "Replace PSU"})
	if code != 201 || iv["status"] != "scheduled" {
		t.Fatalf("dispatch: %d %v", code, iv)
	}
	id := iv["id"].(string)
	if code, _, _ := e.do("GET", "/v1/interventions/"+id, clientA, nil); code != 404 {
		t.Errorf("client must not see an unfinished intervention, got %d", code)
	}

	// Only the assigned engineer can act; inputs are validated.
	if code, _, _ := e.do("POST", "/v1/interventions/"+id+"/check-in", eng2, map[string]any{"lat": 4.05, "lng": 9.7}); code != 403 {
		t.Errorf("other engineer check-in: want 403, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/interventions/"+id+"/check-in", eng1, map[string]any{"lat": 95, "lng": 9.7}); code != 422 {
		t.Errorf("bad latitude: want 422, got %d", code)
	}
	future := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	if code, _, _ := e.do("POST", "/v1/interventions/"+id+"/check-in", eng1, map[string]any{"lat": 4.05, "lng": 9.7, "at": future}); code != 422 {
		t.Errorf("future time: want 422, got %d", code)
	}
	if code, _, _ := e.do("POST", "/v1/interventions/"+id+"/check-out", eng1, map[string]any{"lat": 4.05, "lng": 9.7, "report": "Replaced the faulty PSU"}); code != 409 {
		t.Errorf("check-out before check-in: want 409, got %d", code)
	}
	// Offline work keeps its real time.
	started := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
	code, b, _ := e.do("POST", "/v1/interventions/"+id+"/check-in", eng1, map[string]any{"lat": 4.05, "lng": 9.7, "at": started.Format(time.RFC3339)})
	if code != 200 || b["status"] != "in_progress" || b["checkin_at"] != started.Format(time.RFC3339) {
		t.Fatalf("check-in: %d %v", code, b)
	}
	if code, _, _ := e.do("POST", "/v1/interventions/"+id+"/check-in", eng1, map[string]any{"lat": 4.05, "lng": 9.7}); code != 409 {
		t.Errorf("double check-in: want 409, got %d", code)
	}

	out := func(m map[string]any) int {
		base := map[string]any{"lat": 4.05, "lng": 9.7, "report": "Replaced the faulty PSU and tested"}
		for k, v := range m {
			base[k] = v
		}
		c, _, _ := e.do("POST", "/v1/interventions/"+id+"/check-out", eng1, base)
		return c
	}
	if c := out(map[string]any{"report": "short"}); c != 422 {
		t.Errorf("short report: want 422, got %d", c)
	}
	if c := out(map[string]any{"signed_by": "Client Rep"}); c != 422 {
		t.Errorf("name without signature: want 422, got %d", c)
	}
	if c := out(map[string]any{"signed_by": "X", "signature": "data:image/svg+xml;base64,PHN2Zy8+"}); c != 422 {
		t.Errorf("svg signature: want 422, got %d", c)
	}
	if c := out(map[string]any{"at": started.Add(-time.Hour).Format(time.RFC3339)}); c != 422 {
		t.Errorf("check-out before check-in time: want 422, got %d", c)
	}
	code, done, _ := e.do("POST", "/v1/interventions/"+id+"/check-out", eng1, map[string]any{"lat": 4.06, "lng": 9.71, "report": "Replaced the faulty PSU and tested", "signed_by": "Client Rep", "signature": pngURL()})
	ref, _ := done["document_ref"].(string)
	if code != 200 || done["status"] != "completed" || !strings.HasPrefix(ref, "HS-FRM-27-") || done["has_signature"] != true {
		t.Fatalf("check-out: %d %v", code, done)
	}
	if c := out(nil); c != 409 {
		t.Errorf("second check-out: want 409, got %d", c)
	}
	// The acceptance document is publicly verifiable.
	if _, v, _ := e.do("GET", "/v1/public/verify/"+ref, "", nil); v["valid"] != true {
		t.Errorf("acceptance document must verify: %v", v)
	}

	// Client sees the completed record with the signature; another client does not.
	code, seen, _ := e.do("GET", "/v1/interventions/"+id, clientA, nil)
	if code != 200 || seen["signature"] == nil || seen["report"] == nil {
		t.Errorf("client A should see the completed report: %d", code)
	}
	if code, _, _ := e.do("GET", "/v1/interventions/"+id, clientB, nil); code != 404 {
		t.Errorf("client B must not see it, got %d", code)
	}
	if _, l, _ := e.do("GET", "/v1/interventions", eng2, nil); len(l["data"].([]any)) != 0 {
		for _, x := range l["data"].([]any) {
			if x.(map[string]any)["engineer"] != "eng-2" {
				t.Errorf("engineer must only list their own interventions")
			}
		}
	}
	// Signed records are frozen in the database.
	conn, err := pgx.Connect(context.Background(), adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(context.Background(), `UPDATE projects.interventions SET report='edited' WHERE id=$1`, id); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Errorf("completed intervention must be immutable, got %v", err)
	}
}

func TestTimesheets(t *testing.T) {
	e := setup(t)
	run := fmt.Sprint(time.Now().UnixNano()) // a fresh engineer each run, so the 24-hour-a-day limit starts clean on a reused database
	eng := token("ts-eng-"+run, "", "engineer")
	mgr := token("ts-mgr-"+run, "", "manager")
	hr := token("ts-hr", "", "hr")
	other := token("ts-other-"+run, "", "engineer")
	day := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

	code, ts, _ := e.do("POST", "/v1/timesheets", eng, map[string]any{"work_date": day, "hours": 8, "note": "On site"})
	if code != 201 || ts["status"] != "draft" {
		t.Fatalf("create: %d %v", code, ts)
	}
	id := ts["id"].(string)
	for name, body := range map[string]map[string]any{
		"zero hours":   {"work_date": day, "hours": 0},
		"25 hours":     {"work_date": day, "hours": 25},
		"future":       {"work_date": "2099-01-01", "hours": 1},
		"bad date":     {"work_date": "x", "hours": 1},
		"over the day": {"work_date": day, "hours": 17},
	} {
		if c, _, _ := e.do("POST", "/v1/timesheets", eng, body); c != 422 {
			t.Errorf("%s: want 422, got %d", name, c)
		}
	}
	if c, _, _ := e.do("POST", "/v1/timesheets/"+id+"/submit", other, nil); c != 409 {
		t.Errorf("submitting someone else's: want 409, got %d", c)
	}
	if c, b, _ := e.do("POST", "/v1/timesheets/"+id+"/submit", eng, nil); c != 200 || b["status"] != "submitted" {
		t.Fatalf("submit: %d %v", c, b)
	}
	if c, _, _ := e.do("POST", "/v1/timesheets/"+id+"/approve", eng, nil); c != 403 {
		t.Errorf("engineer cannot approve: want 403, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/timesheets/"+id+"/approve", hr, nil); c != 403 {
		t.Errorf("hr cannot approve: want 403, got %d", c)
	}
	// A manager's own timesheet cannot be approved by themselves.
	_, own, _ := e.do("POST", "/v1/timesheets", mgr, map[string]any{"work_date": day, "hours": 4})
	e.do("POST", "/v1/timesheets/"+own["id"].(string)+"/submit", mgr, nil)
	if c, _, _ := e.do("POST", "/v1/timesheets/"+own["id"].(string)+"/approve", mgr, nil); c != 403 {
		t.Errorf("self approval: want 403, got %d", c)
	}
	if c, b, _ := e.do("POST", "/v1/timesheets/"+id+"/approve", mgr, nil); c != 200 || b["approved_by"] != "ts-mgr-"+run {
		t.Fatalf("approve: %d %v", c, b)
	}
	// Visibility: own only, unless a reviewer asks for all.
	_, mine, _ := e.do("GET", "/v1/timesheets?all=1", other, nil)
	for _, x := range mine["data"].([]any) {
		if x.(map[string]any)["actor"] != "ts-other" {
			t.Errorf("a non-reviewer must only see their own timesheets")
		}
	}
	_, all, _ := e.do("GET", "/v1/timesheets?all=1", hr, nil)
	seen := false
	for _, x := range all["data"].([]any) {
		if x.(map[string]any)["id"] == id {
			seen = true
		}
	}
	if !seen {
		t.Error("hr should see everyone's timesheets with all=1")
	}
	conn, err := pgx.Connect(context.Background(), adminDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(context.Background(), `UPDATE projects.timesheets SET hours = 1 WHERE id=$1`, id); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Errorf("approved timesheet must be immutable, got %v", err)
	}
}

func TestProjectsAndTasks(t *testing.T) {
	e := setup(t)
	mgr := token("pm", "", "manager")
	eng := token("pe", "", "engineer")
	ta := e.newClient(mgr, "ProjA "+t.Name())
	tb := e.newClient(mgr, "ProjB "+t.Name())
	if c, _, _ := e.do("POST", "/v1/projects", eng, map[string]any{"tenant_id": ta, "name": "x"}); c != 403 {
		t.Errorf("engineer cannot create a project, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/projects", mgr, map[string]any{"tenant_id": ta, "name": "x", "start_date": "2026-05-01", "end_date": "2026-04-01"}); c != 422 {
		t.Errorf("end before start: want 422, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/projects", mgr, map[string]any{"tenant_id": ta, "name": "x", "budget": -5}); c != 422 {
		t.Errorf("negative budget: want 422, got %d", c)
	}
	code, p, _ := e.do("POST", "/v1/projects", mgr, map[string]any{"tenant_id": ta, "name": "SOC rollout", "budget": 2_000_000})
	if code != 201 {
		t.Fatalf("create project: %d %v", code, p)
	}
	pid := p["id"].(string)
	code, tk, _ := e.do("POST", "/v1/projects/"+pid+"/tasks", eng, map[string]any{"title": "Install sensors"})
	if code != 201 {
		t.Fatalf("task: %d %v", code, tk)
	}
	if c, b, _ := e.do("PATCH", "/v1/tasks/"+tk["id"].(string), eng, map[string]any{"status": "done"}); c != 200 || b["status"] != "done" {
		t.Errorf("update task: %d %v", c, b)
	}
	if c, _, _ := e.do("PATCH", "/v1/tasks/"+tk["id"].(string), eng, map[string]any{"status": "weird"}); c != 422 {
		t.Errorf("bad task status: want 422, got %d", c)
	}
	if c, _, _ := e.do("POST", "/v1/projects/00000000-0000-0000-0000-000000000000/tasks", eng, map[string]any{"title": "x"}); c != 404 {
		t.Errorf("task on missing project: want 404, got %d", c)
	}
	if c, b, _ := e.do("GET", "/v1/projects/"+pid, token("ca", ta, "client_user"), nil); c != 200 || len(b["tasks"].([]any)) != 1 {
		t.Errorf("client sees own project and tasks: %d %v", c, b)
	}
	if c, _, _ := e.do("GET", "/v1/projects/"+pid, token("cb", tb, "client_user"), nil); c != 404 {
		t.Errorf("other client must not see the project, got %d", c)
	}
}
