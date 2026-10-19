package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PushMessage is one notification to one device, in the Expo push service format.
type PushMessage struct {
	To    string         `json:"to"`
	Title string         `json:"title"`
	Body  string         `json:"body"`
	Data  map[string]any `json:"data,omitempty"`
	Sound string         `json:"sound,omitempty"`
}

// PushResult is the service's verdict for one message, in the same order as the request.
type PushResult struct {
	Status string `json:"status"`
	Error  string `json:"-"`
}

// Sender delivers push messages. ExpoSender is the real one; tests use a fake server.
type Sender interface {
	Send(ctx context.Context, msgs []PushMessage) ([]PushResult, error)
}

// ExpoSender posts to the Expo push service (https://exp.host/--/api/v2/push/send).
type ExpoSender struct {
	URL         string
	AccessToken string // optional, for accounts that enforce push security
	Client      *http.Client
}

func NewExpoSender(accessToken string) *ExpoSender {
	return &ExpoSender{URL: "https://exp.host/--/api/v2/push/send", AccessToken: accessToken, Client: &http.Client{Timeout: 10 * time.Second}}
}

func (e *ExpoSender) Send(ctx context.Context, msgs []PushMessage) ([]PushResult, error) {
	body, _ := json.Marshal(msgs)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if e.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.AccessToken)
	}
	resp, err := e.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("push service returned %d", resp.StatusCode)
	}
	var out struct {
		Data []struct {
			Status  string `json:"status"`
			Details struct {
				Error string `json:"error"`
			} `json:"details"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Data) != len(msgs) {
		return nil, fmt.Errorf("unexpected push service response")
	}
	res := make([]PushResult, len(out.Data))
	for i, d := range out.Data {
		res[i] = PushResult{Status: d.Status, Error: d.Details.Error}
	}
	return res, nil
}

// Notifier turns outbox events into push notifications. It is a platform.Publisher so it can sit
// beside the broker publisher. Delivery is best effort: a failure is logged, never retried forever.
type Notifier struct {
	pool   *pgxpool.Pool
	sender Sender
}

func NewNotifier(pool *pgxpool.Pool, sender Sender) *Notifier {
	return &Notifier{pool: pool, sender: sender}
}

const batchSize = 100 // the Expo service accepts up to 100 messages per request

func (n *Notifier) Publish(ctx context.Context, subject string, payload []byte, _ string) error {
	var p map[string]any
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil
	}
	msg, aud := Plan(subject, p)
	if msg == nil {
		return nil
	}
	tokens, err := n.recipients(ctx, aud)
	if err != nil || len(tokens) == 0 {
		return err
	}
	for start := 0; start < len(tokens); start += batchSize {
		end := min(start+batchSize, len(tokens))
		batch := make([]PushMessage, 0, end-start)
		for _, t := range tokens[start:end] {
			batch = append(batch, PushMessage{To: t, Title: msg.Title, Body: msg.Body, Data: msg.Data, Sound: "default"})
		}
		results, err := n.sender.Send(ctx, batch)
		if err != nil {
			slog.Warn("push delivery failed", "subject", subject, "err", err)
			continue
		}
		for i, r := range results {
			if r.Status != "ok" && r.Error == "DeviceNotRegistered" { // the app was uninstalled: stop sending to it
				if _, err := n.pool.Exec(ctx, `SELECT notify.forget_token($1)`, batch[i].To); err != nil {
					slog.Warn("forget token", "err", err)
				}
			}
		}
	}
	return nil
}

func (n *Notifier) recipients(ctx context.Context, a *Audience) ([]string, error) {
	var actor, tenant any
	if a.Actor != "" {
		actor = a.Actor
	}
	if a.Tenant != "" {
		tenant = a.Tenant
	}
	roles := a.Roles
	if roles == nil {
		roles = []string{}
	}
	rows, err := n.pool.Query(ctx, `SELECT token, actor FROM notify.tokens_for($1,$2,$3)`, actor, tenant, roles)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var token, owner string
		if err := rows.Scan(&token, &owner); err != nil {
			return nil, err
		}
		if a.Except != "" && owner == a.Except {
			continue
		}
		out = append(out, token)
	}
	return out, rows.Err()
}
