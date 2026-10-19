package docs

import (
	"crypto/ed25519"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// The QR code on an issued document can carry a signed address, as the brand pack's security notes require:
//
//	https://<portal>/v/<reference>.<version>.<yyyymmdd>.<class>.<signature>
//
// The signature is Ed25519 over "<reference>|<version>|<yyyymmdd>|<class>". Anyone can check it with the public key;
// only the holder of the private key (kept in a secret store, never in the repository) can make one. Without the
// private key the QR carries a plain /verify?ref= address instead, which still works but proves nothing offline.

//go:embed assets/HEKATE_QR_Public.pem
var qrPublicPEM []byte

var qrPublic = func() ed25519.PublicKey {
	b, _ := pem.Decode(qrPublicPEM)
	k, err := x509.ParsePKIXPublicKey(b.Bytes)
	if err != nil {
		panic("embedded QR public key: " + err.Error())
	}
	return k.(ed25519.PublicKey)
}()

// UseQRPublicKeyForTesting makes signed addresses verify against another key and returns a function that restores
// HEKATE's. It exists so tests can sign with a key pair they generate; production code never calls it.
func UseQRPublicKeyForTesting(k ed25519.PublicKey) (restore func()) {
	old := qrPublic
	qrPublic = k
	return func() { qrPublic = old }
}

// classCodes are the one-letter classification codes of the signed address.
var classCodes = map[string]string{"PUBLIC": "P", "INTERNAL": "I", "CONFIDENTIAL": "C", "RESTRICTED": "R"}

// LoadQRSigner reads an unencrypted PKCS#8 Ed25519 private key in PEM form. An empty path means "no signer".
func LoadQRSigner(path string) (ed25519.PrivateKey, error) {
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b, _ := pem.Decode(raw)
	if b == nil {
		return nil, errors.New("QR signing key: no PEM block (the key must be unencrypted PKCS#8)")
	}
	k, err := x509.ParsePKCS8PrivateKey(b.Bytes)
	if err != nil {
		return nil, fmt.Errorf("QR signing key: %w", err)
	}
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("QR signing key is not an Ed25519 key")
	}
	if !priv.Public().(ed25519.PublicKey).Equal(qrPublic) { // the signer must be the key HEKATE has published
		return nil, errors.New("QR signing key does not match the published HEKATE QR public key")
	}
	return priv, nil
}

func signedMessage(ref, version, date, class string) []byte {
	return []byte(ref + "|" + version + "|" + date + "|" + class)
}

// SignedQRURL is the signed address for an issued document.
func (o Options) SignedQRURL(ref string, issued time.Time) string {
	version := o.Version
	if version == "" {
		version = "1.0"
	}
	class := classCodes[Classification(ref)]
	if class == "" {
		class = "I"
	}
	date := issued.UTC().Format("20060102")
	sig := base64.RawURLEncoding.EncodeToString(ed25519.Sign(o.QRSigner, signedMessage(ref, version, date, class)))
	return strings.TrimRight(o.PortalURL, "/") + "/v/" + ref + "." + version + "." + date + "." + class + "." + sig
}

// QRContent is what the QR code encodes: the signed address when a signer is configured, else the plain one.
func (o Options) QRContent(ref string, issued time.Time) string {
	if len(o.QRSigner) == ed25519.PrivateKeySize {
		return o.SignedQRURL(ref, issued)
	}
	return o.VerifyURL(ref)
}

// SignedQR is a parsed signed address.
type SignedQR struct {
	Ref, Version, Date, Class string
}

// ParseSignedQR splits "<reference>.<version>.<yyyymmdd>.<class>.<signature>" (the part after /v/) and checks the
// signature against the published HEKATE public key. The returned error says why a code is not accepted.
func ParseSignedQR(token string) (SignedQR, error) {
	token = strings.TrimSpace(token)
	if i := strings.Index(token, "/v/"); i >= 0 {
		token = token[i+3:]
	}
	// The reference has no dots, so the first dot ends it; the version is "major.minor", so it takes two parts.
	parts := strings.Split(token, ".")
	if len(parts) != 6 {
		return SignedQR{}, errors.New("not a HEKATE signed code")
	}
	q := SignedQR{Ref: strings.ToUpper(parts[0]), Version: parts[1] + "." + parts[2], Date: parts[3], Class: parts[4]}
	if !refCode.MatchString(q.Ref) || len(q.Date) != 8 || len(q.Class) != 1 {
		return SignedQR{}, errors.New("not a HEKATE signed code")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[5])
	if err != nil || len(sig) != ed25519.SignatureSize {
		return SignedQR{}, errors.New("the signature is malformed")
	}
	if !ed25519.Verify(qrPublic, signedMessage(q.Ref, q.Version, q.Date, q.Class), sig) {
		return SignedQR{}, errors.New("the signature is not HEKATE's")
	}
	return q, nil
}
