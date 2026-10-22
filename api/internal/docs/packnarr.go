package docs

// Narrative documents laid out as the pack's forms. A narrative is a list of titled free-text sections; each pack
// form is described here as a schema (header fields, numbered sections with English and French headings, check
// boxes, signature blocks). Entered sections are placed into the schema section whose heading they carry (matched
// by exact normalised heading, English or French); entered sections that match nothing are printed after the
// pack's sections under "Additional sections", never dropped. Pack sections nobody filled print as empty boxes.

import (
	"regexp"
	"strings"
	"time"
)

type secDef struct {
	bar     bool   // starts a new numbered section bar
	en, fr  string // heading (bar text, or box label when not a bar)
	alias   []string
	minH    float64
	opts    []string // check boxes instead of a text box
	multi   bool     // several options may be ticked
	perRow  int
	grid    []gcol // blank table shown when nothing was entered
	rows    int
	fees    bool   // the pack's fee table and totals, blank when nothing was entered
	fixedEN string // pack wording printed when nothing was entered
	fixedFR string
	letter  string // overrides the bar number (A, B ...)
}

type hfield struct {
	label, fr, key string // key: ref, date, subject, client, or "" for blank
	w              float64
}

type signSpec struct {
	title   string
	hekate  bool
	clientN bool
}

type schema struct {
	code, en, fr string
	top          []hfield
	top2         []hfield
	intro        [2]string // bilingual line under the title block
	partyBar     bool      // print the pack's "A Client" block
	secs         []secDef
	outro        [2]string
	signs        []signSpec
	blocks       []blk // box-level layout drawn by renderBlocks (packblocks.go); replaces the fields above
}

var bullets = func(a ...string) string { return "- " + strings.Join(a, "\n- ") }

const pkAbout = "HEKATE SYSTEMS SARL is a Cameroonian cybersecurity and technology company based in Douala. We secure, build and run the digital infrastructure of banks, insurers, healthcare providers, public bodies and growing businesses across Cameroon and the CEMAC region. Our services cover penetration testing, security operations through our HEKATE Cyber Defense Platform (HCDP), incident response, digital forensics, and identity verification through hecID."
const pkAboutFR = "HEKATE SYSTEMS SARL est une société camerounaise de cybersécurité et de technologie basée à Douala, au service des banques, assureurs, structures de santé, administrations et entreprises de la zone CEMAC."

var pkWhy = bullets("Local team in Douala with 24/7 incident line.", "Our own platforms (HCDP, hecID) operated in-country.",
	"Signed, verifiable deliverables: every report and certificate carries a digital signature and a verification QR code.",
	"Strict confidentiality: NDA, rules of engagement and data destruction at closure.")

var schemas = map[string]*schema{
	"PR": {
		code: "HS-PR", en: "COMMERCIAL AND TECHNICAL PROPOSAL", fr: "Proposition commerciale et technique",
		top:  []hfield{{"Proposal no.", "N°", "ref", 90}, {"Date", "", "date", 90}},
		top2: []hfield{{"Client", "", "client", 65}, {"Project", "Projet", "subject", 85}, {"Valid until", "Validité", "", 30}},
		secs: []secDef{
			{bar: true, en: "Executive summary", fr: "Synthèse", alias: []string{"executive summary", "synthèse", "synthese"}, minH: 25},
			{bar: true, en: "About HEKATE SYSTEMS", fr: "À propos de HEKATE SYSTEMS", alias: []string{"about hekate systems", "about hekate"}, fixedEN: pkAbout, fixedFR: pkAboutFR},
			{bar: true, en: "Understanding of the need", fr: "Compréhension du besoin", alias: []string{"understanding of the need", "compréhension du besoin", "comprehension du besoin"}, minH: 25},
			{bar: true, en: "Approach and methodology", fr: "Démarche et méthodologie", alias: []string{"approach and methodology", "approach", "methodology", "démarche et méthodologie"},
				grid: []gcol{{"Phase", "", 50, "L"}, {"Activities", "Activités", 60, "L"}, {"Deliverable", "Livrable", 40, "L"}, {"Duration", "Durée", 30, "L"}}, rows: 5},
			{bar: true, en: "Team", fr: "Équipe", alias: []string{"team", "équipe", "equipe"},
				grid: []gcol{{"Role", "Rôle", 45, "L"}, {"Name", "Nom", 50, "L"}, {"Certifications", "", 55, "L"}, {"Allocation", "Charge", 30, "L"}}, rows: 4},
			{bar: true, en: "Schedule", fr: "Planning", alias: []string{"schedule", "planning", "timeline"},
				grid: []gcol{{"Milestone", "Jalon", 100, "L"}, {"Start", "Début", 40, "L"}, {"End", "Fin", 40, "L"}}, rows: 4},
			{bar: true, en: "Financial proposal", fr: "Offre financière", alias: []string{"financial proposal", "offre financière", "fees", "pricing"}, fees: true, rows: 5},
			{bar: true, en: "Why HEKATE", fr: "Pourquoi HEKATE", alias: []string{"why hekate", "pourquoi hekate"}, fixedEN: pkWhy},
			{bar: true, en: "Terms", fr: "Conditions", alias: []string{"terms", "conditions", "terms and conditions"}, fixedEN: quoteTermsEN, fixedFR: quoteTermsFR},
		},
		signs: []signSpec{{title: "For HEKATE SYSTEMS SARL", hekate: true}, {title: "Client acceptance / Bon pour accord"}},
	},
	"SOW": {
		code: "HS-SOW", en: "STATEMENT OF WORK", fr: "Énoncé des travaux",
		intro: [2]string{"Attached to and governed by the Master Services Agreement (HS-DOC-27) between the parties.", ""},
		top:   []hfield{{"SOW no.", "N°", "ref", 60}, {"MSA ref.", "Réf. contrat-cadre", "", 60}, {"Version", "", "", 25}, {"Date", "", "date", 35}},
		top2:  []hfield{{"Client", "", "client", 90}, {"Project name", "Nom du projet", "subject", 90}},
		secs: []secDef{
			{bar: true, en: "Background and objectives", fr: "Contexte et objectifs", alias: []string{"background and objectives", "background", "contexte et objectifs"}, minH: 20},
			{en: "Objectives", fr: "Objectifs", alias: []string{"objectives", "objectifs"}, minH: 20},
			{bar: true, en: "Scope", fr: "Périmètre", alias: []string{"scope", "périmètre", "perimetre"}, minH: 5},
			{en: "In scope", fr: "Inclus", alias: []string{"in scope", "inclus"}, minH: 20},
			{en: "Out of scope", fr: "Exclus", alias: []string{"out of scope", "exclus"}, minH: 20},
			{bar: true, en: "Deliverables", fr: "Livrables", alias: []string{"deliverables", "livrables"},
				grid: []gcol{{"No.", "", 10, "L"}, {"Deliverable", "Livrable", 55, "L"}, {"Format", "", 30, "L"}, {"Due date", "Échéance", 30, "L"}, {"Acceptance criteria", "Critères", 55, "L"}}, rows: 5},
			{bar: true, en: "Plan and milestones", fr: "Planning et jalons", alias: []string{"plan and milestones", "milestones", "planning et jalons"},
				grid: []gcol{{"Milestone", "Jalon", 90, "L"}, {"Start", "Début", 30, "L"}, {"End", "Fin", 30, "L"}, {"Payment %", "Paiement", 30, "L"}}, rows: 4},
			{bar: true, en: "Team and responsibilities", fr: "Équipe et responsabilités", alias: []string{"team and responsibilities", "team", "équipe et responsabilités"},
				grid: []gcol{{"Role", "Rôle", 60, "L"}, {"HEKATE", "", 60, "L"}, {"Client", "", 60, "L"}}, rows: 4},
			{bar: true, en: "Client prerequisites", fr: "Prérequis client", alias: []string{"client prerequisites", "prerequisites", "prérequis client"}, minH: 5},
			{en: "Access, accounts, environments, contacts", fr: "Accès, comptes, environnements, contacts", alias: []string{"access, accounts, environments, contacts", "access"}, minH: 20},
			{bar: true, en: "Fees", fr: "Honoraires", alias: []string{"fees", "honoraires", "pricing"}, fees: true, rows: 4},
			{bar: true, en: "Assumptions and risks", fr: "Hypothèses et risques", alias: []string{"assumptions and risks", "hypothèses et risques"}, minH: 5},
			{en: "Assumptions", fr: "Hypothèses", alias: []string{"assumptions", "hypothèses", "hypotheses"}, minH: 20},
			{en: "Risks", fr: "Risques", alias: []string{"risks", "risques"}, minH: 20},
		},
		signs: []signSpec{{title: "HEKATE SYSTEMS SARL", hekate: true}, {title: "Client / Client"}},
	},
	"OC": {
		code: "HS-OC", en: "ORDER CONFIRMATION", fr: "Accusé de réception de commande",
		top:      []hfield{{"Document no.", "N°", "ref", 55}, {"Date", "", "date", 35}, {"Client ref.", "Réf. client", "", 45}, {"Valid until", "Échéance", "", 45}},
		partyBar: true,
		secs: []secDef{
			{en: "Client purchase order no.", fr: "N° bon de commande", alias: []string{"client purchase order no.", "purchase order", "client purchase order"}, minH: 8},
			{en: "Quotation ref.", fr: "Réf. devis", alias: []string{"quotation ref.", "quotation ref", "quotation"}, minH: 8},
			{en: "Planned start and end", fr: "Début et fin prévus", alias: []string{"planned start and end", "planned start", "planned end", "dates"}, minH: 8},
			{bar: true, letter: "B", en: "Confirmed services", fr: "Prestations confirmées", alias: []string{"confirmed services", "services", "prestations confirmées"}, fees: true, rows: 6},
			{en: "Project manager (HEKATE)", fr: "Chef de projet", alias: []string{"project manager (hekate)", "project manager", "chef de projet"}, minH: 8},
			{en: "Client contact", fr: "Interlocuteur client", alias: []string{"client contact", "interlocuteur client"}, minH: 8},
		},
		outro: [2]string{"HEKATE SYSTEMS SARL confirms acceptance of the order above under the Master Services Agreement and the referenced quotation. Any change to scope follows the Change Request procedure (HS-FRM-23).",
			"HEKATE SYSTEMS SARL confirme l’acceptation de la commande ci-dessus aux conditions du Contrat-cadre et du devis visé. Toute modification suit la procédure de demande de changement (HS-FRM-23)."},
		signs: []signSpec{{title: "HEKATE SYSTEMS SARL", hekate: true}},
	},
	"FRM-23": {
		code: "HS-FRM-23", en: "CHANGE REQUEST FORM", fr: "Demande de modification",
		top:  []hfield{{"CR no.", "N°", "ref", 45}, {"Project / SOW ref.", "Projet", "subject", 85}, {"Date", "", "date", 50}},
		top2: []hfield{{"Requested by", "Demandeur", "", 180}},
		secs: []secDef{
			{en: "Priority", fr: "Priorité", alias: []string{"priority", "priorité", "priorite"}, opts: []string{"Low / Faible", "Medium / Moyenne", "High / Haute", "Urgent"}, perRow: 4},
			{bar: true, en: "Requested change", fr: "Modification demandée", alias: []string{"requested change", "modification demandée"}, minH: 5},
			{en: "Description", fr: "", alias: []string{"description", "change description"}, minH: 25},
			{en: "Reason", fr: "Justification", alias: []string{"reason", "justification"}, minH: 20},
			{bar: true, en: "Impact assessment (HEKATE)", fr: "Analyse d’impact", alias: []string{"impact assessment (hekate)", "impact assessment", "analyse d’impact", "impact"}, minH: 5},
			{en: "Effort (days)", fr: "Charge (j)", alias: []string{"effort (days)", "effort", "charge"}, minH: 8},
			{en: "Cost impact (FCFA)", fr: "Impact coût", alias: []string{"cost impact (fcfa)", "cost impact", "cost", "impact coût"}, minH: 8},
			{en: "Schedule impact", fr: "Impact délai", alias: []string{"schedule impact", "impact délai"}, minH: 8},
			{en: "Risks and dependencies", fr: "Risques et dépendances", alias: []string{"risks and dependencies", "risks", "risques et dépendances"}, minH: 20},
			{en: "Decision", fr: "Décision", alias: []string{"decision", "décision"}, opts: []string{"Approved / Approuvée", "Approved with conditions / Avec réserves", "Rejected / Rejetée", "Deferred / Reportée"}, perRow: 2},
		},
		signs: []signSpec{{title: "Client approver / Approbateur client"}, {title: "HEKATE project manager / Chef de projet", hekate: true}},
	},
	"FRM-14": {
		code: "HS-FRM-14", en: "INCIDENT REPORT AND EMERGENCY RESPONSE REQUEST", fr: "Déclaration d’incident et demande d’intervention d’urgence",
		intro: [2]string{"Urgent? Call the HEKATE incident line and e-mail incident@hekatesystems.com. Do not switch off affected machines; disconnect them from the network if possible.", ""},
		top:   []hfield{{"Organisation", "", "client", 90}, {"Reported by", "Déclarant", "", 90}},
		top2:  []hfield{{"Telephone (24/7)", "", "", 50}, {"E-mail", "", "", 60}, {"Date & time detected", "Détecté le", "date", 70}},
		secs: []secDef{
			{bar: true, en: "What happened?", fr: "Que s’est-il passé ?", alias: []string{"what happened?", "what happened"}, minH: 5},
			{en: "Type", alias: []string{"type", "incident type"}, multi: true, perRow: 4, opts: []string{"Ransomware", "Phishing / BEC", "Account compromise", "Data breach / Fuite", "Malware", "DDoS", "Insider", "Fraud / Fraude", "Lost device", "Unknown / Inconnu"}},
			{en: "Description: what was seen, by whom, when", fr: "Description", alias: []string{"description", "description: what was seen, by whom, when"}, minH: 30},
			{bar: true, en: "Impact", fr: "Impact", alias: []string{"impact"}, minH: 5},
			{en: "Severity", fr: "Gravité", alias: []string{"severity", "gravité", "gravite"}, perRow: 2, opts: []string{"Critical - business stopped", "High - major service affected", "Medium", "Low"}},
			{en: "Systems affected", fr: "Systèmes touchés", alias: []string{"systems affected", "systèmes touchés"}, minH: 15},
			{en: "Users / data affected", fr: "Utilisateurs / données", alias: []string{"users / data affected", "users affected", "data affected"}, minH: 15},
			{en: "Personal data involved?", fr: "Données personnelles ?", alias: []string{"personal data involved?", "personal data involved", "personal data"}, perRow: 3, opts: []string{"Yes / Oui", "No / Non", "Unknown / Inconnu"}},
			{bar: true, en: "Actions already taken", fr: "Mesures déjà prises", alias: []string{"actions already taken", "actions taken", "mesures déjà prises"}, minH: 5},
			{en: "Containment, backups, contacts made", fr: "Confinement, sauvegardes, contacts", alias: []string{"containment, backups, contacts made", "containment"}, minH: 25},
			{en: "Notified", fr: "Notifiés", alias: []string{"notified", "notifiés"}, multi: true, perRow: 3, opts: []string{"Management", "Bank", "ANTIC / CIRT", "Insurer", "Police / Gendarmerie", "Regulator (COBAC...)"}},
		},
		signs: []signSpec{{title: "Reporter / Déclarant", clientN: true}},
	},
}

// packTitles holds the English and French titles of the pack's other numbered forms. Those print in the pack's
// title block and header cells with the entered sections as numbered bars; their own boxes are not reproduced.
var packTitles = map[string][2]string{
	"RPT":    {"PENETRATION TEST REPORT", "Rapport de test d’intrusion"},
	"FRM-06": {"TRANSMITTAL AND FAX COVER SHEET", "Bordereau d’envoi et page de garde fax"},
	"FRM-07": {"CLIENT INFORMATION AND ONBOARDING FORM", "Fiche d’identification et d’ouverture de compte client"},
	"FRM-08": {"SERVICE REQUEST AND REQUIREMENTS QUESTIONNAIRE", "Demande de service et questionnaire des besoins"},
	"FRM-09": {"PENETRATION TEST SCOPING FORM", "Fiche de cadrage du test d’intrusion"},
	"FRM-10": {"SECURITY ASSESSMENT PRE-ENGAGEMENT QUESTIONNAIRE", "Questionnaire préalable d’évaluation de sécurité"},
	"FRM-11": {"HECID E-KYC INTEGRATION REQUEST", "Demande d’intégration hecID e-KYC"},
	"FRM-12": {"BACKGROUND SCREENING REQUEST", "Demande de vérification d’antécédents"},
	"FRM-13": {"CLIENT KYC AND DUE-DILIGENCE FORM", "Fiche KYC et diligence raisonnable client"},
	"FRM-15": {"CUSTOMER FEEDBACK AND SATISFACTION FORM", "Questionnaire de satisfaction client"},
	"FRM-24": {"RATE CARD", "Grille tarifaire"},
	"FRM-26": {"PROJECT KICK-OFF RECORD", "Procès-verbal de lancement de projet"},
	"FRM-31": {"PROJECT CLOSURE AND SIGN-OFF", "Clôture et validation de projet"},
	"FRM-32": {"JOB APPLICATION FORM", "Formulaire de candidature"},
	"FRM-33": {"OFFER OF EMPLOYMENT", "Promesse d’embauche"},
	"FRM-37": {"VISITOR AND LABORATORY ACCESS LOG", "Registre des visiteurs et d’accès au laboratoire"},
	"FRM-39": {"SUPPLIER REGISTRATION FORM", "Fiche de référencement fournisseur"},
	"FRM-41": {"SUPPLIER EVALUATION FORM", "Évaluation fournisseur"},
}

// LayoutFaithful lists the narrative codes printed with the pack's own structure (header cells, section order,
// boxes, tables, check boxes, signature blocks), and FrameOnly those printed in the pack's title block and bars only.
// Used by the tests and the documentation.
func LayoutFaithful() []string {
	out := []string{"PR", "SOW", "OC", "FRM-23", "FRM-14"}
	for _, c := range packOrder {
		if _, ok := frameBlocks[c]; ok {
			out = append(out, c)
		}
	}
	return out
}

var packOrder = []string{"RPT", "FRM-06", "FRM-07", "FRM-08", "FRM-09", "FRM-10", "FRM-11", "FRM-12", "FRM-13", "FRM-15", "FRM-24", "FRM-26", "FRM-31", "FRM-32", "FRM-33", "FRM-37", "FRM-39", "FRM-41"}

func FrameOnly() []string {
	var out []string
	for _, c := range packOrder {
		if _, ok := frameBlocks[c]; !ok {
			out = append(out, c)
		}
	}
	return out
}

var normRE = regexp.MustCompile(`^[\s\d.)\-:]+`)

func normHeading(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = normRE.ReplaceAllString(s, "")
	s = strings.TrimRight(s, " :")
	return strings.Join(strings.Fields(s), " ")
}

// schemaFor finds the pack schema for a document. The reference carries the form code; a draft has none, so the
// heading (kind) is matched against the pack's English titles.
func schemaFor(ref, kind string) *schema {
	if m := refCode.FindStringSubmatch(ref); m != nil {
		if s, ok := schemas[m[1]]; ok {
			return s
		}
		if bs := blockSchema(m[1]); bs != nil {
			return bs
		}
		if t, ok := packTitles[m[1]]; ok {
			return &schema{code: "HS-" + m[1], en: t[0], fr: t[1]}
		}
		return nil
	}
	k := strings.ToLower(strings.TrimSpace(kind))
	for _, c := range []string{"PR", "SOW", "OC", "FRM-23", "FRM-14"} {
		if strings.ToLower(schemas[c].en) == k {
			return schemas[c]
		}
	}
	for c, t := range packTitles {
		if strings.ToLower(t[0]) == k {
			if bs := blockSchema(c); bs != nil {
				return bs
			}
			return &schema{code: "HS-" + c, en: t[0], fr: t[1]}
		}
	}
	return nil
}

func renderNarrative(o Options, ref, kind string, issued time.Time, title string, client Party, sections []Section, signedBy string, draft bool) ([]byte, error) {
	sc := schemaFor(ref, kind)
	if sc == nil {
		return renderNarrativePlain(o, ref, kind, issued, title, client, sections, signedBy, draft)
	}
	d := newDocMode(o, ref, kind, issued, draft)
	d.packTitle(sc.en, sc.fr, sc.code, issued)
	if draft {
		d.pdf.SetFont("Helvetica", "I", 8.5)
		d.pdf.CellFormat(0, 5, d.tr("Draft - not issued"), "", 1, "L", false, 0, "")
	}
	if sc.intro[0] != "" {
		if sc.intro[1] != "" {
			d.bilingualNote(sc.intro[0], sc.intro[1])
		} else {
			d.pdf.SetFont("Helvetica", "I", 8.5)
			d.pdf.MultiCell(0, 4.2, d.tr(sc.intro[0]), "", "L", false)
			d.pdf.Ln(1)
		}
	}
	if sc.blocks != nil {
		d.renderBlocks(sc, issued, title, client, sections, signedBy)
		return d.bytes()
	}
	val := func(key string) string {
		switch key {
		case "ref":
			return d.docNo()
		case "date":
			return date(issued)
		case "subject":
			if sc.code == "HS-FRM-23" && client.Name != "" { // the form has no client cell: keep the client with the project
				return strings.TrimSpace(title + " (" + client.Name + ")")
			}
			return title
		case "client":
			return client.Name
		}
		return ""
	}
	row := func(fs []hfield) {
		var cells []fcell
		for _, f := range fs {
			cells = append(cells, fc(f.label, f.fr, val(f.key), f.w))
		}
		if len(cells) > 0 {
			d.packFields(cells...)
		}
	}
	if sc.top == nil { // forms without a layout schema: document number, date, client, subject
		d.packFields(fc("Form no.", "N°", d.docNo(), 60), fc("Date", "", date(issued), 40), fc("Client", "", client.Name, 80))
		if title != "" {
			d.packFields(fc("Subject", "Objet", title, width))
		}
	}
	row(sc.top)
	row(sc.top2)
	if sc.partyBar {
		d.clientBlock("A", client)
	}

	if len(sc.secs) == 0 {
		for i, s := range sections {
			h := strings.TrimSpace(s.Heading)
			if h == "" {
				h = "Notes"
			}
			d.packBar(itoa(i+1), h, "")
			d.packBox("", "", s.Body, 12)
		}
	} else {
		d.fillSchema(sc, sections)
	}

	if strings.TrimSpace(sc.outro[0]) != "" {
		d.bilingualNote(sc.outro[0], sc.outro[1])
	}
	var signs []signCol
	n := 0
	for _, sg := range sc.signs {
		c := signCol{title: sg.title}
		if sg.hekate && strings.TrimSpace(signedBy) != "" {
			c.name = strings.TrimSpace(signedBy)
		}
		signs = append(signs, c)
		n++
	}
	if n == 0 {
		c := signCol{title: "For HEKATE SYSTEMS SARL"}
		if strings.TrimSpace(signedBy) != "" {
			c.name = strings.TrimSpace(signedBy)
		}
		signs = append(signs, c)
	}
	if sc.code == "HS-FRM-14" {
		d.packSign(signs...)
		d.packBar("", "For HEKATE use only", "Réservé à HEKATE")
		d.packFields(fc("Incident no.", "", "", 90), fc("Severity (HEKATE)", "", "", 90))
		d.packFields(fc("Responder assigned", "", "", 90), fc("Time acknowledged", "", "", 90))
		d.packFields(fc("Retainer / contract", "", "", 90), fc("Evidence case no.", "", "", 90))
	} else {
		d.packSign(signs...)
	}
	return d.bytes()
}

// fillSchema places the entered sections into the pack's sections, in the pack's order.
func (d *doc) fillSchema(sc *schema, sections []Section) {
	placed := make([][]Section, len(sc.secs))
	var extra []Section
	for _, s := range sections {
		h := normHeading(s.Heading)
		hit := -1
		if h != "" {
			for i, sd := range sc.secs {
				for _, a := range sd.alias {
					if a == h {
						hit = i
					}
				}
				if hit < 0 && slotMatches(h, sd.en, sd.fr) {
					hit = i
				}
				if hit >= 0 {
					break
				}
			}
		}
		if hit < 0 {
			extra = append(extra, s)
		} else {
			placed[hit] = append(placed[hit], s)
		}
	}
	bar := 0
	for i, sd := range sc.secs {
		var body string
		for k, s := range placed[i] {
			if k > 0 {
				body += "\n\n"
			}
			body += s.Body
		}
		body = strings.TrimSpace(body)
		switch {
		case sd.bar:
			bar++
			no := itoa(bar)
			if sd.letter != "" {
				no = sd.letter
			}
			d.packBar(no, sd.en, sd.fr)
			switch {
			case sd.fees:
				if body != "" {
					d.packBox("", "", body, 12)
				} else {
					d.packGrid(feeCols, nil, sd.rows)
					d.packTotals(Money{}, false)
				}
				continue
			case body != "":
				d.packBox("", "", body, sd.minH)
			case sd.grid != nil:
				d.packGrid(sd.grid, nil, sd.rows)
			case sd.fixedEN != "":
				d.bilingualNoteOpt(sd.fixedEN, sd.fixedFR)
			case sd.minH > 8:
				d.packBox("", "", "", sd.minH)
			}
		case sd.opts != nil:
			d.optionsFor(sd, body)
		default:
			if body == "" && sd.fixedEN != "" {
				d.bilingualNoteOpt(sd.fixedEN, sd.fixedFR)
			} else {
				d.packBox(sd.en, sd.fr, body, sd.minH)
			}
		}
	}
	if len(extra) > 0 {
		d.packBar("", "Additional sections", "Sections complémentaires")
		for _, s := range extra {
			h := strings.TrimSpace(s.Heading)
			if h == "" {
				h = "Notes"
			}
			d.packBox(h, "", s.Body, 10)
		}
	}
}

func (d *doc) bilingualNoteOpt(en, fr string) {
	if fr == "" {
		d.note(en)
		return
	}
	d.bilingualNote(en, fr)
}

// optionTokens are the words of an option that identify it in entered text ("Phishing / BEC" -> phishing, bec).
func optionTokens(o string) []string {
	var out []string
	for _, p := range strings.Split(o, " / ") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
		if i := strings.Index(p, " - "); i > 0 {
			out = append(out, p[:i])
		}
	}
	return out
}

// tickOptions decides which options the entered text names. Single-choice groups tick the option with the longest
// matching word ("approved with conditions" ticks that one, not plain "approved"); nothing entered, nothing ticked.
// rest is the entered text with the option words removed, to tell whether anything else was said.
func tickOptions(sd secDef, body string) (ticked map[string]bool, rest string) {
	ticked = map[string]bool{}
	low := strings.ToLower(body)
	best, bestLen := "", 0
	rest = low
	for _, o := range sd.opts {
		for _, t := range optionTokens(o) {
			if !strings.Contains(low, t) {
				continue
			}
			if sd.multi {
				ticked[o] = true
			} else if len(t) > bestLen {
				best, bestLen = o, len(t)
			}
			rest = strings.ReplaceAll(rest, t, "")
		}
	}
	if best != "" {
		ticked[best] = true
	}
	return ticked, rest
}

// optionsFor draws check boxes, ticking the options named in the entered text, and any other text in a box.
func (d *doc) optionsFor(sd secDef, body string) {
	ticked, rest := tickOptions(sd, body)
	d.packChecks(sd.en, sd.fr, sd.opts, ticked, sd.perRow)
	if body != "" && len(strings.Trim(rest, " ,.;:-\n/")) > 3 {
		d.packBox("", "", body, 8)
	}
}

// renderNarrativePlain is the brand-framed text layout for narrative kinds the pack has no form for (letter,
// certificate, guarantee policy, contract, penetration test report) and for documents of unknown kind.
func renderNarrativePlain(o Options, ref, kind string, issued time.Time, title string, client Party, sections []Section, signedBy string, draft bool) ([]byte, error) {
	d := newDocMode(o, ref, kind, issued, draft)
	sub := ref
	if draft {
		sub = "Draft - not issued"
	}
	d.title(strings.ToUpper(kind), sub)
	d.kv("Date", date(issued))
	d.kv("Subject", title)
	d.pdf.Ln(2)
	d.party("Prepared for", client)
	for _, s := range sections {
		d.pdf.Ln(2)
		if strings.TrimSpace(s.Heading) != "" {
			d.pdf.SetFont("Helvetica", "B", 11)
			d.pdf.CellFormat(0, 7, d.tr(s.Heading), "", 1, "L", false, 0, "")
		}
		d.pdf.SetFont("Helvetica", "", 10)
		d.pdf.MultiCell(0, 5, d.tr(s.Body), "", "L", false)
	}
	if strings.TrimSpace(signedBy) != "" {
		d.pdf.Ln(10)
		d.pdf.SetFont("Helvetica", "", 10)
		d.pdf.CellFormat(0, 5, d.tr("For HEKATE SYSTEMS SARL: "+signedBy), "", 1, "L", false, 0, "")
	}
	return d.bytes()
}
