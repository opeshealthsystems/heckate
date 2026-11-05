package accounting

import (
	"net/http"
	"strings"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

var closers = []string{"ceo", "manager"}

// closePeriod locks a finished month. Only management can do it, only for a month that has ended,
// and it cannot be undone.
func closePeriod(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Period string `json:"period"`
		Note   string `json:"note"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	p, err := time.Parse("2006-01", in.Period)
	if err != nil {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid period", "period must be YYYY-MM")
	}
	now := time.Now().UTC()
	if !p.Before(time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)) {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Month not over", "only a month that has ended can be closed")
	}
	ctx := r.Context()
	// Serialise with posting, so no entry slips into the month while it is being closed.
	if _, err := s.Tx.Exec(ctx, `SELECT pg_advisory_xact_lock(727275)`); err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `INSERT INTO accounting.closed_periods (period, closed_by, note) VALUES ($1,$2,$3)`, p, s.Claims.Sub, strings.TrimSpace(in.Note)); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return nil, platform.Err(http.StatusConflict, "Already closed", "this period is already closed")
		}
		if strings.Contains(err.Error(), "row-level security") {
			return nil, platform.Err(http.StatusForbidden, "Forbidden", "only management can close a period")
		}
		return nil, err
	}
	if err := s.Audit(ctx, "close", "accounting_period", in.Period); err != nil {
		return nil, err
	}
	if err := s.Emit(ctx, "accounting.period.closed", map[string]any{"period": in.Period}); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: map[string]any{"period": in.Period, "closed_by": s.Claims.Sub}}, nil
}

func listClosed(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT to_char(period,'YYYY-MM'), closed_by, closed_at, note FROM accounting.closed_periods ORDER BY period DESC LIMIT 240`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var p, by, note string
		var at time.Time
		if err := rows.Scan(&p, &by, &at, &note); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"period": p, "closed_by": by, "closed_at": at, "note": note})
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}
