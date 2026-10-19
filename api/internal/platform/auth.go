package platform

import (
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Claims are the token attributes the API relies on (RBAC + ABAC).
type Claims struct {
	Sub      string
	Email    string
	TenantID string
	Roles    []string
	Exp      time.Time // when the token stops being valid (long-lived connections must end then)
}

var staffRoles = map[string]bool{
	"ceo": true, "manager": true, "finance": true, "sales": true, "support": true,
	"engineer": true, "hr": true, "employee": true, "logistics": true, "procurement": true,
}

// Scope maps roles to the row-level-security role: staff, client or other.
func (c *Claims) Scope() string {
	for _, r := range c.Roles {
		if staffRoles[r] {
			return "staff"
		}
	}
	for _, r := range c.Roles {
		if r == "client_user" || r == "client_admin" {
			return "client"
		}
	}
	return "other"
}

func (c *Claims) HasAny(roles ...string) bool {
	for _, want := range roles {
		for _, have := range c.Roles {
			if have == want {
				return true
			}
		}
	}
	return false
}

// Verifier validates bearer tokens: RS256 via Keycloak JWKS, HS256 only when a dev secret is configured.
type Verifier struct {
	cfg  Config
	mu   sync.Mutex
	keys map[string]*rsa.PublicKey
	at   time.Time
	http *http.Client
}

func NewVerifier(cfg Config) *Verifier {
	return &Verifier{cfg: cfg, keys: map[string]*rsa.PublicKey{}, http: &http.Client{Timeout: 5 * time.Second}}
}

var ErrUnauthorized = errors.New("invalid token")

func (v *Verifier) Verify(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrUnauthorized
	}
	var hdr struct{ Alg, Kid string }
	if err := decodeJSON(parts[0], &hdr); err != nil {
		return nil, ErrUnauthorized
	}
	signed := []byte(parts[0] + "." + parts[1])
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrUnauthorized
	}
	switch hdr.Alg {
	case "HS256":
		if v.cfg.DevSecret == "" {
			return nil, ErrUnauthorized
		}
		m := hmac.New(sha256.New, []byte(v.cfg.DevSecret))
		m.Write(signed)
		if !hmac.Equal(m.Sum(nil), sig) {
			return nil, ErrUnauthorized
		}
	case "RS256":
		key, err := v.key(hdr.Kid)
		if err != nil {
			return nil, ErrUnauthorized
		}
		h := sha256.Sum256(signed)
		if rsa.VerifyPKCS1v15(key, crypto.SHA256, h[:], sig) != nil {
			return nil, ErrUnauthorized
		}
	default:
		return nil, ErrUnauthorized
	}

	var p struct {
		Sub         string   `json:"sub"`
		Email       string   `json:"email"`
		Iss         string   `json:"iss"`
		Aud         any      `json:"aud"`
		Exp         int64    `json:"exp"`
		TenantID    string   `json:"tenant_id"`
		Roles       []string `json:"roles"`
		RealmAccess struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}
	if err := decodeJSON(parts[1], &p); err != nil || p.Sub == "" {
		return nil, ErrUnauthorized
	}
	if p.Exp == 0 || time.Now().Unix() >= p.Exp {
		return nil, ErrUnauthorized
	}
	if v.cfg.Issuer != "" && p.Iss != v.cfg.Issuer {
		return nil, ErrUnauthorized
	}
	if v.cfg.Audience != "" && !audMatches(p.Aud, v.cfg.Audience) {
		return nil, ErrUnauthorized
	}
	return &Claims{Sub: p.Sub, Email: p.Email, TenantID: p.TenantID, Roles: append(p.Roles, p.RealmAccess.Roles...), Exp: time.Unix(p.Exp, 0)}, nil
}

func audMatches(aud any, want string) bool {
	switch a := aud.(type) {
	case string:
		return a == want
	case []any:
		for _, x := range a {
			if s, _ := x.(string); s == want {
				return true
			}
		}
	}
	return false
}

func decodeJSON(seg string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func (v *Verifier) key(kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if k, ok := v.keys[kid]; ok && time.Since(v.at) < time.Hour {
		return k, nil
	}
	if v.cfg.JWKSURL == "" {
		return nil, errors.New("no JWKS configured")
	}
	resp, err := v.http.Get(v.cfg.JWKSURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var set struct {
		Keys []struct{ Kid, Kty, N, E string } `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return nil, err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	v.keys, v.at = keys, time.Now()
	if k, ok := keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown kid %q", kid)
}

// SignDevToken mints an HS256 token. Used by tests and the dev-token command only.
func SignDevToken(secret string, claims map[string]any) string {
	enc := func(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }
	head := enc(map[string]string{"alg": "HS256", "typ": "JWT"})
	body := enc(claims)
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(head + "." + body))
	return head + "." + body + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
