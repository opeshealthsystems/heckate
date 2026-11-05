package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/modules/sales"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// SignatureHeader carries "sha256=" + hex(HMAC-SHA256(secret, body)) on provider callbacks.
const SignatureHeader = "X-Hekate-Signature"

// pendingTTL is how long an unanswered request keeps a new attempt on the same invoice blocked.
const pendingTTL = 15 * time.Minute

type mod struct {
	gw     map[string]Gateway
	rec    sales.Recorder
	secret string
}

func Register(a *platform.App, rec sales.Recorder, cfg platform.Config) {
	m := &mod{gw: Gateways(cfg), rec: rec, secret: cfg.PaymentsWebhookSecret}
	payers := platform.ClientAll
	watchers := platform.Join([]string{"ceo", "manager", "finance"}, platform.ClientAll) // finance follows payments in progress
	a.Route("GET /v1/payment-providers", payers, m.providers)
	a.Route("POST /v1/invoices/{id}/online-payments", payers, platform.Idempotent(m.start))
	a.Route("GET /v1/invoices/{id}/online-payments", watchers, m.list)
	a.Route("POST /v1/online-payments/{id}/refresh", watchers, m.refresh)
	a.Public("POST /v1/webhooks/payments/{provider}", m.webhook)
}

type Intent struct {
	ID          string    `json:"id"`
	InvoiceID   string    `json:"invoice_id"`
	Amount      int64     `json:"amount"`
	Provider    string    `json:"provider"`
	MSISDN      string    `json:"msisdn"`
	Status      string    `json:"status"`
	ProviderRef *string   `json:"provider_ref"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

const intentCols = `id::text, invoice_id::text, amount, provider, msisdn, status, provider_ref, reason, created_at`

func scanIntent(row pgx.Row) (*Intent, error) {
	var x Intent
	err := row.Scan(&x.ID, &x.InvoiceID, &x.Amount, &x.Provider, &x.MSISDN, &x.Status, &x.ProviderRef, &x.Reason, &x.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) || (err != nil && strings.Contains(err.Error(), "invalid input syntax")) {
		return nil, platform.Err(http.StatusNotFound, "Not found", "payment not found")
	}
	return &x, err
}

func (m *mod) providers(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	out := []string{}
	for _, name := range []string{"momo", "orange_money"} {
		if _, ok := m.gw[name]; ok {
			out = append(out, name)
		}
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, nil
}

func (m *mod) start(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Provider string `json:"provider"`
		MSISDN   string `json:"msisdn"`
		Amount   int64  `json:"amount"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	gw, ok := m.gw[in.Provider]
	if !ok {
		return nil, platform.Err(http.StatusServiceUnavailable, "Not available", "online payment with "+in.Provider+" is not available")
	}
	in.MSISDN = strings.TrimPrefix(strings.NewReplacer(" ", "", "-", "").Replace(in.MSISDN), "+")
	if len(in.MSISDN) < 8 || len(in.MSISDN) > 15 || strings.Trim(in.MSISDN, "0123456789") != "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid number", "enter the mobile money number with country code, for example 2376XXXXXXXX")
	}
	ctx := r.Context()
	id := r.PathValue("id")
	status, balance, tenant, err := m.rec.Outstanding(ctx, s, id)
	if err != nil {
		return nil, err
	}
	if status != "issued" || balance <= 0 {
		return nil, platform.Err(http.StatusConflict, "Nothing to pay", "this invoice has no outstanding balance")
	}
	if in.Amount == 0 {
		in.Amount = balance
	}
	if in.Amount < 0 || in.Amount > balance {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid amount", "amount must be positive and not exceed the outstanding balance")
	}
	// A request nobody answered must not block the invoice forever.
	if err := s.AsSystem(ctx, "system:payments", func() error {
		_, err := s.Tx.Exec(ctx, `UPDATE payments.intents SET status='expired', reason='no answer from the provider' WHERE invoice_id=$1 AND status='pending' AND created_at < now() - $2::interval`, id, pendingTTL.String())
		return err
	}); err != nil {
		return nil, err
	}
	var intentID string
	if err := s.Tx.QueryRow(ctx, `INSERT INTO payments.intents (tenant_id, invoice_id, amount, provider, msisdn, created_by) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id::text`,
		tenant, id, in.Amount, in.Provider, in.MSISDN, s.Claims.Sub).Scan(&intentID); err != nil {
		if strings.Contains(err.Error(), "one_pending_per_invoice") {
			return nil, platform.Err(http.StatusConflict, "Payment in progress", "a payment request for this invoice is already waiting for approval on the phone")
		}
		return nil, err
	}
	ref, err := gw.Charge(ctx, ChargeRequest{Reference: intentID, Amount: in.Amount, MSISDN: in.MSISDN, Message: "HEKATE invoice"})
	if err != nil {
		return nil, platform.Err(http.StatusBadGateway, "Provider unavailable", "the payment provider did not accept the request; nothing was charged")
	}
	if err := s.AsSystem(ctx, "system:payments", func() error {
		_, err := s.Tx.Exec(ctx, `UPDATE payments.intents SET provider_ref=$2 WHERE id=$1`, intentID, ref)
		return err
	}); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "start", "online_payment", intentID); err != nil {
		return nil, err
	}
	it, err := scanIntent(s.Tx.QueryRow(ctx, `SELECT `+intentCols+` FROM payments.intents WHERE id=$1`, intentID))
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusAccepted, Body: it}, nil
}

func (m *mod) list(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+intentCols+` FROM payments.intents WHERE invoice_id::text=$1 ORDER BY created_at DESC LIMIT 50`, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Intent{}
	for rows.Next() {
		x, err := scanIntent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

// refresh asks the provider what became of a request and settles it if the answer is final.
func (m *mod) refresh(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	it, err := scanIntent(s.Tx.QueryRow(ctx, `SELECT `+intentCols+` FROM payments.intents WHERE id::text=$1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	if it.Status == "pending" || it.Status == "expired" {
		gw, ok := m.gw[it.Provider]
		if !ok || it.ProviderRef == nil {
			return nil, platform.Err(http.StatusServiceUnavailable, "Not available", "this provider is not available")
		}
		st, err := gw.Status(ctx, *it.ProviderRef)
		if err != nil {
			return nil, platform.Err(http.StatusBadGateway, "Provider unavailable", "could not reach the payment provider; try again shortly")
		}
		if st.Status != "pending" {
			if err := m.settle(ctx, s, it.ID, st); err != nil {
				return nil, err
			}
		}
	}
	out, err := scanIntent(s.Tx.QueryRow(ctx, `SELECT `+intentCols+` FROM payments.intents WHERE id::text=$1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}

// settle applies a final provider answer to an intent, exactly once. A paid amount that differs from
// the request is never booked: the intent goes to review for finance to look at.
func (m *mod) settle(ctx context.Context, s *platform.Scope, intentID string, st State) error {
	return s.AsSystem(ctx, "system:payments", func() error {
		var invoice, provider, status string
		var amount int64
		var ref *string
		if err := s.Tx.QueryRow(ctx, `SELECT invoice_id::text, amount, provider, status, provider_ref FROM payments.intents WHERE id=$1 FOR UPDATE`, intentID).Scan(&invoice, &amount, &provider, &status, &ref); err != nil {
			return err
		}
		if status != "pending" && status != "expired" {
			return nil // already settled: the second answer changes nothing
		}
		set := func(newStatus, reason string) error {
			_, err := s.Tx.Exec(ctx, `UPDATE payments.intents SET status=$2, reason=$3 WHERE id=$1`, intentID, newStatus, reason)
			return err
		}
		switch {
		case st.Status == "failed":
			if err := set("failed", st.Reason); err != nil {
				return err
			}
			return s.Emit(ctx, "payment.online_failed", map[string]any{"intent_id": intentID, "invoice_id": invoice})
		case st.Status == "succeeded" && st.Amount != 0 && st.Amount != amount:
			return set("review", "provider reports a different amount than requested")
		case st.Status == "succeeded":
			refStr := ""
			if ref != nil {
				refStr = *ref
			}
			if err := m.rec.RecordPayment(ctx, s, invoice, amount, provider, refStr); err != nil {
				// e.g. the invoice was paid another way meanwhile: keep the money visible for finance.
				var he *platform.HTTPError
				if errors.As(err, &he) {
					return set("review", he.Title+": "+he.Detail)
				}
				return err
			}
			if err := set("succeeded", ""); err != nil {
				return err
			}
			return s.Emit(ctx, "payment.online_succeeded", map[string]any{"intent_id": intentID, "invoice_id": invoice, "amount": amount})
		}
		return nil
	})
}

// webhook receives a provider's final answer. The signature is checked on the raw body before anything
// else is read; an unsigned or wrongly signed call changes nothing.
func (m *mod) webhook(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	if m.secret == "" {
		return nil, platform.Err(http.StatusServiceUnavailable, "Not available", "payment callbacks are not configured")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		return nil, platform.Err(http.StatusBadRequest, "Bad request", "unreadable body")
	}
	mac := hmac.New(sha256.New, []byte(m.secret))
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(r.Header.Get(SignatureHeader))) {
		return nil, platform.Err(http.StatusUnauthorized, "Unauthorized", "bad signature")
	}
	provider := r.PathValue("provider")
	if _, ok := m.gw[provider]; !ok {
		return nil, platform.Err(http.StatusNotFound, "Not found", "unknown provider")
	}
	var ev struct {
		EventID   string `json:"event_id"`
		Reference string `json:"reference"`
		Status    string `json:"status"`
		Amount    int64  `json:"amount"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal(body, &ev); err != nil || ev.EventID == "" || ev.Reference == "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid event", "event_id and reference are required")
	}
	state := map[string]string{"succeeded": "succeeded", "successful": "succeeded", "failed": "failed"}[strings.ToLower(ev.Status)]
	if state == "" {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Invalid event", "status must be succeeded or failed")
	}
	ctx := r.Context()
	err = s.AsSystem(ctx, "gateway:"+provider, func() error {
		tag, err := s.Tx.Exec(ctx, `INSERT INTO payments.events (provider, event_id, intent_id, outcome, payload)
			SELECT $1, $2, i.id, $3, $4 FROM (SELECT $5::text AS ref) q LEFT JOIN payments.intents i ON i.id::text = q.ref AND i.provider = $1
			ON CONFLICT (provider, event_id) DO NOTHING`, provider, ev.EventID, state, body, ev.Reference)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil // a repeated delivery
		}
		var known bool
		if err := s.Tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM payments.intents WHERE id::text=$1 AND provider=$2)`, ev.Reference, provider).Scan(&known); err != nil {
			return err
		}
		if !known {
			return nil // logged for finance to look at; nothing to settle
		}
		return m.settle(ctx, s, ev.Reference, State{Status: state, Amount: ev.Amount, Reason: ev.Reason})
	})
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]string{"status": "ok"}}, nil
}
