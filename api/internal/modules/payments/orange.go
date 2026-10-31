package payments

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// Orange is the Orange Money WebPay API. It is off unless every credential is configured, and it has not
// been exercised against the live service: it follows the published WebPay contract and is tested only
// against a local stand-in.
//
// The provider's status lookup needs the order id, the amount and its pay token, while Gateway.Status
// receives a single reference, so Charge returns "orderID|payToken|amount" and Status splits it.
type Orange struct {
	cfg  platform.OrangeConfig
	http *http.Client
	mu   sync.Mutex
	tok  string
	exp  time.Time
}

func NewOrange(cfg platform.OrangeConfig, c *http.Client) *Orange {
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}
	if cfg.Currency == "" {
		cfg.Currency = "XAF"
	}
	if cfg.Country == "" {
		cfg.Country = "cm"
	}
	return &Orange{cfg: cfg, http: c}
}

func (o *Orange) Name() string { return "orange_money" }

func (o *Orange) base() string { return strings.TrimRight(o.cfg.BaseURL, "/") }

func (o *Orange) token(ctx context.Context) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.tok != "" && time.Now().Before(o.exp) {
		return o.tok, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, o.base()+"/oauth/v3/token", strings.NewReader(form.Encode()))
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(o.cfg.ClientID+":"+o.cfg.ClientSecret)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := o.http.Do(req)
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
	o.tok, o.exp = t.AccessToken, time.Now().Add(time.Duration(max(t.ExpiresIn-60, 30))*time.Second)
	return o.tok, nil
}

func (o *Orange) post(ctx context.Context, path string, body any) (*http.Response, error) {
	tok, err := o.token(ctx)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, o.base()+path, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProvider, err)
	}
	return resp, nil
}

func (o *Orange) Charge(ctx context.Context, c ChargeRequest) (string, error) {
	resp, err := o.post(ctx, "/orange-money-webpay/"+o.cfg.Country+"/v1/webpayment", map[string]any{
		"merchant_key": o.cfg.MerchantKey, "currency": o.cfg.Currency, "order_id": c.Reference,
		"amount": c.Amount, "return_url": o.cfg.ReturnURL, "cancel_url": o.cfg.CancelURL,
		"notif_url": o.cfg.NotifURL, "lang": "fr", "reference": c.Message,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r struct {
		PayToken string `json:"pay_token"`
	}
	if (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) || json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&r) != nil || r.PayToken == "" {
		return "", fmt.Errorf("%w: web payment answered %d", ErrProvider, resp.StatusCode)
	}
	return c.Reference + "|" + r.PayToken + "|" + strconv.FormatInt(c.Amount, 10), nil
}

func (o *Orange) Status(ctx context.Context, ref string) (State, error) {
	parts := strings.Split(ref, "|")
	if len(parts) != 3 {
		return State{}, fmt.Errorf("%w: malformed provider reference", ErrProvider)
	}
	amt, _ := strconv.ParseInt(parts[2], 10, 64)
	resp, err := o.post(ctx, "/orange-money-webpay/"+o.cfg.Country+"/v1/transactionstatus", map[string]any{
		"order_id": parts[0], "amount": amt, "pay_token": parts[1],
	})
	if err != nil {
		return State{}, err
	}
	defer resp.Body.Close()
	var r struct {
		Status string `json:"status"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&r) != nil {
		return State{}, fmt.Errorf("%w: status answered %d", ErrProvider, resp.StatusCode)
	}
	switch strings.ToUpper(r.Status) {
	case "SUCCESS":
		return State{Status: "succeeded", Amount: amt}, nil
	case "FAILED", "EXPIRED", "CANCELLED":
		return State{Status: "failed", Reason: strings.ToLower(r.Status)}, nil
	}
	return State{Status: "pending"}, nil
}
