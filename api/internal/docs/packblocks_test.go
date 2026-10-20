package docs

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// packBoxes lists, per form drawn from blocks, texts printed in the approved pack PDF, in the pack's order.
// Cell labels print in capitals; bars print as "N  TITLE".
var packBoxes = map[string][]string{
	"FRM-06": {"TRANSMITTAL AND FAX COVER SHEET", "TO", "FROM", "PAGES", "PURPOSE", "For approval / Pour approbation", "Document", "Copies", "Observations", "MESSAGE", "CONFIDENTIAL. This transmission"},
	"FRM-07": {"1  ORGANISATION", "LEGAL NAME", "RCCM NO.", "SECTOR", "Banking & finance", "SIZE", "2  CONTACTS", "Primary contact", "Escalation / Escalade", "3  BILLING", "PO REQUIRED?", "4  SERVICES OF INTEREST", "5  DOCUMENTS TO ATTACH", "Signed NDA", "AUTHORISED SIGNATORY", "FOR HEKATE USE ONLY", "KYC RESULT"},
	"FRM-08": {"ORGANISATION", "REQUEST DATE", "1  WHAT DO YOU NEED?", "Penetration test", "DESCRIBE THE NEED", "2  DRIVERS", "3  ENVIRONMENT", "NUMBER OF USERS", "KEY SYSTEMS AND TECHNOLOGIES", "4  TIMING AND BUDGET", "DECISION PROCESS", "REQUESTER / DEMANDEUR", "FOR HEKATE USE ONLY", "QUOTATION NO."},
	"FRM-09": {"1  TEST TYPE", "External network", "APPROACH", "2  TARGETS IN SCOPE", "Target", "3  EXCLUSIONS", "4  CONSTRAINTS", "TESTING WINDOW", "ALLOWED", "5  EMERGENCY CONTACTS", "Client \\(backup", "6  THIRD PARTIES", "The signatory confirms", "CLIENT AUTHORISING OFFICER", "HEKATE ENGAGEMENT LEAD"},
	"FRM-10": {"Rate each control", "COMPLETED BY", "Governance", "Security policy approved by management", "Third-party security reviews", "MAIN CONCERNS", "COMPLIANCE OBLIGATIONS", "Law 2024-017", "RESPONDENT"},
	"FRM-11": {"USE CASE", "GO-LIVE TARGET", "1  VERIFICATION SERVICES", "Sanctions & PEP screening", "DOCUMENTS", "2  VOLUMES AND INTEGRATION", "CALLBACK / WEBHOOK URL", "3  DATA PROTECTION", "DPO CONTACT", "Data Processing Addendum", "FOR HEKATE USE ONLY", "PRODUCTION DATE"},
	"FRM-12": {"One form per candidate", "1  REQUESTING ORGANISATION", "POSITION APPLIED FOR", "2  CANDIDATE", "SURNAME", "DATE OF BIRTH", "3  CHECKS REQUESTED", "Criminal record extract", "TURNAROUND", "CONSENT ATTACHED", "FOR HEKATE USE ONLY", "RESULT SENT TO"},
	"FRM-13": {"1  CLIENT IDENTIFICATION", "COUNTRY OF REGISTRATION", "2  OWNERSHIP AND CONTROL", "Beneficial owner", "Director", "3  RISK SCREENING", "RESULTS", "RISK RATING", "ANALYST COMMENTS", "DECISION", "COMPLIANCE ANALYST", "APPROVED BY", "NEXT REVIEW DATE", "FILE REFERENCE"},
	"FRM-15": {"very dissatisfied", "PROJECT / SERVICE", "Understanding of your needs", "Overall satisfaction", "WOULD YOU RECOMMEND HEKATE?", "WHAT DID WE DO WELL?", "WHAT SHOULD WE IMPROVE?", "MAY WE QUOTE YOU AS A REFERENCE?", "RESPONDENT"},
	"FRM-24": {"Indicative rates excluding tax", "FCFA HT", "Penetration test \x97 web application", "Consulting \x97 consultant", "VALID FROM", "APPROVED BY"},
	"FRM-26": {"PROJECT", "SOW / PO REF.", "FACILITATOR", "1  ATTENDEES", "Signature", "2  CONFIRMED", "Rules of engagement", "3  ACTIONS", "Responsable", "RISKS, DECISIONS AND NOTES", "CLIENT SPONSOR", "HEKATE PROJECT MANAGER"},
	"FRM-31": {"CLOSURE DATE", "PROJECT MANAGER", "1  CLOSURE CHECKLIST", "All deliverables accepted", "2  OUTCOME", "BUDGET PLANNED", "SCHEDULE VARIANCE", "LESSONS LEARNED", "FOLLOW-UP OPPORTUNITIES", "HEKATE CEO"},
	"FRM-32": {"Personal data is used only for recruitment", "POSITION APPLIED FOR", "1  PERSONAL DETAILS", "2  EDUCATION", "Institution", "3  CERTIFICATIONS", "ISO 27001 LI/LA", "4  EXPERIENCE", "Employer", "5  LANGUAGES", "English", "WHY HEKATE?", "EXPECTED SALARY", "NOTICE PERIOD", "I certify the information above", "APPLICANT / CANDIDAT"},
	"FRM-33": {"TO", "ADDRESS", "Dear ____", "is pleased to offer you employment", "POSITION", "TRIAL PERIOD", "GROSS MONTHLY SALARY", "PLACE OF WORK", "REPORTS TO", "within 7 days", "ACCEPTED BY THE CANDIDATE"},
	"FRM-37": {"Every visitor shows ID", "SITE", "REGISTER PERIOD", "Organisation", "Host", "Badge", "Lab Y/N", "Signature"},
	"FRM-39": {"1  COMPANY", "TAX REGIME", "CATEGORY", "2  BANK DETAILS", "SWIFT / BIC", "3  SECURITY AND COMPLIANCE", "CNPS clearance", "The supplier confirms", "SUPPLIER LEGAL REPRESENTATIVE", "FOR HEKATE USE ONLY", "BANK DETAILS VERIFIED BY CALL-BACK"},
	"FRM-41": {"Score 1", "SUPPLIER", "PERIOD", "Quality of goods / services", "Information security practices", "Overall", "DECISION", "Keep with improvement plan", "EVALUATOR", "APPROVED BY"},
}

func TestFramedFormsReproducePackBoxes(t *testing.T) {
	for _, c := range packOrder {
		if _, ok := frameBlocks[c]; !ok {
			t.Errorf("%s has no block layout", c)
			continue
		}
		ref := "HS-" + c + "-2026-000001"
		sc := schemaFor(ref, "")
		render := func() []byte {
			b, err := RenderNarrative(opts, ref, sc.en, at, "T", packClient, nil, "")
			if err != nil {
				t.Fatal(c, err)
			}
			return b
		}
		b := render()
		assertOrder(t, b, packBoxes[c])
		if bytes.Contains(b, []byte("ADDITIONAL SECTIONS")) {
			t.Errorf("%s: nothing was entered, so there is nothing additional", c)
		}
		if bytes.Contains(b, []byte("XAF")) || bytes.Contains(b, []byte("Name / Nom: A")) {
			t.Errorf("%s: blank must stay blank", c)
		}
		if !bytes.Equal(b, render()) {
			t.Errorf("%s: output must be byte-stable", c)
		}
	}
	if len(FrameOnly()) != 0 {
		t.Errorf("forms still frame-only: %v", FrameOnly())
	}
}

func TestEveryPrefilledHeadingLandsInItsPackBox(t *testing.T) {
	for code, hs := range PackHeadings() {
		if len(hs) == 0 {
			t.Errorf("%s: no headings", code)
			continue
		}
		var secs []Section
		for _, h := range hs {
			secs = append(secs, Section{h, "x"})
		}
		ref := code + "-2026-000001"
		sc := schemaFor(ref, "")
		if sc == nil {
			t.Fatalf("%s: no schema", code)
		}
		b, err := RenderNarrative(opts, ref, sc.en, at, "T", packClient, secs, "")
		if err != nil {
			t.Fatal(code, err)
		}
		if bytes.Contains(b, []byte("ADDITIONAL SECTIONS")) {
			t.Errorf("%s: a pre-filled heading did not match any pack box", code)
		}
	}
}

func TestPackHeadingsFileInSyncWithLayouts(t *testing.T) {
	want, err := json.MarshalIndent(PackHeadings(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = []byte("// Generated by `UPDATE_PACK_HEADINGS=1 go test ./internal/docs -run PackHeadingsFile` from the pack layouts in\n// api/internal/docs (packblocks.go, packnarr.go); do not edit by hand. The headings the editor pre-fills for each form.\nexport const PACK_HEADINGS: Record<string, string[]> = " + string(want) + ";\n")
	path := "../../../web/lib/pack-headings.ts"
	if os.Getenv("UPDATE_PACK_HEADINGS") != "" {
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("web/lib/pack-headings.ts is out of date: run UPDATE_PACK_HEADINGS=1 go test ./internal/docs -run PackHeadingsFile")
	}
}

func TestTextLandsInCellsChecksAndTables(t *testing.T) {
	secs := []Section{
		{"Trading name / Nom commercial", "Acme Trading"},
		{"Sector / Secteur", "Health and Energy"},
		{"Contacts", "Technical / Technique | Jo Biya | CTO | jo@acme.cm | 677000000\nCall after 5pm"},
		{"Legal form", "SARL"},
	}
	b, err := RenderNarrative(opts, "HS-FRM-07-2026-000001", "", at, "T", packClient, secs, "")
	if err != nil {
		t.Fatal(err)
	}
	mustHave(t, "frm07", b, "Acme Trading", "Jo Biya", "677000000", "Call after 5pm", "SARL", "Acme SARL", "RC/DLA/2021/B/9")
	if bytes.Contains(b, []byte("ADDITIONAL SECTIONS")) {
		t.Error("all four entries have a pack place")
	}
	ticked, _ := tickOptions(secDef{opts: []string{"Banking & finance", "Energy", "Health", "NGO"}, multi: true}, "Health and Energy")
	if len(ticked) != 2 || !ticked["Health"] || !ticked["Energy"] {
		t.Errorf("sector ticks: %v", ticked)
	}
}

func TestScoreTableMarksTheScore(t *testing.T) {
	b, err := RenderNarrative(opts, "HS-FRM-10-2026-000001", "", at, "T", packClient, []Section{{"Controls", "Named security owner (CISO/RSSI) | 2 | agreed\nNot a table line"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	mustHave(t, "frm10", b, "agreed", "Not a table line")
	if bytes.Contains(b, []byte("ADDITIONAL SECTIONS")) {
		t.Error("controls heading must land in the controls table")
	}
}

func TestDraftOfFramedFormFindsItsLayoutByTitle(t *testing.T) {
	b, err := RenderNarrativeDraft(opts, "Rate card", at, "Rates", packClient, []Section{{"Rates", "Red-team exercise | 1500000 | 2000000"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	mustHave(t, "draft", b, "RATE CARD", "GRILLE TARIFAIRE", "FCFA HT", "Red-team exercise", "1500000", "2000000", "DRAFT PREVIEW - NOT ISSUED")
	if bytes.Contains(b, []byte("Document no.")) {
		t.Error("a draft has no document number")
	}
}
