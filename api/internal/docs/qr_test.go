package docs

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"
)

func TestSignedQRRoundTripAndTampering(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	defer UseQRPublicKeyForTesting(pub)()
	o := Options{PortalURL: "https://hekatesystems.com/", QRSigner: priv, Version: "2.0"}
	issued := time.Date(2026, 10, 16, 9, 30, 0, 0, time.UTC)
	ref := "HS-INV-2026-000012"
	url := o.QRContent(ref, issued)
	if !strings.HasPrefix(url, "https://hekatesystems.com/v/HS-INV-2026-000012.2.0.20261016.C.") {
		t.Fatalf("signed address: %s", url)
	}
	q, err := ParseSignedQR(url)
	if err != nil || q.Ref != ref || q.Version != "2.0" || q.Date != "20261016" || q.Class != "C" {
		t.Fatalf("a genuine code must parse and verify: %+v %v", q, err)
	}
	if o.QRContent(ref, issued) != url {
		t.Error("Ed25519 signatures are deterministic, so the same document must give the same code")
	}
	// Any change to any field, or a signature from another key, is refused.
	for name, bad := range map[string]string{
		"date":      strings.Replace(url, ".20261016.", ".20261017.", 1),
		"class":     strings.Replace(url, ".C.", ".P.", 1),
		"version":   strings.Replace(url, ".2.0.", ".1.0.", 1),
		"reference": strings.Replace(url, "000012", "000013", 1),
		"truncated": url[:len(url)-4],
		"garbage":   "https://hekatesystems.com/v/not-a-code",
	} {
		if _, err := ParseSignedQR(bad); err == nil {
			t.Errorf("a code with a changed %s must be refused", name)
		}
	}
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	forged := Options{PortalURL: "https://hekatesystems.com", QRSigner: other}.QRContent(ref, issued)
	if _, err := ParseSignedQR(forged); err == nil {
		t.Error("a code signed by someone else's key must be refused")
	}
}

func TestWithoutASignerTheQRIsAPlainVerifyAddress(t *testing.T) {
	o := Options{PortalURL: "https://hekatesystems.com"}
	if got := o.QRContent("HS-QT-2026-000001", time.Now()); got != "https://hekatesystems.com/verify?ref=HS-QT-2026-000001" {
		t.Errorf("unsigned QR: %s", got)
	}
}

func TestOnlyHekatesPublishedKeyCanBeLoadedAsSigner(t *testing.T) {
	if s, err := LoadQRSigner(""); s != nil || err != nil {
		t.Errorf("no path means no signer: %v %v", s, err)
	}
	if _, err := LoadQRSigner("/does/not/exist"); err == nil {
		t.Error("a missing key file must be an error")
	}
}
