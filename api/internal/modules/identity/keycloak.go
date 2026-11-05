package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrUserExists means Keycloak already has an account with that email.
var ErrUserExists = errors.New("a user with this email already exists")

// Keycloak creates users through the admin REST API with a service-account client (client credentials grant).
// The client needs the realm-management roles manage-users and view-realm.
type Keycloak struct {
	AdminURL     string // e.g. https://auth.example.com/admin/realms/hekate
	TokenURL     string // e.g. https://auth.example.com/realms/hekate/protocol/openid-connect/token
	ClientID     string
	ClientSecret string
	HTTP         *http.Client
}

func (k *Keycloak) Enabled() bool { return k != nil && k.AdminURL != "" && k.ClientID != "" && k.ClientSecret != "" }

func (k *Keycloak) client() *http.Client {
	if k.HTTP != nil {
		return k.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (k *Keycloak) token(ctx context.Context) (string, error) {
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {k.ClientID}, "client_secret": {k.ClientSecret}}
	req, _ := http.NewRequestWithContext(ctx, "POST", k.TokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := k.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("keycloak token: status %d", resp.StatusCode)
	}
	var t struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return "", err
	}
	return t.AccessToken, nil
}

func (k *Keycloak) call(ctx context.Context, tok, method, path string, body any) (*http.Response, error) {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequestWithContext(ctx, method, strings.TrimRight(k.AdminURL, "/")+path, rd)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	return k.client().Do(req)
}

// CreateUser makes the account, gives it one realm role and the tenant, and emails the person a link to set
// their password and verify their address. It returns the new Keycloak user id.
func (k *Keycloak) CreateUser(ctx context.Context, email, first, last, role, tenant string) (string, error) {
	tok, err := k.token(ctx)
	if err != nil {
		return "", err
	}
	user := map[string]any{
		"username": email, "email": email, "firstName": first, "lastName": last, "enabled": true,
		"attributes":      map[string][]string{"tenant_id": {tenant}},
		"requiredActions": []string{"UPDATE_PASSWORD", "VERIFY_EMAIL"},
	}
	resp, err := k.call(ctx, tok, "POST", "/users", user)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return "", ErrUserExists
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("keycloak create user: status %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	id := loc[strings.LastIndex(loc, "/")+1:]
	if id == "" {
		return "", errors.New("keycloak create user: no id returned")
	}

	resp, err = k.call(ctx, tok, "GET", "/roles/"+url.PathEscape(role), nil)
	if err != nil {
		return id, err
	}
	var rep map[string]any
	err = json.NewDecoder(resp.Body).Decode(&rep)
	resp.Body.Close()
	if resp.StatusCode != 200 || err != nil {
		return id, fmt.Errorf("keycloak role %s: status %d", role, resp.StatusCode)
	}
	resp, err = k.call(ctx, tok, "POST", "/users/"+id+"/role-mappings/realm", []any{rep})
	if err != nil {
		return id, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return id, fmt.Errorf("keycloak assign role: status %d", resp.StatusCode)
	}

	resp, err = k.call(ctx, tok, "PUT", "/users/"+id+"/execute-actions-email?lifespan=259200", []string{"UPDATE_PASSWORD", "VERIFY_EMAIL"})
	if err != nil {
		return id, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return id, fmt.Errorf("keycloak send email: status %d", resp.StatusCode)
	}
	return id, nil
}
