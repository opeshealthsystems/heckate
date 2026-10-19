package projects

import (
	"net/http"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/internal/reports"
)

// Timesheets: reviewers export everyone's, everyone else their own.
func exportTimesheets(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	q := `SELECT ` + tsCols + ` FROM projects.timesheets`
	var args []any
	if !s.Claims.HasAny("ceo", "manager", "hr", "finance") {
		q += ` WHERE actor = $1`
		args = append(args, s.Claims.Sub)
	}
	rows, err := s.Tx.Query(r.Context(), q+` ORDER BY work_date DESC, id LIMIT 50000`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Timesheet{}
	for rows.Next() {
		t, err := scanTS(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	data, err := reports.Rows(&platform.Result{Body: map[string]any{"data": out}}, "data")
	if err != nil {
		return nil, err
	}
	return reports.CSV("timesheets", []reports.Col{reports.C("Date", "work_date"), reports.C("Employee", "actor"), reports.C("Hours", "hours"), reports.C("Project id", "project_id"), reports.C("Ticket id", "ticket_id"), reports.C("Note", "note"), reports.C("Status", "status"), reports.C("Approved by", "approved_by")}, data)
}
