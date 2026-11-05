package payroll

import (
	"net/http"
	"strings"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/internal/reports"
)

func registerExports(a *platform.App) {
	a.Route("GET /v1/exports/payroll", viewers, exportPayroll)
}

// exportPayroll is the payroll register of one run (?run_id=): one row per employee, amounts in whole XAF.
func exportPayroll(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	run := r.URL.Query().Get("run_id")
	if run == "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid request", "run_id is required")
	}
	rows, err := s.Tx.Query(r.Context(), `SELECT to_char(p.period,'YYYY-MM'), p.full_name, p.actor, p.gross, p.irpp, p.cac, p.net, p.employer_cost,
		COALESCE((SELECT sum((d->>'amount')::bigint) FROM jsonb_array_elements(p.deductions) d),0)
		FROM payroll.payslips p WHERE p.run_id::text=$1 ORDER BY p.full_name`, run)
	if err != nil {
		if isInvalid(err) {
			return nil, platform.Err(http.StatusNotFound, "Not found", "run not found")
		}
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var period, name, actor string
		var gross, irpp, cac, net, cost, ded int64
		if err := rows.Scan(&period, &name, &actor, &gross, &irpp, &cac, &net, &cost, &ded); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"period": period, "full_name": name, "actor": actor, "gross": gross, "deductions": ded, "irpp": irpp, "cac": cac, "net": net, "employer_cost": cost})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return reports.CSV("payroll-register", []reports.Col{reports.C("Period", "period"), reports.C("Employee", "full_name"), reports.C("Account", "actor"), reports.C("Gross", "gross"),
		reports.C("Social deductions", "deductions"), reports.C("IRPP", "irpp"), reports.C("CAC", "cac"), reports.C("Net pay", "net"), reports.C("Employer cost", "employer_cost")}, out)
}

func isInvalid(err error) bool {
	return err != nil && strings.Contains(err.Error(), "invalid input syntax")
}
