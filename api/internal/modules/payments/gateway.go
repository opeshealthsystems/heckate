// Package payments takes online mobile-money payments for issued invoices. Providers sit behind a
// Gateway; a payment only counts once the provider confirms it (a signed callback or a status lookup).
package payments

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

type ChargeRequest struct {
	Reference string // our intent id; it is also the idempotency reference at the provider
	Amount    int64
	MSISDN    string
	Message   string
}

type State struct {
	Status string // pending, succeeded or failed
	Amount int64  // what the provider says it collected; 0 when it does not say
	Reason string
}

// Gateway is one payment provider.
type Gateway interface {
	Name() string // matches the method recorded on the payment: momo or orange_money
	Charge(ctx context.Context, c ChargeRequest) (providerRef string, err error)
	Status(ctx context.Context, providerRef string) (State, error)
}

// ErrProvider means the provider could not be reached or refused the request.
var ErrProvider = errors.New("payment provider unavailable")

// Gateways builds the providers the configuration enables.
func Gateways(cfg platform.Config) map[string]Gateway {
	out := map[string]Gateway{}
	if cfg.PaymentsSandbox {
		out["momo"] = Sandbox{name: "momo"}
		out["orange_money"] = Sandbox{name: "orange_money"}
	}
	if m := cfg.MoMo; m.BaseURL != "" && m.SubscriptionKey != "" && m.APIUser != "" && m.APIKey != "" {
		out["momo"] = NewMoMo(m, nil)
	}
	return out
}

// Sandbox pretends to be a provider: it accepts every charge and never settles on its own. Tests and
// demos settle it by posting a signed callback. It never moves money.
type Sandbox struct{ name string }

func (s Sandbox) Name() string { return s.name }
func (s Sandbox) Charge(_ context.Context, c ChargeRequest) (string, error) {
	return "sbx-" + c.Reference, nil
}
func (Sandbox) Status(context.Context, string) (State, error) { return State{Status: "pending"}, nil }

// MoMo is the MTN Mobile Money Collection API ("request to pay").
type MoMo struct {
	cfg  platform.MoMoConfig
	http *http.Client
	mu   sync.Mutex
	tok  string
	exp  time.Time
}

func NewMoMo(cfg platform.MoMoConfig, c *http.Client) *MoMo {
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}
	if cfg.Currency == "" {
		cfg.Currency = "XAF"
	}
	return &MoMo{cfg: cfg, http: c}
}

func (m *MoMo) Name() string { return "momo" }

func (m *MoMo) token(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tok != "" && time.Now().Before(m.exp) {
		return m.tok, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(m.cfg.BaseURL, "/")+"/collection/token/", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(m.cfg.APIUser+":"+m.cfg.APIKey)))
	req.Header.Set("Ocp-Apim-Subscription-Key", m.cfg.SubscriptionKey)
	resp, err := m.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrProvider, err)
	}
	defer resp.Body.Close()
	var t struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&t) != nil || t.AccessToken == "" {
		return "", fmt.Errorf("%w: token request answered %d", ErrProvider, resp.StatusCode)
	}
	m.tok, m.exp = t.AccessToken, time.Now().Add(time.Duration(t.ExpiresIn-60)*time.Second)
	return m.tok, nil
}

func (m *MoMo) do(ctx context.Context, method, path, ref string, body any) (*http.Response, error) {
	tok, err := m.token(ctx)
	if err != nil {
		return nil, err
	}
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequestWithContext(ctx, method, strings.TrimRight(m.cfg.BaseURL, "/")+path, rd)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Target-Environment", m.cfg.Environment)
	req.Header.Set("Ocp-Apim-Subscription-Key", m.cfg.SubscriptionKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Reference-Id", ref)
		if m.cfg.CallbackURL != "" {
			req.Header.Set("X-Callback-Url", m.cfg.CallbackURL)
		}
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProvider, err)
	}
	return resp, nil
}

func (m *MoMo) Charge(ctx context.Context, c ChargeRequest) (string, error) {
	resp, err := m.do(ctx, http.MethodPost, "/collection/v1_0/requesttopay", c.Reference, map[string]any{
		"amount": strconv.FormatInt(c.Amount, 10), "currency": m.cfg.Currency, "externalId": c.Reference,
		"payer":        map[string]string{"partyIdType": "MSISDN", "partyId": c.MSISDN},
		"payerMessage": c.Message, "payeeNote": c.Message,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("%w: request to pay answered %d", ErrProvider, resp.StatusCode)
	}
	return c.Reference, nil
}

func (m *MoMo) Status(ctx context.Context, ref string) (State, error) {
	resp, err := m.do(ctx, http.MethodGet, "/collection/v1_0/requesttopay/"+ref, "", nil)
	if err != nil {
		return State{}, err
	}
	defer resp.Body.Close()
	var r struct {
		Status string `json:"status"`
		Amount string `json:"amount"`
		Reason any    `json:"reason"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&r) != nil {
		return State{}, fmt.Errorf("%w: status answered %d", ErrProvider, resp.StatusCode)
	}
	amt, _ := strconv.ParseInt(strings.Split(r.Amount, ".")[0], 10, 64)
	st := State{Amount: amt}
	if r.Reason != nil {
		st.Reason = fmt.Sprint(r.Reason)
	}
	switch strings.ToUpper(r.Status) {
	case "SUCCESSFUL":
		st.Status = "succeeded"
	case "FAILED", "REJECTED", "TIMEOUT":
		st.Status = "failed"
	default:
		st.Status = "pending"
	}
	return st, nil
}
