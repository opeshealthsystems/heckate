package docs

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"
)

var at = time.Date(2026, 10, 19, 9, 30, 0, 0, time.UTC)
var opts = Options{PortalURL: "https://app.example.com", Letterhead: []string{"Douala, Cameroon"}, Compress: false}

func TestXAF(t *testing.T) {
	cases := map[int64]string{0: "0 XAF", 5: "5 XAF", 999: "999 XAF", 1000: "1 000 XAF", 1192500: "1 192 500 XAF", -2500: "-2 500 XAF", 1000000000: "1 000 000 000 XAF"}
	for in, want := range cases {
		if got := XAF(in); got != want {
			t.Errorf("XAF(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestLineAmountRoundsHalfUp(t *testing.T) {
	if got := LineAmount(Line{Quantity: 2.5, UnitPrice: 333}); got != 833 {
		t.Errorf("got %d, want 833", got)
	}
	if got := LineAmount(Line{Quantity: 0.1, UnitPrice: 1000}); got != 100 {
		t.Errorf("got %d, want 100", got)
	}
}

func invoice(t *testing.T, o Options, ref string, lines []Line) []byte {
	t.Helper()
	b, err := RenderInvoice(o, ref, at, &at, "Penetration test", Party{Name: "Société Générale d'Électricité", RCCM: "RC/DLA/2020/B/123"}, lines, Money{Net: 1000000, TVA: 175000, CAC: 17500, Total: 1192500})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestInvoicePDFContentAndVerifyLink(t *testing.T) {
	b := invoice(t, opts, "HS-INV-2026-000007", []Line{{"External penetration test", 1, 1000000}})
	if !bytes.HasPrefix(b, []byte("%PDF-")) || !bytes.Contains(b, []byte("%%EOF")) {
		t.Fatal("not a PDF")
	}
	for _, want := range []string{"HS-INV-2026-000007", "INVOICE", "External penetration test", "1 192 500 XAF", "1 000 000 XAF", "175 000 XAF", "17 500 XAF", "https://app.example.com/verify?ref=HS-INV-2026-000007", "RC/DLA/2020/B/123"} {
		if !bytes.Contains(b, []byte(want)) {
			t.Errorf("PDF must contain %q", want)
		}
	}
	// Accents survive the conversion to the PDF's Latin-1 text encoding.
	if !bytes.Contains(b, []byte("Soci\xe9t\xe9 G\xe9n\xe9rale d'\xc9lectricit\xe9")) {
		t.Error("accented client name must be printed correctly")
	}
}

func TestOutputIsDeterministic(t *testing.T) {
	lines := []Line{{"A", 1, 100}, {"B", 2.5, 200}}
	if !bytes.Equal(invoice(t, opts, "HS-INV-2026-000001", lines), invoice(t, opts, "HS-INV-2026-000001", lines)) {
		t.Error("the same invoice must always produce the same bytes (its hash is registered)")
	}
	c := opts
	c.Compress = true
	if !bytes.Equal(invoice(t, c, "HS-INV-2026-000001", lines), invoice(t, c, "HS-INV-2026-000001", lines)) {
		t.Error("compressed output must also be deterministic")
	}
	if bytes.Equal(invoice(t, opts, "HS-INV-2026-000001", lines), invoice(t, opts, "HS-INV-2026-000002", lines)) {
		t.Error("a different reference must change the file")
	}
	if bytes.Equal(invoice(t, opts, "HS-INV-2026-000001", lines), invoice(t, opts, "HS-INV-2026-000001", []Line{{"A", 1, 101}, {"B", 2.5, 200}})) {
		t.Error("a changed amount must change the file")
	}
}

func TestLongInvoiceBreaksAcrossPages(t *testing.T) {
	var lines []Line
	for i := 0; i < 80; i++ {
		lines = append(lines, Line{Description: strings.Repeat("Managed security monitoring service. ", 3), Quantity: 1, UnitPrice: int64(1000 + i)})
	}
	b := invoice(t, opts, "HS-INV-2026-000009", lines)
	if n := bytes.Count(b, []byte("/Type /Page\n")); n < 3 {
		t.Errorf("80 long lines should span several pages, got %d", n)
	}
	if !bytes.Contains(b, []byte("Page 2 / ")) && !bytes.Contains(b, []byte("Page 2 /")) {
		t.Error("pages must be numbered")
	}
}

func TestNoQRWithoutPortalURL(t *testing.T) {
	o := opts
	o.PortalURL = ""
	b := invoice(t, o, "HS-INV-2026-000001", []Line{{"A", 1, 100}})
	if bytes.Contains(b, []byte("/verify?ref=")) || bytes.Contains(b, []byte("Scan to verify")) {
		t.Error("without a portal URL there is nothing to link to")
	}
}

func TestVisitWithAndWithoutSignature(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	img.Set(5, 5, color.Black)
	var sig bytes.Buffer
	_ = png.Encode(&sig, img)
	v := Visit{Engineer: "eng-1", Summary: "Replace PSU", CheckIn: at, CheckOut: at.Add(90 * time.Minute), InPos: "4.05000, 9.70000", OutPos: "4.05010, 9.70010",
		Report: "Replaced the faulty power supply and tested failover.", SignedBy: "Jean Dupont", Signature: sig.Bytes()}
	b, err := RenderVisit(opts, "HS-FRM-27-2026-000003", Party{Name: "Acme"}, v)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"HS-FRM-27-2026-000003", "Replaced the faulty power supply", "Jean Dupont", "4.05000, 9.70000", "/Subtype /Image"} {
		if !bytes.Contains(b, []byte(want)) {
			t.Errorf("visit PDF must contain %q", want)
		}
	}
	// Garbage in the signature field must not break rendering; it is replaced by a note.
	v.Signature = append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, 30)...)
	b, err = RenderVisit(opts, "HS-FRM-27-2026-000003", Party{Name: "Acme"}, v)
	if err != nil || !bytes.Contains(b, []byte("signature image unavailable")) {
		t.Errorf("an undecodable signature must be skipped, not fatal: %v", err)
	}
	v.SignedBy, v.Signature = "", nil
	b, _ = RenderVisit(opts, "HS-FRM-27-2026-000003", Party{Name: "Acme"}, v)
	if !bytes.Contains(b, []byte("without a client signature")) {
		t.Error("an unsigned visit must say so")
	}
}

func TestQuoteNotes(t *testing.T) {
	b, err := RenderQuote(opts, "HS-QT-2026-000004", at, &at, "Pentest", Party{Name: "Acme"}, []Line{{"Test", 1, 100}}, Money{Net: 100, TVA: 18, CAC: 2, Total: 120}, "Payment within 30 days.")
	if err != nil || !bytes.Contains(b, []byte("QUOTATION")) || !bytes.Contains(b, []byte("Payment within 30 days.")) {
		t.Errorf("quote: %v", err)
	}
}

// The register stores a hash of each issued file, so rendering must not depend on map iteration
// order or the clock. Repeating many times is what exposed an ordering bug in the PDF library.
func TestDeterministicAcrossManyRuns(t *testing.T) {
	lines := []Line{{"A", 1, 100}, {"B", 2.5, 200}}
	want := invoice(t, opts, "HS-INV-2026-000001", lines)
	for i := 0; i < 300; i++ {
		if !bytes.Equal(want, invoice(t, opts, "HS-INV-2026-000001", lines)) {
			t.Fatalf("run %d produced different bytes", i)
		}
	}
	v := Visit{Engineer: "e", CheckIn: at, CheckOut: at.Add(time.Hour), InPos: "1,2", OutPos: "1,2", Report: "Work done on site, tested.", SignedBy: "A B"}
	first, _ := RenderVisit(opts, "HS-FRM-27-2026-000001", Party{Name: "Acme"}, v)
	for i := 0; i < 100; i++ {
		again, _ := RenderVisit(opts, "HS-FRM-27-2026-000001", Party{Name: "Acme"}, v)
		if !bytes.Equal(first, again) {
			t.Fatalf("visit run %d produced different bytes", i)
		}
	}
}
