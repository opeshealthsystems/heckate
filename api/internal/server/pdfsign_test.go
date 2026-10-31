package server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/modules/documents"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// With a signing pair configured, the stored original, the register hash, the verification page and every download
// all refer to the same signed bytes, and the signing time is the register's issue time.
func TestSignedPDFIsTheRegisteredOriginal(t *testing.T) {
	_ = setup(t) // skips without a database and migrates it
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "Throwaway Test Signer"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	kd, _ := x509.MarshalPKCS8PrivateKey(key)
	dir := t.TempDir()
	cf, kf := filepath.Join(dir, "c.pem"), filepath.Join(dir, "k.pem")
	_ = os.WriteFile(cf, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)
	_ = os.WriteFile(kf, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kd}), 0o600)

	srv := httptest.NewServer(New(platform.Config{DevSecret: secret, PortalURL: "https://app.example.com", PaymentsSandbox: true, PDFSigningCertFile: cf, PDFSigningKeyFile: kf}, testPool(t)))
	t.Cleanup(func() { srv.Close(); documents.SetPDFSigner(nil) })
	s := &env{t, srv}

	sfx := fmt.Sprint(time.Now().UnixNano())
	fin := token("sign-fin", "", "finance")
	tenant := s.newClient(token("m", "", "manager"), "Signed PDF "+sfx)
	_, inv, _ := s.do("POST", "/v1/invoices", fin, map[string]any{"tenant_id": tenant, "title": "Signed " + sfx, "lines": lines(500_000)})
	id := inv["id"].(string)
	_, issued, _ := s.do("POST", "/v1/invoices/"+id+"/issue", fin, nil)
	ref := issued["ref"].(string)

	resp, first := s.pdf(fin, "/v1/invoices/"+id+"/pdf")
	if resp.StatusCode != 200 {
		t.Fatalf("pdf: %d", resp.StatusCode)
	}
	if !bytes.Contains(first, []byte("/SubFilter /ETSI.CAdES.detached")) || !bytes.Contains(first, []byte("/ByteRange [0 ")) {
		t.Fatal("the PDF must carry a PAdES signature when a pair is configured")
	}
	if resp.Header.Get("X-Document-SHA256") != hexSum(first) {
		t.Error("the hash header must be the SHA-256 of the signed bytes")
	}
	_, v, _ := s.do("GET", "/v1/public/verify/"+ref, "", nil)
	if v["sha256"] != hexSum(first) {
		t.Errorf("the register and the verification page must carry the hash of the signed file: %v", v)
	}
	_, again := s.pdf(fin, "/v1/invoices/"+id+"/pdf")
	if !bytes.Equal(first, again) {
		t.Error("downloads must be byte-identical (the stored signed original)")
	}

	// The signing time is the register's issue time.
	_, d, _ := s.do("GET", "/v1/documents/"+ref, fin, nil)
	issuedAt, err := time.Parse(time.RFC3339Nano, d["issued_at"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if want := "/M (D:" + issuedAt.UTC().Format("20060102150405") + "+00'00')"; !bytes.Contains(first, []byte(want)) {
		t.Errorf("signing time must be the issue time, want %s", want)
	}

	// The digest over the ByteRange is intact, and a changed byte breaks it.
	m := regexp.MustCompile(`/ByteRange \[0 (\d+) (\d+) (\d+)\]`).FindSubmatch(first)
	a, _ := strconv.Atoi(string(m[1]))
	b, _ := strconv.Atoi(string(m[2]))
	if b+mustAtoi(m[3]) != len(first) {
		t.Fatal("ByteRange must reach the end of the file")
	}
	digest := func(p []byte) [32]byte { return sha256.Sum256(append(append([]byte{}, p[:a]...), p[b:]...)) }
	orig := digest(first)
	tampered := append([]byte(nil), first...)
	tampered[10] ^= 1
	if digest(tampered) == orig {
		t.Error("tampering must change the signed digest")
	}

	// Independent validator, when the machine has one (poppler). Self-signed means the issuer is unknown, but the
	// signature itself must be valid.
	if bin, err := exec.LookPath("pdfsig"); err == nil {
		f := filepath.Join(dir, "x.pdf")
		_ = os.WriteFile(f, first, 0o600)
		out, _ := exec.Command(bin, f).CombinedOutput()
		if !bytes.Contains(out, []byte("Signature is Valid")) {
			t.Errorf("pdfsig: %s", out)
		}
		tf := filepath.Join(dir, "t.pdf")
		_ = os.WriteFile(tf, tampered, 0o600)
		if out, _ := exec.Command(bin, tf).CombinedOutput(); bytes.Contains(out, []byte("Signature is Valid")) {
			t.Errorf("pdfsig accepted a tampered file: %s", out)
		}
	}
}

func mustAtoi(b []byte) int { n, _ := strconv.Atoi(string(b)); return n }
