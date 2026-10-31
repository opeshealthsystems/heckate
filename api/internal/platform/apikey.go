package platform

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
)

// KeyPrefix marks bearer credentials that are API keys rather than JWTs.
const KeyPrefix = "hk_"

// APIKey describes the key behind a request authenticated with one.
type APIKey struct {
	ID     string
	Scopes []string
	Rate   int
	Quota  int64
}

// NewAPIKey returns a fresh secret, its display prefix and the hash stored in the database.
func NewAPIKey() (secret, prefix, hash string) {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	body := hex.EncodeToString(b)
	prefix = KeyPrefix + body[:8]
	secret = prefix + "_" + body[8:]
	return secret, prefix, HashKey(secret)
}

func HashKey(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

// keyResources are the first path segments an API key may read, when its scope names them.
// Key management, devices and the live stream are never reachable with a key.
var keyDenied = map[string]bool{"api-keys": true, "api-plans": true, "devices": true, "stream": true}

// KeyAllows reports whether a key with these scopes may call this route pattern.
func KeyAllows(scopes []string, pattern string) bool {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok || (method != http.MethodGet && method != http.MethodHead) {
		return false
	}
	seg := strings.TrimPrefix(path, "/v1/")
	if i := strings.Index(seg, "/"); i >= 0 {
		seg = seg[:i]
	}
	if seg == "" || keyDenied[seg] {
		return false
	}
	for _, s := range scopes {
		if s == "read:*" || s == "read:"+seg {
			return true
		}
	}
	return false
}

type keyLimiters struct {
	mu sync.Mutex
	by map[int]*Limiter
}

func (k *keyLimiters) get(rate int) *Limiter {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.by == nil {
		k.by = map[int]*Limiter{}
	}
	l := k.by[rate]
	if l == nil {
		l = NewLimiter(rate, 60e9)
		k.by[rate] = l
	}
	return l
}
