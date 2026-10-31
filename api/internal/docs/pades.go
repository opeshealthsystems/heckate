package docs

// PDF signing: a PAdES-B-B style signature (CMS SignedData, detached, SHA-256, SubFilter ETSI.CAdES.detached)
// appended to a rendered PDF as an incremental update, so the original bytes are untouched and the signature
// covers them. It is dependency-free (stdlib crypto, encoding/asn1) and deterministic for RSA keys: the signing
// time comes from the caller (the register's issue time), never the clock.
//
// Scope, stated plainly: the signature is invisible, has no RFC 3161 timestamp and carries no revocation data
// (so it is not PAdES-B-T or B-LT). It was checked with an independent parser in tests and by hand with poppler's
// pdfsig; it has not been opened in Adobe Acrobat, and a self-signed certificate will show as untrusted there.

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// sigReserve is the room reserved for the CMS blob, in bytes (twice that in hex). A leaf plus a couple of
// intermediates with an RSA-4096 signature fits comfortably.
const sigReserve = 12288

// PDFSigner signs PDFs with one certificate and key.
type PDFSigner struct {
	leaf  *x509.Certificate
	chain []*x509.Certificate // leaf first, then any intermediates supplied in the certificate file
	key   crypto.Signer
}

// LoadPDFSigner reads a PEM certificate (leaf first, optional intermediates after it) and an unencrypted PEM
// private key (PKCS#8, PKCS#1 RSA or SEC1 EC). Both paths empty means "no signer" (nil, nil); exactly one set is
// an error. The key must match the leaf and the leaf must be valid at `now`.
func LoadPDFSigner(certFile, keyFile string, now time.Time) (*PDFSigner, error) {
	if certFile == "" && keyFile == "" {
		return nil, nil
	}
	if certFile == "" || keyFile == "" {
		return nil, errors.New("both HEKATE_PDF_SIGNING_CERT_FILE and HEKATE_PDF_SIGNING_KEY_FILE are required")
	}
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	return NewPDFSigner(certPEM, keyPEM, now)
}

// NewPDFSigner is LoadPDFSigner for PEM bytes already in memory.
func NewPDFSigner(certPEM, keyPEM []byte, now time.Time) (*PDFSigner, error) {
	var chain []*x509.Certificate
	for rest := certPEM; ; {
		var b *pem.Block
		b, rest = pem.Decode(rest)
		if b == nil {
			break
		}
		if b.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(b.Bytes)
		if err != nil {
			return nil, fmt.Errorf("signing certificate: %w", err)
		}
		chain = append(chain, c)
	}
	if len(chain) == 0 {
		return nil, errors.New("signing certificate: no CERTIFICATE PEM block")
	}
	kb, _ := pem.Decode(keyPEM)
	if kb == nil {
		return nil, errors.New("signing key: no PEM block (the key must be unencrypted)")
	}
	var key any
	var err error
	switch kb.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(kb.Bytes)
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(kb.Bytes)
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(kb.Bytes)
	default:
		err = fmt.Errorf("unsupported PEM type %q (encrypted keys must be decrypted into the secret store first)", kb.Type)
	}
	if err != nil {
		return nil, fmt.Errorf("signing key: %w", err)
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, errors.New("signing key: not a signing key")
	}
	switch signer.(type) {
	case *rsa.PrivateKey, *ecdsa.PrivateKey:
	default:
		return nil, errors.New("signing key: only RSA and ECDSA keys are supported")
	}
	leaf := chain[0]
	if !publicKeysEqual(leaf.PublicKey, signer.Public()) {
		return nil, errors.New("signing key does not match the first certificate in the certificate file")
	}
	if now.Before(leaf.NotBefore) || now.After(leaf.NotAfter) {
		return nil, fmt.Errorf("signing certificate is not valid now (valid %s to %s)", leaf.NotBefore.Format("2006-01-02"), leaf.NotAfter.Format("2006-01-02"))
	}
	return &PDFSigner{leaf: leaf, chain: chain, key: signer}, nil
}

func publicKeysEqual(a, b any) bool {
	type eq interface{ Equal(crypto.PublicKey) bool }
	if x, ok := a.(eq); ok {
		return x.Equal(b)
	}
	return false
}

// Subject names the certificate in logs.
func (s *PDFSigner) Subject() string { return s.leaf.Subject.String() }

var (
	oidData          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
	oidSignedData    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidContentType   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 3}
	oidMessageDigest = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}
	oidSigningCertV2 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 47}
	oidSHA256        = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidSHA256WithRSA = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11}
	oidECDSAWithSHA2 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}
)

func tlv(tag byte, content ...[]byte) []byte {
	n := 0
	for _, c := range content {
		n += len(c)
	}
	out := []byte{tag}
	switch {
	case n < 0x80:
		out = append(out, byte(n))
	case n < 0x100:
		out = append(out, 0x81, byte(n))
	case n < 0x10000:
		out = append(out, 0x82, byte(n>>8), byte(n))
	default:
		out = append(out, 0x83, byte(n>>16), byte(n>>8), byte(n))
	}
	for _, c := range content {
		out = append(out, c...)
	}
	return out
}

func mustDER(v any) []byte {
	b, err := asn1.Marshal(v)
	if err != nil {
		panic(err) // only fixed, well-typed values are marshalled
	}
	return b
}

func attr(oid asn1.ObjectIdentifier, value []byte) []byte {
	return tlv(0x30, mustDER(oid), tlv(0x31, value))
}

// cms builds the detached SignedData for a document digest.
func (s *PDFSigner) cms(docDigest []byte) ([]byte, error) {
	certHash := sha256.Sum256(s.leaf.Raw)
	issuerSerial := tlv(0x30, tlv(0x30, tlv(0xA4, s.leaf.RawIssuer)), mustDER(s.leaf.SerialNumber))
	essCert := tlv(0x30, tlv(0x30, tlv(0x30, tlv(0x04, certHash[:]), issuerSerial)))
	attrs := [][]byte{
		attr(oidContentType, mustDER(oidData)),
		attr(oidMessageDigest, tlv(0x04, docDigest)),
		attr(oidSigningCertV2, essCert),
	}
	sort.Slice(attrs, func(i, j int) bool { return bytes.Compare(attrs[i], attrs[j]) < 0 }) // DER SET OF is sorted
	body := bytes.Join(attrs, nil)
	toSign := tlv(0x31, body) // the signature covers the attributes encoded as an explicit SET
	digest := sha256.Sum256(toSign)
	sig, err := s.key.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		return nil, err
	}
	sha := tlv(0x30, mustDER(oidSHA256), []byte{0x05, 0x00})
	var sigAlg []byte
	if _, isRSA := s.key.(*rsa.PrivateKey); isRSA {
		sigAlg = tlv(0x30, mustDER(oidSHA256WithRSA), []byte{0x05, 0x00})
	} else {
		sigAlg = tlv(0x30, mustDER(oidECDSAWithSHA2))
	}
	signerInfo := tlv(0x30,
		mustDER(1),
		tlv(0x30, s.leaf.RawIssuer, mustDER(s.leaf.SerialNumber)),
		sha,
		tlv(0xA0, body), // signedAttrs, IMPLICIT [0]
		sigAlg,
		tlv(0x04, sig),
	)
	var certs [][]byte
	for _, c := range s.chain {
		certs = append(certs, c.Raw)
	}
	signedData := tlv(0x30,
		mustDER(1),
		tlv(0x31, sha),
		tlv(0x30, mustDER(oidData)),
		tlv(0xA0, certs...),
		tlv(0x31, signerInfo),
	)
	return tlv(0x30, mustDER(oidSignedData), tlv(0xA0, signedData)), nil
}

var (
	reStartXref = regexp.MustCompile(`startxref\s+(\d+)\s+%%EOF\s*$`)
	reRoot      = regexp.MustCompile(`/Root (\d+) 0 R`)
	reInfo      = regexp.MustCompile(`/Info (\d+) 0 R`)
	reSize      = regexp.MustCompile(`/Size (\d+)`)
	reXrefHead  = regexp.MustCompile(`^\s*(\d+) (\d+)[ ]*\r?\n`)
	reKids      = regexp.MustCompile(`/Kids\s*\[\s*(\d+) 0 R`)
)

// pdfIndex is what the signer needs to know about a classic, single-section PDF as written by the renderer.
type pdfIndex struct {
	offsets    []int // by object number
	size       int
	root, info int
	prevXref   int
}

func parsePDF(pdf []byte) (*pdfIndex, error) {
	tail := pdf
	if len(tail) > 1024 {
		tail = tail[len(tail)-1024:]
	}
	m := reStartXref.FindSubmatch(tail)
	if m == nil {
		return nil, errors.New("pdf: no startxref")
	}
	x, _ := strconv.Atoi(string(m[1]))
	if x <= 0 || x >= len(pdf) || !bytes.HasPrefix(pdf[x:], []byte("xref")) {
		return nil, errors.New("pdf: startxref does not point at a classic xref table (cross-reference streams are not supported)")
	}
	rest := pdf[x+4:]
	hm := reXrefHead.FindSubmatch(rest)
	if hm == nil {
		return nil, errors.New("pdf: unsupported xref layout")
	}
	first, _ := strconv.Atoi(string(hm[1]))
	count, _ := strconv.Atoi(string(hm[2]))
	if first != 0 || count < 2 {
		return nil, errors.New("pdf: unsupported xref layout (expected a single section starting at object 0)")
	}
	rest = rest[len(hm[0]):]
	idx := &pdfIndex{offsets: make([]int, count), prevXref: x}
	for i := 0; i < count; i++ {
		if len(rest) < 20 {
			return nil, errors.New("pdf: truncated xref")
		}
		off, _ := strconv.Atoi(string(bytes.TrimSpace(rest[:10])))
		idx.offsets[i] = off
		rest = rest[20:]
	}
	tr := bytes.Index(rest, []byte("trailer"))
	if tr < 0 {
		return nil, errors.New("pdf: no trailer")
	}
	trailer := rest[tr:]
	r, s := reRoot.FindSubmatch(trailer), reSize.FindSubmatch(trailer)
	if r == nil || s == nil {
		return nil, errors.New("pdf: trailer lacks /Root or /Size")
	}
	idx.root, _ = strconv.Atoi(string(r[1]))
	idx.size, _ = strconv.Atoi(string(s[1]))
	if i := reInfo.FindSubmatch(trailer); i != nil {
		idx.info, _ = strconv.Atoi(string(i[1]))
	}
	if idx.size != count || idx.root <= 0 || idx.root >= count {
		return nil, errors.New("pdf: inconsistent trailer")
	}
	return idx, nil
}

// object returns the dictionary text of a plain (non-stream) object.
func (p *pdfIndex) object(pdf []byte, n int) (string, error) {
	if n <= 0 || n >= len(p.offsets) {
		return "", fmt.Errorf("pdf: object %d out of range", n)
	}
	head := fmt.Sprintf("%d 0 obj", n)
	at := pdf[p.offsets[n]:]
	if !bytes.HasPrefix(at, []byte(head)) {
		return "", fmt.Errorf("pdf: object %d is not at its xref offset", n)
	}
	end := bytes.Index(at, []byte("endobj"))
	if end < 0 {
		return "", fmt.Errorf("pdf: object %d unterminated", n)
	}
	body := string(bytes.TrimSpace(at[len(head):end]))
	if len(body) < 4 || body[:2] != "<<" || body[len(body)-2:] != ">>" || bytes.Contains(at[:end], []byte("stream")) {
		return "", fmt.Errorf("pdf: object %d is not a plain dictionary", n)
	}
	return body, nil
}

// SignPDF returns pdf with a detached PAdES signature appended as an incremental update. The input is left
// unchanged inside the output. `at` becomes the signature's /M entry; use a time from the record, not the clock.
func (s *PDFSigner) SignPDF(pdf []byte, at time.Time) ([]byte, error) {
	idx, err := parsePDF(pdf)
	if err != nil {
		return nil, err
	}
	pages, err := idx.object(pdf, 1)
	if err != nil {
		return nil, err
	}
	kid := reKids.FindStringSubmatch(pages)
	if kid == nil {
		return nil, errors.New("pdf: no pages")
	}
	pageNo, _ := strconv.Atoi(kid[1])
	page, err := idx.object(pdf, pageNo)
	if err != nil {
		return nil, err
	}
	root, err := idx.object(pdf, idx.root)
	if err != nil {
		return nil, err
	}
	if bytes.Contains([]byte(root), []byte("/AcroForm")) {
		return nil, errors.New("pdf: document already has a form or signature; refusing to modify it")
	}
	sigNo, widNo := idx.size, idx.size+1

	// Rewritten catalog (adds the form) and first page (adds the widget annotation).
	root = root[:len(root)-2] + fmt.Sprintf("/AcroForm << /Fields [%d 0 R] /SigFlags 3 >>\n>>", widNo)
	if i := bytes.Index([]byte(page), []byte("/Annots [")); i >= 0 {
		i += len("/Annots [")
		page = page[:i] + fmt.Sprintf("%d 0 R ", widNo) + page[i:]
	} else if bytes.Contains([]byte(page), []byte("/Annots")) {
		return nil, errors.New("pdf: page annotations are an indirect object; unsupported")
	} else {
		page = page[:len(page)-2] + fmt.Sprintf("/Annots [%d 0 R]\n>>", widNo)
	}

	var b bytes.Buffer
	b.Write(pdf)
	if !bytes.HasSuffix(pdf, []byte("\n")) {
		b.WriteByte('\n')
	}
	offs := map[int]int{}
	put := func(n int, body string) {
		offs[n] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", n, body)
	}
	put(idx.root, root)
	put(pageNo, page)
	// Signature dictionary. ByteRange numbers are fixed-width so they can be patched without moving anything.
	offs[sigNo] = b.Len()
	fmt.Fprintf(&b, "%d 0 obj\n<< /Type /Sig /Filter /Adobe.PPKLite /SubFilter /ETSI.CAdES.detached\n/ByteRange [0 0000000000 0000000000 0000000000]\n/M (D:%s+00'00')\n/Contents <", sigNo, at.UTC().Format("20060102150405"))
	contentsAt := b.Len()
	b.Write(bytes.Repeat([]byte("0"), sigReserve*2))
	b.WriteString(">\n>>\nendobj\n")
	put(widNo, fmt.Sprintf("<< /Type /Annot /Subtype /Widget /FT /Sig /T (Signature1) /V %d 0 R /F 132 /Rect [0 0 0 0] /P %d 0 R >>", sigNo, pageNo))

	nums := make([]int, 0, len(offs))
	for n := range offs {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	xrefAt := b.Len()
	b.WriteString("xref\n")
	for i := 0; i < len(nums); {
		j := i
		for j+1 < len(nums) && nums[j+1] == nums[j]+1 {
			j++
		}
		fmt.Fprintf(&b, "%d %d\n", nums[i], j-i+1)
		for k := i; k <= j; k++ {
			fmt.Fprintf(&b, "%010d 00000 n \n", offs[nums[k]])
		}
		i = j + 1
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root %d 0 R", idx.size+2, idx.root)
	if idx.info > 0 {
		fmt.Fprintf(&b, " /Info %d 0 R", idx.info)
	}
	fmt.Fprintf(&b, " /Prev %d >>\nstartxref\n%d\n%%%%EOF\n", idx.prevXref, xrefAt)

	out := b.Bytes()
	contentsEnd := contentsAt + sigReserve*2 + 1 // past the closing '>'
	br := fmt.Sprintf("[0 %010d %010d %010d]", contentsAt-1, contentsEnd, len(out)-contentsEnd)
	brAt := offs[sigNo] + bytes.Index(out[offs[sigNo]:], []byte("[0 0000000000 0000000000 0000000000]"))
	copy(out[brAt:], br)
	h := sha256.New()
	h.Write(out[:contentsAt-1])
	h.Write(out[contentsEnd:])
	blob, err := s.cms(h.Sum(nil))
	if err != nil {
		return nil, err
	}
	if len(blob) > sigReserve {
		return nil, fmt.Errorf("signature (%d bytes) does not fit the reserved %d bytes", len(blob), sigReserve)
	}
	copy(out[contentsAt:], hex.EncodeToString(blob))
	return out, nil
}
