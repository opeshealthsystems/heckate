package docs

import (
	"bytes"
	"testing"
)

func TestFrenchAmount(t *testing.T) {
	cases := map[int64]string{
		1:          "Un franc CFA",
		21:         "Vingt et un francs CFA",
		71:         "Soixante et onze francs CFA",
		80:         "Quatre-vingts francs CFA",
		91:         "Quatre-vingt-onze francs CFA",
		200:        "Deux cents francs CFA",
		1000:       "Mille francs CFA",
		80000:      "Quatre-vingt mille francs CFA",
		1788750:    "Un million sept cent quatre-vingt-huit mille sept cent cinquante francs CFA",
		2000000:    "Deux millions de francs CFA",
		1200000000: "Un milliard deux cents millions de francs CFA",
	}
	for n, want := range cases {
		if got := FrenchAmount(n); got != want {
			t.Errorf("FrenchAmount(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestVatLabelNeverContradictsAmount(t *testing.T) {
	if a, b := vatLabel(Money{Net: 1000000, TVA: 175000, CAC: 17500}); a != "TVA 17.5 %" || b != "CAC (10 % of TVA)" {
		t.Errorf("standard rates should carry the pack labels: %q %q", a, b)
	}
	if a, b := vatLabel(Money{Net: 100, TVA: 20, CAC: 5}); a != "TVA" || b != "CAC" {
		t.Errorf("non-standard amounts must not claim the standard rate: %q %q", a, b)
	}
	if a, _ := vatLabel(Money{Net: 100000}); a != "TVA" {
		t.Errorf("an exempt invoice must not claim a rate: %q", a)
	}
}

var packClient = Party{Name: "Acme SARL", RCCM: "RC/DLA/2021/B/9", Email: "a@acme.cm"}

// assertOrder checks that each text appears after the one before it: the pack's section order.
func assertOrder(t *testing.T, b []byte, parts []string) {
	t.Helper()
	pos := 0
	for _, p := range parts {
		i := bytes.Index(b[pos:], []byte(p))
		if i < 0 {
			t.Errorf("%q missing or out of order", p)
			return
		}
		pos += i + len(p)
	}
}

func TestQuoteFollowsPackLayout(t *testing.T) {
	render := func() []byte {
		b, err := RenderQuote(opts, "HS-QT-2026-000004", at, &at, "Pentest", packClient, []Line{{"Test", 1, 100000}}, Money{Net: 100000, TVA: 17500, CAC: 1750, Total: 119250}, "")
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	b := render()
	assertOrder(t, b, []string{"QUOTATION", "DEVIS / FACTURE PRO FORMA", "HS-QT", "DOCUMENT NO.", "VALID UNTIL", "CLIENT", "RCCM NO.", "QUOTED SERVICES", "Prestations propos\xe9es", "Unit price", "Total excl. tax / Total HT", "TVA 17.5 %", "TOTAL incl. tax / TOTAL TTC", "Arr\xeat\xe9 \xe0 la somme de", "TERMS", "Accepted as quoted / Accept\xe9", "Declined / Refus\xe9"})
	mustHave(t, "quote", b, "N\xb0 RCCM", "Cent dix-neuf mille deux cent cinquante francs CFA", "verify?ref=HS-QT-2026-000004")
	if !bytes.Equal(b, render()) {
		t.Error("quote must be byte-stable")
	}
}

func TestAssetFormFollowsFRM38(t *testing.T) {
	b, err := RenderAssetForm(opts, "HS-FRM-38-2026-000006", Asset{Item: "Laptop X1", Serial: "SN-42", Holder: "Jean Dupont", IssuedBy: "Admin", IssuedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	assertOrder(t, b, []string{"ASSET ISSUE AND RETURN FORM", "FICHE DE REMISE", "EMPLOYEE", "STAFF NO.", "Asset tag", "Serial No.", "Returned", "SECURITY SET-UP", "Full-disk encryption", "MFA enrolled", "Acceptable-use policy signed", "I have received the equipment", "EMPLOYEE / EMPLOY\xc9", "IT / INFORMATIQUE - ISSUE", "IT / INFORMATIQUE - RETURN"})
	mustHave(t, "asset", b, "Laptop X1", "SN-42", "Jean Dupont", "Signature & stamp / Signature et cachet:", "HS-FRM-38-2026-000006")
}

func TestSOWSchemaOrderAndPlacement(t *testing.T) {
	secs := []Section{{"5. Team and responsibilities", "Role split."}, {"Scope", "Portal."}, {"Risks", "Delay."}, {"CUSTOM HEADING", "Kept text."}, {"Background and objectives", "Why."}}
	b, err := RenderNarrative(opts, "HS-SOW-2026-000001", "Statement of work", at, "Portal", packClient, secs, "A. Manager")
	if err != nil {
		t.Fatal(err)
	}
	// Pack order whatever the order entered; unmatched sections follow, nothing is dropped.
	assertOrder(t, b, []string{"STATEMENT OF WORK", "SOW NO.", "MSA REF.", "BACKGROUND AND OBJECTIVES", "Why.", "Portal.", "DELIVERABLES", "PLAN AND MILESTONES", "TEAM AND RESPONSIBILITIES", "Role split.", "CLIENT PREREQUISITES", "FEES", "ASSUMPTIONS AND RISKS", "Delay.", "ADDITIONAL SECTIONS", "CUSTOM HEADING", "Kept text.", "HEKATE SYSTEMS SARL", "Name / Nom: A. Manager", "CLIENT / CLIENT"})
	mustHave(t, "sow", b, "Total excl. tax / Total HT", "Attached to and governed by the Master Services Agreement")
}

func TestBlankStaysBlank(t *testing.T) {
	b, _ := RenderNarrative(opts, "HS-SOW-2026-000001", "Statement of work", at, "Portal", packClient, nil, "")
	if bytes.Contains(b, []byte("XAF")) {
		t.Error("a fee table nobody filled in must carry no figures")
	}
	if bytes.Contains(b, []byte("Name / Nom: A")) {
		t.Error("no signer entered, so none is printed")
	}
	po, _ := RenderPurchaseOrder(opts, "HS-PO-2026-000004", at, "Laptops", Party{Name: "Fournisseur SA"}, []Line{{"Laptop", 2, 450000}}, 900000)
	if bytes.Contains(po, []byte("Neuf cent")) {
		t.Error("a PO shows only its net total; the amount-in-words line must stay blank")
	}
}

func TestDraftHasNoQRAndNoReference(t *testing.T) {
	b, err := RenderNarrativeDraft(opts, "Change request form", at, "Portal", packClient, []Section{{"Priority", "High"}, {"Decision", "Approved with conditions"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	mustHave(t, "draft", b, "CHANGE REQUEST FORM", "DRAFT PREVIEW - NOT ISSUED - NOT VALID", "DEMANDE DE MODIFICATION", "HS-FRM-23")
	if bytes.Contains(b, []byte("/verify?ref=")) || bytes.Contains(b, []byte("Scan to verify")) {
		t.Error("a draft must carry no QR and no verification address")
	}
}

func TestOptionTicking(t *testing.T) {
	var decision, typ secDef
	for _, s := range schemas["FRM-23"].secs {
		if s.en == "Decision" {
			decision = s
		}
	}
	for _, s := range schemas["FRM-14"].secs {
		if s.en == "Type" {
			typ = s
		}
	}
	got, _ := tickOptions(decision, "Approved with conditions")
	if len(got) != 1 || !got["Approved with conditions / Avec réserves"] {
		t.Errorf("only 'approved with conditions' must be ticked: %v", got)
	}
	got, _ = tickOptions(decision, "")
	if len(got) != 0 {
		t.Errorf("nothing entered, nothing ticked: %v", got)
	}
	got, _ = tickOptions(typ, "Ransomware and phishing")
	if len(got) != 2 || !got["Ransomware"] || !got["Phishing / BEC"] {
		t.Errorf("multi-choice must tick every named option: %v", got)
	}
}

func TestEveryPackFormHasTitleBlock(t *testing.T) {
	for _, c := range append(LayoutFaithful(), FrameOnly()...) {
		ref := "HS-" + c + "-2026-000001"
		sc := schemaFor(ref, "")
		if sc == nil {
			t.Fatalf("%s: no schema", c)
		}
		b, err := RenderNarrative(opts, ref, sc.en, at, "T", packClient, []Section{{"Anything", "Body text"}}, "")
		if err != nil {
			t.Fatal(c, err)
		}
		mustHave(t, c, b, sc.en, "FORM / FORMULAIRE", sc.code, "Body text", "verify?ref="+ref)
		if sc2 := schemaFor("DRAFT", sc.en); sc2 == nil || sc2.code != sc.code {
			t.Errorf("%s: a draft must find the same layout by its title", c)
		}
	}
}

func TestNonPackKindsKeepPlainLayout(t *testing.T) {
	b, err := RenderNarrative(opts, "HS-LT-2026-000001", "Letter", at, "Hello", packClient, []Section{{"", "Dear client"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	mustHave(t, "letter", b, "LETTER", "Dear client")
	if bytes.Contains(b, []byte("FORM / FORMULAIRE")) {
		t.Error("a letter is not a pack form")
	}
}

func TestVisitFollowsFRM27(t *testing.T) {
	b, err := RenderVisit(opts, "HS-FRM-27-2026-000003", Party{Name: "Acme"}, Visit{Engineer: "E", Summary: "PSU", CheckIn: at, CheckOut: at, InPos: "1,2", OutPos: "1,2", Report: "Done.", SignedBy: "Jean"})
	if err != nil {
		t.Fatal(err)
	}
	assertOrder(t, b, []string{"DELIVERY NOTE AND ACCEPTANCE CERTIFICATE", "PV NO.", "WORK CARRIED OUT", "Done.", "ACCEPTANCE", "Accepted without reservation", "Reservation", "FOR THE CLIENT / POUR LE CLIENT", "FOR HEKATE SYSTEMS SARL"})
}

func TestInvoiceAndPOFollowPack(t *testing.T) {
	inv, _ := RenderInvoice(opts, "HS-INV-2026-000007", at, &at, "S", packClient, []Line{{"A", 1, 100}}, Money{Net: 100, TVA: 18, CAC: 2, Total: 120})
	assertOrder(t, inv, []string{"INVOICE", "FACTURE", "HS-INV", "INVOICED SERVICES", "PAYMENT", "ACCOUNT NO.", "SWIFT", "ISSUED BY / \xc9MISE PAR - HEKATE SYSTEMS SARL"})
	po, _ := RenderPurchaseOrder(opts, "HS-PO-2026-000004", at, "S", Party{Name: "F SA"}, []Line{{"A", 1, 100}}, 100)
	assertOrder(t, po, []string{"PURCHASE ORDER", "BON DE COMMANDE", "SUPPLIER", "ITEMS", "DELIVERY ADDRESS", "APPROVED BY / APPROUV\xc9 PAR", "SUPPLIER ACCEPTANCE / ACCORD FOURNISSEUR"})
}
