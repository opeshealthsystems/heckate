package docs

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"
)

var signAt = time.Date(2026, 10, 19, 9, 30, 0, 0, time.UTC)

// throwaway returns a self-signed certificate and key as PEM; nothing here is a real credential.
func throwaway(t *testing.T, ec bool) (certPEM, keyPEM []byte) {
	t.Helper()
	var key crypto.Signer
	var err error
	if ec {
		key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	} else {
		key, err = rsa.GenerateKey(rand.Reader, 2048)
	}
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{SerialNumber: big.NewInt(4242), Subject: pkix.Name{CommonName: "Test Signer", Organization: []string{"Test"}},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	kd, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kd})
}

type cmsAlg struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}
type cmsAttr struct {
	Type   asn1.ObjectIdentifier
	Values asn1.RawValue
}
type cmsSignerInfo struct {
	Version   int
	SID       asn1.RawValue
	Digest    cmsAlg
	Attrs     asn1.RawValue `asn1:"tag:0"`
	SigAlg    cmsAlg
	Signature []byte
}
type cmsSignedData struct {
	Version int
	Digests asn1.RawValue
	Encap   asn1.RawValue
	Certs   asn1.RawValue   `asn1:"tag:0"`
	Signers []cmsSignerInfo `asn1:"set"`
}
type cmsContent struct {
	Type    asn1.ObjectIdentifier
	Content asn1.RawValue `asn1:"explicit,tag:0"`
}

type errStr string

func (e errStr) Error() string { return string(e) }

// verifyPDF is an independent check: it finds the signature the way a reader would (ByteRange, Contents), decodes
// the CMS with encoding/asn1 structs, recomputes the digest over the byte ranges and checks the signature.
func verifyPDF(pdf []byte) error {
	m := regexp.MustCompile(`/ByteRange \[(\d+) (\d+) (\d+) (\d+)\]`).FindSubmatch(pdf)
	if m == nil {
		return errStr("no ByteRange")
	}
	var br [4]int
	for i := range br {
		br[i], _ = strconv.Atoi(string(m[i+1]))
	}
	if br[0] != 0 || br[2]+br[3] != len(pdf) || pdf[br[1]] != '<' || pdf[br[2]-1] != '>' {
		return errStr("ByteRange does not cover the file around /Contents")
	}
	h := sha256.New()
	h.Write(pdf[:br[1]])
	h.Write(pdf[br[2]:])
	want := h.Sum(nil)
	raw, err := hex.DecodeString(string(pdf[br[1]+1 : br[2]-1]))
	if err != nil {
		return err
	}
	var ci cmsContent
	if _, err := asn1.Unmarshal(raw, &ci); err != nil { // trailing zero padding is allowed
		return err
	}
	var sd cmsSignedData
	if _, err := asn1.Unmarshal(ci.Content.Bytes, &sd); err != nil {
		return err
	}
	certs, err := x509.ParseCertificates(sd.Certs.Bytes)
	if err != nil || len(certs) == 0 {
		return errStr("no certificates in CMS")
	}
	si := sd.Signers[0]
	var gotDigest []byte
	ess := false
	for rest := si.Attrs.Bytes; len(rest) > 0; {
		var a cmsAttr
		if rest, err = asn1.Unmarshal(rest, &a); err != nil {
			return err
		}
		switch {
		case a.Type.Equal(oidMessageDigest):
			if _, err := asn1.Unmarshal(a.Values.Bytes, &gotDigest); err != nil {
				return err
			}
		case a.Type.Equal(oidSigningCertV2):
			c := sha256.Sum256(certs[0].Raw)
			if !bytes.Contains(a.Values.Bytes, c[:]) {
				return errStr("signing-certificate-v2 does not name the signer certificate")
			}
			ess = true
		}
	}
	if !ess {
		return errStr("signing-certificate-v2 attribute missing (required for PAdES-B-B)")
	}
	if !bytes.Equal(gotDigest, want) {
		return errStr("messageDigest differs from the digest of the ByteRange content")
	}
	signed := append([]byte{0x31}, si.Attrs.FullBytes[1:]...) // re-tag [0] IMPLICIT as SET for verification
	d := sha256.Sum256(signed)
	switch pk := certs[0].PublicKey.(type) {
	case *rsa.PublicKey:
		return rsa.VerifyPKCS1v15(pk, crypto.SHA256, d[:], si.Signature)
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(pk, d[:], si.Signature) {
			return errStr("bad ECDSA signature")
		}
	}
	return nil
}

func signer(t *testing.T, ec bool) *PDFSigner {
	t.Helper()
	c, k := throwaway(t, ec)
	s, err := NewPDFSigner(c, k, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSignedPDFVerifiesAndKeepsOriginal(t *testing.T) {
	for _, ec := range []bool{false, true} {
		for _, compress := range []bool{false, true} {
			o := opts
			o.Compress = compress
			orig := invoice(t, o, "HS-INV-2026-000007", []Line{{"External penetration test", 1, 1000000}})
			out, err := signer(t, ec).SignPDF(orig, signAt)
			if err != nil {
				t.Fatalf("ec=%v: %v", ec, err)
			}
			if !bytes.HasPrefix(out, orig) {
				t.Error("incremental update must leave the original bytes untouched")
			}
			if err := verifyPDF(out); err != nil {
				t.Errorf("ec=%v compress=%v: %v", ec, compress, err)
			}
			if !bytes.Contains(out, []byte("/M (D:20261019093000+00'00')")) || !bytes.Contains(out, []byte("ETSI.CAdES.detached")) {
				t.Error("signing time must come from the argument and the SubFilter must be CAdES detached")
			}
			// the new xref chain must be walkable: startxref -> xref -> /Prev -> original xref
			sx := regexp.MustCompile(`startxref\n(\d+)\n%%EOF\n$`).FindSubmatch(out)
			x, _ := strconv.Atoi(string(sx[1]))
			if !bytes.HasPrefix(out[x:], []byte("xref\n")) || !regexp.MustCompile(`/Prev (\d+)`).Match(out[x:]) {
				t.Error("broken xref chain")
			}
			for _, m := range regexp.MustCompile(`(?m)^(\d{10}) 00000 n $`).FindAllSubmatch(out[x:], -1) {
				off, _ := strconv.Atoi(string(m[1]))
				if !regexp.MustCompile(`^\d+ 0 obj\n`).Match(out[off:]) {
					t.Errorf("xref entry %s does not point at an object", m[1])
				}
			}
		}
	}
}

func TestSigningIsDeterministicForRSA(t *testing.T) {
	s := signer(t, false)
	orig := invoice(t, opts, "HS-INV-2026-000007", []Line{{"x", 1, 100}})
	a, _ := s.SignPDF(orig, signAt)
	b, _ := s.SignPDF(orig, signAt)
	if !bytes.Equal(a, b) {
		t.Error("same input, key and time must give the same bytes")
	}
	c, _ := s.SignPDF(orig, signAt.Add(time.Second))
	if bytes.Equal(a, c) {
		t.Error("a different signing time must change the output")
	}
}

func TestTamperedSignedPDFFails(t *testing.T) {
	orig := invoice(t, opts, "HS-INV-2026-000007", []Line{{"External penetration test", 1, 1000000}})
	out, err := signer(t, false).SignPDF(orig, signAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPDF(out); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func([]byte){
		"text in the original": func(b []byte) { copy(b[bytes.Index(b, []byte("External")):], "Internal") },
		"appended xref":        func(b []byte) { b[len(b)-20] ^= 1 },
	} {
		cp := append([]byte(nil), out...)
		mut(cp)
		if verifyPDF(cp) == nil {
			t.Errorf("tampering (%s) must fail verification", name)
		}
	}
	// a signature made over different bytes must not verify, whoever made it
	other, _ := signer(t, false).SignPDF(orig, signAt.Add(time.Hour))
	re := regexp.MustCompile(`/Contents <[0-9a-f]+>`)
	cp := re.ReplaceAll(out, re.Find(other))
	if verifyPDF(cp) == nil {
		t.Error("a signature over different bytes must fail")
	}
}

func TestSignerLoading(t *testing.T) {
	c, k := throwaway(t, false)
	dir := t.TempDir()
	cf, kf := filepath.Join(dir, "c.pem"), filepath.Join(dir, "k.pem")
	if err := os.WriteFile(cf, c, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kf, k, 0o600); err != nil {
		t.Fatal(err)
	}
	if s, err := LoadPDFSigner("", "", time.Now()); s != nil || err != nil {
		t.Error("nothing configured must mean no signer and no error")
	}
	if _, err := LoadPDFSigner(cf, "", time.Now()); err == nil {
		t.Error("half a pair must be refused")
	}
	if _, err := LoadPDFSigner(cf, kf, time.Now()); err != nil {
		t.Errorf("good pair: %v", err)
	}
	if _, err := LoadPDFSigner(cf, kf, time.Now().Add(72*time.Hour)); err == nil {
		t.Error("an expired certificate must be refused")
	}
	_, k2 := throwaway(t, false)
	if _, err := NewPDFSigner(c, k2, time.Now()); err == nil {
		t.Error("a key that does not match the certificate must be refused")
	}
	if _, err := NewPDFSigner(c, []byte("junk"), time.Now()); err == nil {
		t.Error("junk key")
	}
}

func TestSignRefusesAlreadySignedOrUnknownPDF(t *testing.T) {
	s := signer(t, false)
	orig := invoice(t, opts, "HS-INV-2026-000007", []Line{{"x", 1, 100}})
	once, err := s.SignPDF(orig, signAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SignPDF(once, signAt); err == nil {
		t.Error("signing twice must be refused, not corrupt the file")
	}
	if _, err := s.SignPDF([]byte("%PDF-1.4 nonsense"), signAt); err == nil {
		t.Error("garbage must be refused")
	}
}
