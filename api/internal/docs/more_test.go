package docs

import (
	"bytes"
	"testing"
)

func mustHave(t *testing.T, name string, b []byte, wants ...string) {
	t.Helper()
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		t.Fatalf("%s: not a PDF", name)
	}
	for _, w := range wants {
		if !bytes.Contains(b, []byte(w)) {
			t.Errorf("%s must contain %q", name, w)
		}
	}
}

func TestMoreTemplates(t *testing.T) {
	client := Party{Name: "Acme SARL", RCCM: "RC/DLA/2021/B/9"}
	cn := func() ([]byte, error) {
		return RenderCreditNote(opts, "HS-CN-2026-000002", at, "HS-INV-2026-000007", "Billing error", client, Money{Net: 100000, TVA: 17500, CAC: 1750, Total: 119250})
	}
	rc := func() ([]byte, error) {
		return RenderReceipt(opts, "HS-RCT-2026-000003", at, client, "HS-INV-2026-000007", 1192500, 500000, "momo", "TX123")
	}
	po := func() ([]byte, error) {
		return RenderPurchaseOrder(opts, "HS-PO-2026-000004", at, "Laptops", Party{Name: "Fournisseur SA"}, []Line{{"Laptop", 2, 450000}}, 900000)
	}
	rfq := func() ([]byte, error) {
		return RenderRFQ(opts, "HS-RFQ-2026-000005", at, "Switches", &at, []QtyLine{{"Switch 24p", 3}})
	}
	af := func() ([]byte, error) {
		return RenderAssetForm(opts, "HS-FRM-38-2026-000006", Asset{Item: "Laptop X1", Serial: "SN-42", Holder: "Jean Dupont", IssuedBy: "Admin", IssuedAt: at})
	}
	type c struct {
		name string
		fn   func() ([]byte, error)
		want []string
	}
	for _, x := range []c{
		{"credit note", cn, []string{"HS-CN-2026-000002", "HS-INV-2026-000007", "119 250 XAF", "verify?ref=HS-CN-2026-000002"}},
		{"receipt", rc, []string{"HS-RCT-2026-000003", "500 000 XAF", "TX123"}},
		{"purchase order", po, []string{"HS-PO-2026-000004", "900 000 XAF", "Fournisseur SA"}},
		{"rfq", rfq, []string{"HS-RFQ-2026-000005", "Switch 24p"}},
		{"asset form", af, []string{"HS-FRM-38-2026-000006", "SN-42", "Jean Dupont"}},
	} {
		a, err := x.fn()
		if err != nil {
			t.Fatal(x.name, err)
		}
		b, _ := x.fn()
		mustHave(t, x.name, a, x.want...)
		if !bytes.Equal(a, b) {
			t.Errorf("%s: output must be deterministic", x.name)
		}
	}
}

func TestClassificationBannerAndMicrotext(t *testing.T) {
	cases := map[string]string{
		"HS-INV-2026-000007":     "CONFIDENTIAL",
		"HS-FRM-14-2026-000002":  "RESTRICTED",
		"HS-FRM-15-2026-000002":  "INTERNAL",
		"HS-GP-2026-000001":      "PUBLIC",
		"HS-RPT-2026-000001":     "RESTRICTED",
		"HS-PAY-2026-000009":     "RESTRICTED",
		"HS-UNKNOWN-2026-000001": "CONFIDENTIAL",
		"garbage":                "CONFIDENTIAL",
	}
	for ref, want := range cases {
		if got := Classification(ref); got != want {
			t.Errorf("Classification(%q) = %q, want %q", ref, got, want)
		}
	}
	b, err := RenderCreditNote(opts, "HS-FRM-14-2026-000002", at, "HS-INV-2026-000007", "x", Party{Name: "A"}, Money{Net: 1, Total: 1})
	if err != nil {
		t.Fatal(err)
	}
	mustHave(t, "classified PDF", b, "RESTRICTED", "HEKATESYSTEMS")
}
