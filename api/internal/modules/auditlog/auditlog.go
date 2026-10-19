// Package auditlog lets management and the external auditor read the audit trail and check that it
// has not been altered. Writing happens elsewhere (Scope.Audit); sealing runs in the background.
package auditlog

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

var readers = []string{"ceo", "manager", "auditor"}

func Register(a *platform.App) {
	a.Route("GET /v1/audit", readers, list)
	a.Route("GET /v1/audit/verify", readers, verify)
}

type Entry struct {
	ID       int64     `json:"id"`
	At       time.Time `json:"at"`
	TenantID *string   `json:"tenant_id"`
	Actor    string    `json:"actor"`
	Action   string    `json:"action"`
	Entity   string    `json:"entity"`
	EntityID *string   `json:"entity_id"`
	TraceID  *string   `json:"trace_id"`
	ChainSeq *int64    `json:"chain_seq"`
	Hash     *string   `json:"hash"`
}

func list(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit == 0 {
		limit = 100
	}
	var before, actor, entity any
	if v := q.Get("before"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, platform.Err(http.StatusBadRequest, "Bad request", "before must be a number")
		}
		before = n
	}
	if v := q.Get("actor"); v != "" {
		actor = v
	}
	if v := q.Get("entity"); v != "" {
		entity = v
	}
	rows, err := s.Tx.Query(r.Context(), `SELECT id, at, tenant_id::text, actor, action, entity, entity_id, trace_id, chain_seq, hash FROM platform.audit_read($1,$2,$3,$4)`, limit, before, actor, entity)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.At, &e.TenantID, &e.Actor, &e.Action, &e.Entity, &e.EntityID, &e.TraceID, &e.ChainSeq, &e.Hash); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	var next any
	if len(out) == limit && limit > 0 {
		next = out[len(out)-1].ID
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out, "next_before": next}}, rows.Err()
}

func verify(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var checked, unsealed int64
	var broken *int64
	if err := s.Tx.QueryRow(r.Context(), `SELECT checked, first_broken, unsealed FROM platform.verify_audit()`).Scan(&checked, &broken, &unsealed); err != nil {
		if strings.Contains(err.Error(), "not allowed") {
			return nil, platform.Err(http.StatusForbidden, "Forbidden", "your role cannot read the audit trail")
		}
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{
		"checked": checked, "intact": broken == nil, "first_broken_seq": broken, "waiting_to_be_sealed": unsealed,
	}}, nil
}

// Seal links new audit rows into the hash chain on a timer. Safe to run in every instance: only one
// sealer works at a time and the others return at once.
func Seal(ctx context.Context, pool *pgxpool.Pool, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			var n int
			if err := pool.QueryRow(ctx, `SELECT platform.seal_audit()`).Scan(&n); err != nil && ctx.Err() == nil {
				slog.Error("audit sealer", "err", err)
			}
		}
	}
}
