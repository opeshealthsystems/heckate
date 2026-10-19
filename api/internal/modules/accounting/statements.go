package accounting

import (
	"net/http"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

type stmtLine struct {
	Account string `json:"account"`
	Name    string `json:"name"`
	Amount  int64  `json:"amount"`
}

// statements returns an income statement for [from, to] and a balance sheet as at `to`. Both are
// computed from the ledger on the fly, so they can never disagree with it. The classification is by
// account class only (see the note in the response): the presentation required by SYSCOHADA, with
// depreciation netting, sub-totals and notes, has to be defined with the accountant.
func statements(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	fromA, toA, err := period(r)
	if err != nil {
		return nil, err
	}
	asAt := time.Now().UTC()
	if t, ok := toA.(time.Time); ok {
		asAt = t
	}
	ctx := r.Context()

	// Income statement: expenses (class 6) and revenues (class 7) inside the period.
	rows, err := s.Tx.Query(ctx, `SELECT a.code, a.name, a.class, coalesce(sum(l.debit),0), coalesce(sum(l.credit),0)
		FROM accounting.accounts a JOIN accounting.entry_lines l ON l.account = a.code JOIN accounting.entries e ON e.seq = l.entry_seq
		WHERE a.class IN (6,7) AND ($1::date IS NULL OR e.entry_date >= $1) AND e.entry_date <= $2
		GROUP BY a.code, a.name, a.class ORDER BY a.code`, fromA, asAt)
	if err != nil {
		return nil, err
	}
	revenues, expenses := []stmtLine{}, []stmtLine{}
	var totRev, totExp int64
	for rows.Next() {
		var code, name string
		var class int
		var d, c int64
		if err := rows.Scan(&code, &name, &class, &d, &c); err != nil {
			rows.Close()
			return nil, err
		}
		if class == 7 {
			revenues = append(revenues, stmtLine{code, name, c - d})
			totRev += c - d
		} else {
			expenses = append(expenses, stmtLine{code, name, d - c})
			totExp += d - c
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Balance sheet: every balance sheet account up to the date; the result of all earlier periods is
	// not closed to equity yet, so it is shown as one line and the sheet always balances.
	rows, err = s.Tx.Query(ctx, `SELECT a.code, a.name, a.class, coalesce(sum(l.debit),0) - coalesce(sum(l.credit),0)
		FROM accounting.accounts a JOIN accounting.entry_lines l ON l.account = a.code JOIN accounting.entries e ON e.seq = l.entry_seq
		WHERE e.entry_date <= $1 GROUP BY a.code, a.name, a.class ORDER BY a.code`, asAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	assets, liabilities := []stmtLine{}, []stmtLine{}
	var totA, totL, result int64
	for rows.Next() {
		var code, name string
		var class int
		var bal int64
		if err := rows.Scan(&code, &name, &class, &bal); err != nil {
			return nil, err
		}
		switch {
		case class >= 6:
			result -= bal // revenues (credit) minus expenses (debit), cumulative
		case bal > 0:
			assets = append(assets, stmtLine{code, name, bal})
			totA += bal
		case bal < 0:
			liabilities = append(liabilities, stmtLine{code, name, -bal})
			totL += -bal
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	liabilities = append(liabilities, stmtLine{"RES", "Result not yet closed to equity", result})
	totL += result

	return &platform.Result{Status: http.StatusOK, Body: map[string]any{
		"income_statement": map[string]any{"revenues": revenues, "expenses": expenses, "total_revenues": totRev, "total_expenses": totExp, "result": totRev - totExp},
		"balance_sheet":    map[string]any{"as_at": asAt.Format("2006-01-02"), "assets": assets, "liabilities_and_equity": liabilities, "total_assets": totA, "total_liabilities_and_equity": totL, "balanced": totA == totL},
		"note":             "Classified by account class only. The SYSCOHADA presentation (netting of depreciation, sub-totals, notes) must be defined with the accountant before these are filed.",
	}}, nil
}
