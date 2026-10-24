package hr

import (
	"net/http"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/internal/reports"
)

// Expense claims for the people who review and pay them. Everyone else exports only their own.
func exportClaims(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	q := `SELECT ` + claimCols + ` FROM hr.expense_claims`
	var args []any
	if !s.Claims.HasAny("hr", "manager", "ceo", "finance") {
		q += ` WHERE actor=$1`
		args = append(args, s.Claims.Sub)
	}
	rows, err := s.Tx.Query(r.Context(), q+` ORDER BY expense_date DESC LIMIT 50000`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Claim{}
	for rows.Next() {
		c, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	data, err := reports.Rows(&platform.Result{Body: map[string]any{"data": out}}, "data")
	if err != nil {
		return nil, err
	}
	return reports.CSV("expense-claims", []reports.Col{reports.C("Date", "expense_date"), reports.C("Employee", "actor"), reports.C("Category", "category"), reports.C("Description", "description"), reports.C("Amount (XAF)", "amount"), reports.C("Status", "status"), reports.C("Decided by", "decided_by"), reports.C("Paid by", "paid_by")}, data)
}
