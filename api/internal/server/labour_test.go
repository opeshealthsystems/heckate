package server

import (
	"fmt"
	"testing"
	"time"
)

func TestLabourFromApprovedTimesheets(t *testing.T) {
	e := setup(t)
	sfx := fmt.Sprint(time.Now().UnixNano())
	mgr := token("lb2-mgr", "", "manager")
	fin := token("lb2-fin", "", "finance")
	eng := token("lb2-eng", "", "engineer")
	hr := token("lb2-hr", "", "hr")
	w1, w2 := "lb2-w1-"+sfx, "lb2-w2-"+sfx
	ta := e.newClient(mgr, "Labour Co "+sfx)
	client := token("lb2-c", ta, "client_admin")
	_, p, _ := e.do("POST", "/v1/projects", mgr, map[string]any{"tenant_id": ta, "name": "Labour " + sfx, "budget": 1_000_000})
	pid := p["id"].(string)

	// Rates: only ceo/manager/finance read or write; nobody else, including HR, engineers and clients.
	rate := func(actor, from string, r int) map[string]any {
		return map[string]any{"actor": actor, "effective_from": from, "rate": r}
	}
	for _, tok := range []string{eng, hr, client} {
		if c, _, _ := e.do("PUT", "/v1/labour-rates", tok, rate(w1, day(-30), 1000)); c != 403 {
			t.Errorf("setting a rate without the role: want 403, got %d", c)
		}
		if c, _, _ := e.do("GET", "/v1/labour-rates", tok, nil); c != 403 {
			t.Errorf("reading rates without the role: want 403, got %d", c)
		}
	}
	if c, _, _ := e.do("PUT", "/v1/labour-rates", fin, rate(w1, day(-30), 0)); c != 422 {
		t.Errorf("zero rate: want 422, got %d", c)
	}

	// Two employees log approved hours; w2 has no rate at all.
	log := func(actor string, offset int, hours float64) {
		tok := token(actor, "", "engineer")
		c, ts, _ := e.do("POST", "/v1/timesheets", tok, map[string]any{"work_date": day(offset), "hours": hours, "project_id": pid})
		if c != 201 {
			t.Fatalf("timesheet: %d %v", c, ts)
		}
		id := ts["id"].(string)
		e.do("POST", "/v1/timesheets/"+id+"/submit", tok, nil)
		if c, b, _ := e.do("POST", "/v1/timesheets/"+id+"/approve", mgr, nil); c != 200 {
			t.Fatalf("approve: %d %v", c, b)
		}
	}
	// A draft that is never approved must not count.
	e.do("POST", "/v1/timesheets", token(w1, "", "engineer"), map[string]any{"work_date": day(-1), "hours": 5, "project_id": pid})

	log(w1, -20, 8) // before the second rate
	log(w1, -5, 4)  // after it
	log(w2, -5, 3)  // unpriced

	progress := func(tok string) map[string]any {
		_, pr, _ := e.do("GET", "/v1/projects/"+pid+"/progress", tok, nil)
		return pr
	}
	// No rates yet: nothing is guessed.
	pr := progress(fin)
	lab := pr["labour"].(map[string]any)
	if lab["cost"].(float64) != 0 || lab["priced_hours"].(float64) != 0 || lab["unpriced_hours"].(float64) != 15 || pr["actual"].(float64) != 0 {
		t.Fatalf("with no rates: %v", pr)
	}

	if c, b, _ := e.do("PUT", "/v1/labour-rates", fin, rate(w1, day(-40), 1000)); c != 200 {
		t.Fatalf("set rate: %d %v", c, b)
	}
	if c, _, _ := e.do("PUT", "/v1/labour-rates", mgr, rate(w1, day(-10), 2000)); c != 200 {
		t.Fatalf("set second rate: %d", c)
	}
	_, rates, _ := e.do("GET", "/v1/labour-rates", fin, nil)
	n := 0
	for _, x := range listOf(t, rates) {
		if x.(map[string]any)["actor"] == w1 {
			n++
		}
	}
	if n != 2 {
		t.Errorf("rates listed for %s: %d", w1, n)
	}

	e.do("POST", "/v1/projects/"+pid+"/costs", fin, map[string]any{"description": "Racks", "category": "materials", "amount": 100_000})
	pr = progress(mgr)
	lab = pr["labour"].(map[string]any)
	// 8h x 1000 (the rate in force on that date) + 4h x 2000 = 16000; w2's 3h are unpriced.
	if lab["cost"].(float64) != 16_000 || lab["priced_hours"].(float64) != 12 || lab["unpriced_hours"].(float64) != 3 {
		t.Errorf("labour: %v", lab)
	}
	if pr["actual"].(float64) != 116_000 || pr["remaining"].(float64) != 884_000 {
		t.Errorf("actual includes labour and manual costs: %v", pr)
	}

	// Setting the same date again corrects it.
	e.do("PUT", "/v1/labour-rates", fin, rate(w1, day(-10), 3000))
	if lab := progress(fin)["labour"].(map[string]any); lab["cost"].(float64) != 20_000 {
		t.Errorf("after correction: %v", lab)
	}

	// The portfolio agrees; clients and engineers never see money.
	_, pf, _ := e.do("GET", "/v1/projects/portfolio", mgr, nil)
	ok := false
	for _, x := range listOf(t, pf) {
		if m := x.(map[string]any); m["project_id"] == pid {
			ok = m["actual"].(float64) == 120_000
		}
	}
	if !ok {
		t.Errorf("portfolio actual should include labour: %v", pf)
	}
	cp := progress(client)
	for _, k := range []string{"labour", "actual", "budget"} {
		if _, has := cp[k]; has {
			t.Errorf("the client must not see %s", k)
		}
	}
	if ep := progress(eng); ep["labour"] != nil {
		t.Errorf("engineer must not see labour: %v", ep["labour"])
	}
}
