package docs

// Box-level layouts for the pack's numbered forms that are printed from a block list (HS-FRM-06 ... 41). Each form
// is transcribed from the approved pack PDF in web/public/forms: the same section bars, bilingual field cells,
// check-box groups, tables (with the pack's own row labels) and signature blocks, in the pack's order. Only wording
// that is printed in the pack is reproduced; every value cell stays blank unless the document carries the value
// (date, client name, subject) or the author typed it under that cell's heading.
//
// Entered sections are matched to a cell, check group, table or box by heading (English, French or "English /
// French", as the web editor pre-fills them). Text for a cell goes in the cell; text for a check group ticks the
// options it names; text for a table is read as "row | value | value" lines; text for a box goes in the box. Sections
// that match nothing are printed under "Additional sections", never dropped.

import (
	"strings"
	"time"
)

type blkKind int

const (
	bBar blkKind = iota
	bCells
	bChecks
	bTable
	bBoxK
	bNoteK
	bTextK
	bSignK
	bUseK
)

// hc is one field cell of a block row. key says which document value fills it when nothing was typed for it:
// "date", "client", "subject", "rccm", "email".
type hc struct {
	en, fr string
	w      float64
	key    string
}

type tblDef struct {
	cols      []gcol
	rows      [][]string // pack rows with their fixed text; other cells blank
	n         int        // minimum number of ruled rows
	fixedCols int        // columns holding the pack's fixed text
	keyCol    int        // column that names a fixed row
	scoreAt   int        // first score column (header = the score), 0 when none
	scoreN    int
}

type blk struct {
	kind   blkKind
	no     bool // bar: numbered
	en, fr string
	cells  []hc
	opts   []string
	multi  bool
	perRow int
	tbl    *tblDef
	minH   float64
	signs  []signSpec
}

func fcl(en, fr string, w float64) hc { return hc{en: en, fr: fr, w: w} }
func fck(en, fr string, w float64, key string) hc {
	return hc{en: en, fr: fr, w: w, key: key}
}
func row(cells ...hc) blk { return blk{kind: bCells, cells: cells} }
func bar(en, fr string) blk {
	return blk{kind: bBar, no: true, en: en, fr: fr}
}
func barU(en, fr string) blk { return blk{kind: bBar, en: en, fr: fr} }
func chk(en, fr string, perRow int, multi bool, opts ...string) blk {
	return blk{kind: bChecks, en: en, fr: fr, perRow: perRow, multi: multi, opts: opts}
}
func box(en, fr string, minH float64) blk { return blk{kind: bBoxK, en: en, fr: fr, minH: minH} }
func note(en, fr string) blk              { return blk{kind: bNoteK, en: en, fr: fr} }
func text(en string) blk                  { return blk{kind: bTextK, en: en} }
func signs(s ...signSpec) blk             { return blk{kind: bSignK, signs: s} }
func use(cells ...hc) blk                 { return blk{kind: bUseK, cells: cells} }
func table(label, fr string, t tblDef) blk {
	return blk{kind: bTable, en: label, fr: fr, tbl: &t}
}

// fixedRows builds table rows from the pack's fixed texts: one row per entry, entries laid out from column 0.
func fixedRows(cols int, texts ...[]string) [][]string {
	out := make([][]string, len(texts))
	for i, t := range texts {
		r := make([]string, cols)
		copy(r, t)
		out[i] = r
	}
	return out
}

func col1(a ...string) [][]string {
	out := make([][]string, len(a))
	for i, s := range a {
		out[i] = []string{s}
	}
	return out
}

const (
	certifyEN  = "I certify that the information provided is accurate and authorise HEKATE SYSTEMS SARL to verify it and to process it for the purpose of the business relationship, in accordance with its Data Protection Policy."
	certifyFR  = "Je certifie l’exactitude des informations fournies et autorise HEKATE SYSTEMS SARL à les vérifier et à les traiter aux fins de la relation d’affaires, conformément à sa politique de protection des données."
	pentestEN  = "The signatory confirms that the client owns or is authorised to have tested every target listed above, and authorises HEKATE SYSTEMS SARL to test them within the limits of this form and of the Rules of Engagement."
	pentestFR  = "Le signataire confirme que le client est propriétaire de chaque cible listée ou dûment autorisé, et autorise HEKATE SYSTEMS SARL à les tester dans les limites du présent formulaire et des règles d’engagement."
	dpaEN      = "Integration is subject to the Data Processing Addendum (HS-DOC-34) and to a lawful basis and consent for each data subject (HS-DOC-32)."
	dpaFR      = "L’intégration est soumise à l’avenant sur le traitement des données (HS-DOC-34) et à une base légale et au consentement de chaque personne (HS-DOC-32)."
	applyEN    = "I certify the information above is true and consent to background screening (HS-DOC-32) if I am shortlisted."
	applyFR    = "Je certifie l’exactitude de ces informations et accepte une vérification d’antécédents (HS-DOC-32) si je suis présélectionné(e)."
	offerEN    = "Following our recent interviews, HEKATE SYSTEMS SARL is pleased to offer you employment on the terms below, subject to satisfactory background screening and to the signature of the employment contract (HS-DOC-35)."
	offerFR    = "Suite à nos entretiens, HEKATE SYSTEMS SARL a le plaisir de vous proposer un emploi aux conditions ci-dessous, sous réserve d’une vérification d’antécédents satisfaisante et de la signature du contrat de travail (HS-DOC-35)."
	offerAccEN = "Please confirm acceptance by signing and returning this letter within 7 days. This offer lapses after that date."
	offerAccFR = "Merci de confirmer votre accord en retournant ce courrier signé sous 7 jours, faute de quoi l’offre deviendra caduque."
	supplierEN = "The supplier confirms the accuracy of this information, complies with the HEKATE Code of Conduct and anti-bribery rules (HS-DOC-28), and will notify any change of bank details in writing, signed by its legal representative."
	supplierFR = "Le fournisseur certifie ces informations, respecte le Code de conduite et les règles anticorruption HEKATE (HS-DOC-28) et notifiera par écrit signé tout changement de coordonnées bancaires."
	faxEN      = "CONFIDENTIAL. This transmission is intended only for the addressee. If you received it in error, notify the sender and destroy it; any use or disclosure is prohibited."
	faxFR      = "CONFIDENTIEL. Destiné exclusivement au destinataire ; en cas d’erreur, prévenir l’expéditeur et détruire ; toute utilisation est interdite."
)

var signClient = func(t string) signSpec { return signSpec{title: t} }
var signHekate = func(t string) signSpec { return signSpec{title: t, hekate: true} }

// frameBlocks holds the block layouts, keyed by the form code ("FRM-06").
var frameBlocks = map[string][]blk{
	"FRM-06": {
		row(fcl("To", "À", 90), fcl("From", "De", 90)),
		row(fck("Organisation", "", 70, "client"), fck("Date", "", 50, "date"), fcl("Pages (incl. this)", "Pages", 60)),
		row(fcl("E-mail / fax", "", 90), fcl("Our ref.", "N/Réf.", 90)),
		chk("Purpose", "Objet", 3, true, "For your information / Pour information", "For approval / Pour approbation", "For signature / Pour signature", "For action / Pour suite à donner", "As requested / Suite à votre demande"),
		table("Documents", "", tblDef{cols: []gcol{{"No.", "", 12, "L"}, {"Document", "", 80, "L"}, {"Copies", "", 20, "L"}, {"Remarks", "Observations", 68, "L"}},
			rows: fixedRows(4, []string{"1"}, []string{"2"}, []string{"3"}, []string{"4"}, []string{"5"}, []string{"6"}), n: 6, fixedCols: 1}),
		box("Message", "", 36),
		note(faxEN, faxFR),
	},
	"FRM-07": {
		text("Type directly into the fields or complete in block capitals. Return to sales@hekatesystems.com with the documents listed in section 5."),
		bar("Organisation", "Organisation"),
		row(fck("Legal name", "Raison sociale", 90, "client"), fcl("Trading name", "Nom commercial", 90)),
		row(fcl("Legal form", "Forme juridique", 60), fck("RCCM no.", "N° RCCM", 60, "rccm"), fcl("NIU", "NIU", 60)),
		row(fcl("Registered address", "Siège", 90), fcl("City, country", "Ville, pays", 90)),
		row(fcl("Website", "Site", 90), fcl("Main telephone", "Téléphone", 90)),
		chk("Sector", "Secteur", 4, true, "Banking & finance", "Energy", "Insurance", "Education", "Telecom", "Retail", "Government", "NGO", "Health", "Other / Autre"),
		chk("Size", "Effectif", 5, false, "1–10", "11–50", "51–250", "251–1000", "> 1000"),
		bar("Contacts", "Interlocuteurs"),
		table("Contacts", "", tblDef{cols: []gcol{{"Role", "Rôle", 38, "L"}, {"Name", "Nom", 42, "L"}, {"Title", "Fonction", 30, "L"}, {"E-mail", "", 40, "L"}, {"Telephone", "", 30, "L"}},
			rows: fixedRows(5, []string{"Primary contact / Principal"}, []string{"Technical / Technique"}, []string{"Billing / Facturation"}, []string{"Escalation / Escalade"}), n: 4, fixedCols: 1}),
		bar("Billing", "Facturation"),
		row(fcl("Invoice address", "Adresse de facturation", 90), fcl("PO required?", "Bon de commande requis ?", 90)),
		row(fck("Invoice e-mail", "E-mail facturation", 90, "email"), fcl("Tax exemption ref.", "Réf. exonération", 90)),
		row(fcl("Payment terms", "Délai de paiement", 180)),
		bar("Services of interest", "Services souhaités"),
		chk("Services", "", 3, true, "Penetration testing", "SOC / HCDP monitoring", "Digital forensics", "Background screening", "Training", "Vulnerability assessment", "Incident response", "hecID e-KYC", "Security audit / ISO 27001", "Consulting"),
		bar("Documents to attach", "Pièces à joindre"),
		chk("Attached", "Joint", 2, true, "RCCM extract / Extrait RCCM", "Legal representative ID / CNI du représentant", "NIU certificate / Attestation NIU", "Proof of address / Justificatif d’adresse", "Signed NDA (HS-DOC-26) / NDA signé"),
		note(certifyEN, certifyFR),
		signs(signClient("Authorised signatory / Signataire habilité")),
		barU("For HEKATE use only", "Réservé à HEKATE"),
		use(fcl("Client ID", "", 90), fcl("Account manager", "", 90), fcl("KYC result (HS-FRM-13)", "", 90), fcl("Approved by", "", 90), fcl("Date opened", "", 90), fcl("CRM entry", "", 90)),
	},
	"FRM-08": {
		row(fck("Organisation", "", 100, "client"), fck("Request date", "Date", 80, "date")),
		row(fcl("Contact", "Interlocuteur", 90), fcl("E-mail / Telephone", "", 90)),
		bar("What do you need?", "Votre besoin"),
		chk("Service", "", 2, true, "Penetration test → also complete HS-FRM-09", "SOC / HCDP monitoring", "Digital forensics", "Background screening → HS-FRM-12", "Audit / compliance", "Security assessment → HS-FRM-10", "Incident response (urgent → HS-FRM-14)", "hecID e-KYC → HS-FRM-11", "Training / awareness", "Other / Autre"),
		box("Describe the need and the expected outcome", "Décrivez le besoin et le résultat attendu", 32),
		bar("Drivers", "Motivations"),
		chk("Reason", "Raison", 2, true, "Regulatory (COBAC, ANTIC, BEAC) / Réglementaire", "Audit finding / Constat d’audit", "New system launch / Mise en production", "Client or partner request", "Recent incident / Incident récent", "Proactive / Préventif"),
		bar("Environment", "Environnement"),
		row(fcl("Number of users", "Utilisateurs", 45), fcl("Servers", "Serveurs", 45), fcl("Sites", "Sites", 45), fcl("Cloud provider", "Cloud", 45)),
		box("Key systems and technologies (ERP, core banking, AD, M365…)", "Systèmes clés", 22),
		bar("Timing and budget", "Délais et budget"),
		row(fcl("Desired start", "Début souhaité", 60), fcl("Deadline", "Échéance", 60), fcl("Budget range (FCFA)", "Budget", 60)),
		chk("Decision process", "Processus de décision", 3, false, "Direct order", "Internal tender", "Public tender (ARMP) / Appel d’offres public"),
		signs(signClient("Requester / Demandeur")),
		barU("For HEKATE use only", "Réservé à HEKATE"),
		use(fcl("Request No.", "", 90), fcl("Assigned to", "", 90), fcl("Qualification", "", 90), fcl("Quotation No.", "", 90)),
	},
	"FRM-09": {
		text("This form defines exactly what may be tested. Nothing outside it will be touched. It becomes part of the Rules of Engagement (HS-DOC25)."),
		row(fck("Client", "", 120, "client"), fck("Project ref.", "Réf. projet", 60, "subject")),
		bar("Test type", "Type de test"),
		chk("Type", "", 3, true, "External network", "Web application", "Mobile app (Android / iOS)", "Cloud (AWS / Azure / GCP)", "Physical", "Internal network", "API", "Wi-Fi", "Social engineering / phishing", "Red team"),
		chk("Approach", "Approche", 3, false, "Black box / Boîte noire", "Grey box / Boîte grise", "White box / Boîte blanche"),
		bar("Targets in scope", "Cibles autorisées"),
		table("Targets in scope", "", tblDef{cols: []gcol{{"Target (URL, IP / range / app)", "Cible", 72, "L"}, {"Environment", "Env.", 26, "L"}, {"Owner", "Propriétaire", 38, "L"}, {"Notes", "", 44, "L"}}, n: 5}),
		bar("Exclusions", "Exclusions"),
		box("Systems, hosts or actions not to be tested", "Systèmes ou actions exclus", 26),
		bar("Constraints", "Contraintes"),
		row(fcl("Testing window", "Plage horaire", 90), fcl("Start date", "Début", 45), fcl("End date", "Fin", 45)),
		row(fcl("Source IPs of HEKATE (whitelisting)", "IP sources", 90), fcl("Test accounts provided", "Comptes de test", 90)),
		chk("Allowed", "Autorisé", 3, true, "Denial-of-service testing", "Production testing", "Exploitation of findings", "Password attacks", "Data exfiltration proof"),
		bar("Emergency contacts", "Contacts d’urgence"),
		table("Emergency contacts", "", tblDef{cols: []gcol{{"Party", "Partie", 40, "L"}, {"Name", "Nom", 50, "L"}, {"Telephone (24/7)", "", 45, "L"}, {"E-mail", "", 45, "L"}},
			rows: fixedRows(4, []string{"Client"}, []string{"Client (backup)"}, []string{"HEKATE"}), n: 3, fixedCols: 1}),
		bar("Third parties", "Tiers"),
		chk("Hosting / cloud provider permission obtained?", "Autorisation de l’hébergeur obtenue ?", 3, false, "Yes / Oui", "No / Non", "Not applicable"),
		note(pentestEN, pentestFR),
		signs(signClient("Client authorising officer / Autorité client"), signHekate("HEKATE engagement lead / Responsable HEKATE")),
	},
	"FRM-10": {
		text("Rate each control: 0 = none, 1 = informal, 2 = defined, 3 = managed and measured."),
		row(fck("Organisation", "", 100, "client"), fcl("Completed by", "Rempli par", 80)),
		table("Controls", "", tblDef{cols: []gcol{{"Domain", "Domaine", 26, "L"}, {"Control", "Mesure", 80, "L"}, {"0", "", 9, "C"}, {"1", "", 9, "C"}, {"2", "", 9, "C"}, {"3", "", 9, "C"}, {"Comment", "", 38, "L"}},
			rows: fixedRows(7,
				[]string{"Governance", "Security policy approved by management"}, []string{"", "Named security owner (CISO/RSSI)"}, []string{"", "Risk assessment in the last 12 months"},
				[]string{"Assets", "Hardware and software inventory"}, []string{"", "Data classification"}, []string{"", "Cloud and SaaS inventory"},
				[]string{"Identity", "MFA for remote and admin access"}, []string{"", "Joiner / mover / leaver process"}, []string{"", "Privileged account management"},
				[]string{"Protection", "Patch management within 30 days"}, []string{"", "Endpoint protection / EDR"}, []string{"", "E-mail filtering and DMARC"},
				[]string{"Detection", "Central log collection (SIEM)"}, []string{"", "24/7 monitoring"}, []string{"", "Vulnerability scanning"},
				[]string{"Response", "Incident response plan tested"}, []string{"", "Backups tested and offline"}, []string{"", "Business continuity plan"},
				[]string{"People", "Security awareness training"}, []string{"", "Background checks for key staff"}, []string{"", "Third-party security reviews"}),
			n: 21, fixedCols: 2, keyCol: 1, scoreAt: 2, scoreN: 4}),
		box("Main concerns", "Principales préoccupations", 26),
		chk("Compliance obligations", "Obligations", 3, true, "COBAC / BEAC", "ISO 27001", "ANTIC", "GDPR / RGPD", "PCI DSS", "Law 2024-017 (data protection)"),
		signs(signClient("Respondent / Répondant")),
	},
	"FRM-11": {
		row(fck("Organisation", "", 90, "client"), fcl("Technical contact", "Contact technique", 90)),
		row(fcl("Use case", "Cas d’usage", 120), fcl("Go-live target", "Mise en service", 60)),
		bar("Verification services", "Services de vérification"),
		chk("Checks", "", 2, true, "ID document authentication / Document d’identité", "NIU verification", "Address verification", "Ongoing monitoring", "Face match & liveness / Biométrie faciale", "Phone / SIM verification", "Sanctions & PEP screening"),
		chk("Documents", "", 3, true, "CNI (Cameroon)", "Driving licence / Permis", "Passport / Passeport", "CEMAC ID", "Residence permit / Carte de séjour"),
		bar("Volumes and integration", "Volumes et intégration"),
		row(fcl("Expected checks / month", "Contrôles par mois", 60), fcl("Peak / day", "Pic par jour", 60), fcl("Channels", "Canaux", 60)),
		chk("Integration", "", 3, true, "REST API", "Mobile SDK (Android / iOS)", "Web SDK", "hecID portal (no code)", "Batch file"),
		row(fcl("Callback / webhook URL", "", 90), fcl("IP allow-list", "Liste blanche IP", 90)),
		bar("Data protection", "Protection des données"),
		row(fcl("Data retention (days)", "Conservation (jours)", 60), fcl("Data hosting requirement", "Hébergement", 60), fcl("DPO contact", "Contact DPD", 60)),
		note(dpaEN, dpaFR),
		signs(signClient("Client / Client")),
		barU("For HEKATE use only", "Réservé à HEKATE"),
		use(fcl("Tenant ID", "", 90), fcl("API keys issued", "", 90), fcl("Sandbox date", "", 90), fcl("Production date", "", 90)),
	},
	"FRM-12": {
		text("One form per candidate. The candidate’s signed consent (HS-DOC-32) must be attached; no check starts without it."),
		bar("Requesting organisation", "Organisation demanderesse"),
		row(fck("Organisation", "", 90, "client"), fcl("Requester", "Demandeur", 90)),
		row(fcl("Position applied for", "Poste", 90), fcl("Purpose", "Finalité", 90)),
		bar("Candidate", "Candidat"),
		row(fcl("Surname", "Nom", 90), fcl("Given names", "Prénoms", 90)),
		row(fcl("ID document no.", "N° pièce", 60), fcl("Nationality", "Nationalité", 60), fcl("Date of birth", "Né(e) le", 60)),
		row(fcl("Current address", "Adresse", 90), fcl("Telephone", "", 45), fcl("E-mail", "", 45)),
		bar("Checks requested", "Vérifications demandées"),
		chk("Checks", "", 2, true, "Identity (hecID) / Identité", "Employment history / Parcours", "Professional licences", "Education / Diplômes", "References / Références", "Criminal record extract (bulletin n°3), supplied by candidate", "Sanctions & PEP", "Credit (where lawful) / Solvabilité"),
		chk("Turnaround", "Délai", 2, false, "Standard (10 working days)", "Express (5 working days)"),
		chk("Consent attached", "Consentement joint", 1, false, "Yes — HS-DOC-32 signed / Oui"),
		signs(signClient("Requester / Demandeur")),
		barU("For HEKATE use only", "Réservé à HEKATE"),
		use(fcl("Case No.", "", 90), fcl("Analyst", "", 90), fcl("Date completed", "", 90), fcl("Result sent to", "", 90)),
	},
	"FRM-13": {
		text("Completed by HEKATE before any engagement, under its anti-money-laundering and anti-corruption obligations (HS-DOC-28)."),
		bar("Client identification", "Identification du client"),
		row(fck("Legal name", "Raison sociale", 120, "client"), fcl("Client ID", "", 60)),
		row(fck("RCCM no.", "", 60, "rccm"), fcl("NIU", "", 50), fcl("Country of registration", "Pays", 70)),
		bar("Ownership and control", "Actionnariat et contrôle"),
		table("Beneficial owners", "", tblDef{cols: []gcol{{"Beneficial owner", "Bénéficiaire effectif", 58, "L"}, {"% held", "%", 20, "L"}, {"Nationality", "", 34, "L"}, {"ID verified (hecID)", "", 40, "L"}, {"PEP?", "", 28, "L"}}, n: 3}),
		table("Directors", "", tblDef{cols: []gcol{{"Director", "legal representative", 58, "L"}, {"Position", "Fonction", 54, "L"}, {"ID verified", "", 40, "L"}, {"PEP?", "", 28, "L"}}, n: 2}),
		bar("Risk screening", "Filtrage des risques"),
		chk("Results", "Résultats", 2, true, "UN / EU / OFAC sanctions: no match", "Adverse media: none", "PEP identified", "High-risk country link", "State-owned entity", "Unusual payment route requested"),
		chk("Risk rating", "Niveau de risque", 3, false, "Low / Faible", "Medium / Moyen", "High / Élevé — enhanced due diligence"),
		box("Analyst comments", "Commentaires", 26),
		chk("Decision", "Décision", 4, false, "Approved / Approuvé", "Approved with conditions", "Rejected / Rejeté", "Escalated to management"),
		signs(signHekate("Compliance analyst / Analyste conformité"), signClient("Approved by (manager) / Approuvé par")),
		row(fcl("Next review date", "Prochaine revue", 90), fcl("File reference", "Réf. dossier", 90)),
	},
	"FRM-15": {
		text("1 = very dissatisfied … 5 = very satisfied."),
		row(fck("Organisation", "", 90, "client"), fck("Project / service", "Projet", 90, "subject")),
		table("Ratings", "", tblDef{cols: []gcol{{"Criterion", "Critère", 110, "L"}, {"1", "", 14, "C"}, {"2", "", 14, "C"}, {"3", "", 14, "C"}, {"4", "", 14, "C"}, {"5", "", 14, "C"}},
			rows: fixedRows(6, []string{"Understanding of your needs"}, []string{"Quality of deliverables"}, []string{"Technical expertise"}, []string{"Communication and reporting"}, []string{"Respect of deadlines"}, []string{"Confidentiality and professionalism"}, []string{"Value for money"}, []string{"Overall satisfaction"}),
			n:    8, fixedCols: 1, scoreAt: 1, scoreN: 5}),
		chk("Would you recommend HEKATE?", "Recommanderiez-vous HEKATE ?", 3, false, "Yes / Oui", "Maybe / Peut-être", "No / Non"),
		box("What did we do well?", "Points forts", 24),
		box("What should we improve?", "Axes d’amélioration", 24),
		chk("May we quote you as a reference?", "Pouvons-nous vous citer ?", 2, false, "Yes / Oui", "No / Non"),
		signs(signClient("Respondent / Répondant")),
	},
	"FRM-24": {
		text("Indicative rates excluding tax. Final prices are set in each quotation according to scope and urgency."),
		table("Rates", "", tblDef{cols: []gcol{{"Service", "Prestation", 86, "L"}, {"Unit", "Unité", 34, "L"}, {"Standard (FCFA HT)", "", 30, "R"}, {"Urgent (FCFA HT)", "", 30, "R"}},
			rows: fixedRows(4,
				[]string{"Penetration test — web application", "per application"}, []string{"Penetration test — external network", "per 16 IPs"}, []string{"Penetration test — internal network", "per site"},
				[]string{"Mobile application test (Android / iOS)", "per app"}, []string{"Red-team exercise", "per engagement"}, []string{"Vulnerability assessment", "per 50 hosts"},
				[]string{"Security audit / ISO 27001 gap analysis", "per day"}, []string{"SOC monitoring — HCDP", "per month"}, []string{"Incident response retainer", "per month"},
				[]string{"Emergency incident response", "per day"}, []string{"Digital forensics investigation", "per day"}, []string{"hecID e-KYC verification", "per check"},
				[]string{"Background screening", "per candidate"}, []string{"Security awareness training", "per session"}, []string{"Consulting — senior consultant", "per day"}, []string{"Consulting — consultant", "per day"}),
			n: 16, fixedCols: 2, keyCol: 0}),
		row(fcl("Valid from", "Applicable à compter du", 90), fcl("Approved by", "Approuvé par", 90)),
	},
	"FRM-26": {
		row(fck("Project", "Projet", 90, "subject"), fcl("SOW / PO ref.", "", 90)),
		row(fcl("Location", "Lieu", 60), fcl("Facilitator", "Animateur", 60), fck("Date", "", 60, "date")),
		bar("Attendees", "Participants"),
		table("Attendees", "", tblDef{cols: []gcol{{"Name", "Nom", 62, "L"}, {"Organisation", "", 46, "L"}, {"Role", "Rôle", 40, "L"}, {"Signature", "", 32, "L"}}, n: 8}),
		bar("Confirmed", "Validé en réunion"),
		chk("Items", "", 2, true, "Scope and exclusions", "Schedule and milestones", "Emergency contacts", "Rules of engagement (HS-DOC-25)", "Deliverables and acceptance criteria", "Communication plan", "Access and accounts", "NDA in place"),
		bar("Actions", "Actions"),
		table("Actions", "", tblDef{cols: []gcol{{"Action", "", 78, "L"}, {"Owner", "Responsable", 42, "L"}, {"Due", "Échéance", 30, "L"}, {"Status", "", 30, "L"}}, n: 5}),
		box("Risks, decisions and notes", "Risques, décisions, notes", 26),
		signs(signClient("Client sponsor / Sponsor client"), signHekate("HEKATE project manager")),
	},
	"FRM-31": {
		row(fck("Project", "Projet", 90, "subject"), fck("Closure date", "Date", 90, "date")),
		row(fck("Client", "", 90, "client"), fcl("Project manager", "Chef de projet", 90)),
		bar("Closure checklist", "Liste de clôture"),
		chk("Done", "Fait", 2, true, "All deliverables accepted (HS-FRM-27)", "Retest completed", "Test data and tools removed from client systems", "Evidence archived per retention policy", "Feedback requested (HS-FRM-15)", "Final report delivered", "Client accounts disabled", "Client data returned or securely destroyed", "Final invoice issued", "Lessons learned recorded"),
		bar("Outcome", "Bilan"),
		row(fcl("Budget planned", "Prévu (FCFA)", 60), fcl("Budget actual", "Réel (FCFA)", 60), fcl("Schedule variance", "Écart délai", 60)),
		box("Lessons learned", "Retour d’expérience", 26),
		box("Follow-up opportunities", "Suites possibles", 22),
		signs(signClient("Client sponsor / Sponsor client"), signHekate("HEKATE CEO / Directeur Général")),
	},
	"FRM-32": {
		text("Personal data is used only for recruitment and kept 12 months unless you are hired."),
		row(fcl("Position applied for", "Poste", 120), fcl("Reference", "Réf.", 60)),
		bar("Personal details", "État civil"),
		row(fcl("Surname", "Nom", 90), fcl("Given names", "Prénoms", 90)),
		row(fcl("Nationality", "Nationalité", 60), fcl("Telephone", "", 60), fcl("Date of birth", "Né(e) le", 60)),
		row(fcl("E-mail", "", 90), fcl("Address", "Adresse", 90)),
		row(fcl("Availability", "Disponibilité", 180)),
		bar("Education", "Formation"),
		table("Education", "", tblDef{cols: []gcol{{"Year", "Année", 24, "L"}, {"Institution", "", 70, "L"}, {"Diploma", "Diplôme", 46, "L"}, {"Field", "Spécialité", 40, "L"}}, n: 4}),
		bar("Certifications", "Certifications"),
		chk("Held", "Obtenues", 4, true, "OSCP", "eJPT", "CEH", "GIAC", "CISSP", "ISO 27001 LI/LA", "CISM", "CCNA", "Security+", "Other / Autre"),
		bar("Experience", "Expérience"),
		table("Experience", "", tblDef{cols: []gcol{{"From–to", "Période", 30, "L"}, {"Employer", "Employeur", 46, "L"}, {"Position", "Poste", 44, "L"}, {"Main tasks", "Missions", 60, "L"}}, n: 4}),
		bar("Languages", "Langues"),
		table("Languages", "", tblDef{cols: []gcol{{"Language", "Langue", 70, "L"}, {"Spoken", "Parlé", 55, "L"}, {"Written", "Écrit", 55, "L"}},
			rows: fixedRows(3, []string{"English"}, []string{"Français"}), n: 3, fixedCols: 1}),
		box("Why HEKATE?", "Pourquoi HEKATE ?", 24),
		row(fcl("Expected salary (FCFA/month)", "Prétentions", 90), fcl("Notice period", "Préavis", 90)),
		note(applyEN, applyFR),
		signs(signClient("Applicant / Candidat")),
	},
	"FRM-33": {
		row(fck("To", "À", 120, "client"), fck("Date", "", 60, "date")),
		row(fcl("Address", "Adresse", 180)),
		text("Dear ____________________,"),
		note(offerEN, offerFR),
		row(fcl("Position", "Poste", 90), fcl("Category", "Catégorie", 90)),
		row(fcl("Start date", "Date d’entrée", 60), fcl("Contract type", "Type de contrat", 60), fcl("Trial period", "Période d’essai", 60)),
		row(fcl("Gross monthly salary (FCFA)", "Salaire brut", 90), fcl("Allowances", "Primes et indemnités", 90)),
		row(fcl("Place of work", "Lieu de travail", 90), fcl("Reports to", "Rattachement", 90)),
		note(offerAccEN, offerAccFR),
		signs(signHekate("HEKATE SYSTEMS SARL — Jude Nshome, CEO"), signClient("Accepted by the candidate / Lu et accepté")),
	},
	"FRM-37": {
		text("Every visitor shows ID, wears a visible badge, is escorted at all times and never enters the laboratory without written authorisation."),
		row(fcl("Site", "", 90), fck("Date", "", 45, "date"), fcl("Register period", "Période", 45)),
		table("Visitors", "", tblDef{cols: []gcol{{"Name", "Nom", 32, "L"}, {"Organisation", "", 28, "L"}, {"ID No.", "N° pièce", 22, "L"}, {"Host", "Hôte", 24, "L"}, {"Badge", "", 14, "L"}, {"In", "Entrée", 14, "L"}, {"Out", "Sortie", 14, "L"}, {"Lab Y/N", "", 12, "L"}, {"Signature", "", 20, "L"}}, n: 14}),
	},
	"FRM-39": {
		bar("Company", "Société"),
		row(fck("Legal name", "Raison sociale", 120, "client"), fcl("Legal form", "Forme", 60)),
		row(fck("RCCM no.", "", 60, "rccm"), fcl("NIU", "", 50), fcl("Tax regime", "Régime fiscal", 70)),
		row(fcl("Address", "Adresse", 120), fcl("Website", "", 60)),
		row(fcl("Contact", "Interlocuteur", 70), fcl("E-mail", "", 60), fcl("Telephone", "", 50)),
		chk("Category", "Catégorie", 3, true, "Hardware / Matériel", "Training", "Software & licences", "Professional services", "Cloud / hosting", "Facilities", "Telecom / Internet", "Other / Autre"),
		bar("Bank details", "Coordonnées bancaires"),
		row(fcl("Bank", "Banque", 90), fcl("Account holder", "Titulaire", 90)),
		row(fcl("Account no. (RIB)", "N° de compte (RIB)", 70), fcl("Payment ref.", "Réf. paiement", 55), fcl("SWIFT / BIC", "", 55)),
		bar("Security and compliance", "Sécurité et conformité"),
		chk("Provided", "Fourni", 2, true, "RCCM extract", "Tax clearance (attestation de non-redevance)", "ISO 27001 or equivalent", "Will process HEKATE or client data", "NIU certificate", "CNPS clearance", "Signed NDA (HS-DOC-26)"),
		note(supplierEN, supplierFR),
		signs(signClient("Supplier legal representative / Représentant légal")),
		barU("For HEKATE use only", "Réservé à HEKATE"),
		use(fcl("Supplier ID", "", 90), fcl("KYC / sanctions check", "", 90), fcl("Bank details verified by call-back", "", 90), fcl("Approved by", "", 90)),
	},
	"FRM-41": {
		text("Score 1 (poor) to 5 (excellent). Below 3 on security or overall triggers a review."),
		row(fck("Supplier", "", 90, "client"), fcl("Period", "Période", 90)),
		table("Scores", "", tblDef{cols: []gcol{{"Criterion", "Critère", 72, "L"}, {"Weight", "Poids", 22, "L"}, {"Score 1–5", "", 24, "L"}, {"Comment", "", 62, "L"}},
			rows: fixedRows(4, []string{"Quality of goods / services"}, []string{"Delivery on time"}, []string{"Price competitiveness"}, []string{"Responsiveness and support"}, []string{"Information security practices"}, []string{"Compliance and documentation"}, []string{"Overall"}),
			n:    7, fixedCols: 1}),
		chk("Decision", "", 2, false, "Keep / Maintenir", "Keep with improvement plan", "Suspend / Suspendre", "Remove / Déréférencer"),
		signs(signHekate("Evaluator / Évaluateur"), signClient("Approved by / Approuvé par")),
	},
}

// blockSchema is the schema of a pack form drawn from blocks, or nil when the form has none.
func blockSchema(code string) *schema {
	b, ok := frameBlocks[code]
	t, ok2 := packTitles[code]
	if !ok || !ok2 {
		return nil
	}
	return &schema{code: "HS-" + code, en: t[0], fr: t[1], blocks: b}
}

func heading(en, fr string) string {
	if fr == "" {
		return en
	}
	return en + " / " + fr
}

// matches reports whether an entered heading names the slot: English, French or "English / French".
func slotMatches(h, en, fr string) bool {
	if h == "" {
		return false
	}
	if h == normHeading(en) || h == normHeading(heading(en, fr)) {
		return true
	}
	return fr != "" && h == normHeading(fr)
}

// PackHeadings lists, for each pack form the platform fills in, the section headings the web editor pre-fills
// (English / French as the pack prints them) so that the text typed under a heading lands in that place of the
// printed form. Keys are form codes such as "HS-FRM-07". Values the document already supplies (date, client name,
// subject) and pack wording that prints by itself are left out.
func PackHeadings() map[string][]string {
	out := map[string][]string{}
	for c, bl := range frameBlocks {
		var hs []string
		for _, b := range bl {
			switch b.kind {
			case bCells:
				for _, c := range b.cells {
					if c.key == "" {
						hs = append(hs, heading(c.en, c.fr))
					}
				}
			case bChecks, bTable, bBoxK:
				hs = append(hs, heading(b.en, b.fr))
			}
		}
		out["HS-"+c] = hs
	}
	for c, sc := range schemas {
		var hs []string
		for _, s := range sc.secs {
			if s.fixedEN != "" {
				continue
			}
			hs = append(hs, heading(s.en, s.fr))
		}
		out["HS-"+c] = hs
	}
	return out
}

type blockRun struct {
	d     *doc
	secs  []Section
	norm  []string
	used  []bool
	extra []Section
}

// take returns the text of the first unused entered section whose heading names the slot.
func (r *blockRun) take(en, fr string) (string, bool) {
	for i, h := range r.norm {
		if !r.used[i] && slotMatches(h, en, fr) {
			r.used[i] = true
			return strings.TrimSpace(r.secs[i].Body), true
		}
	}
	return "", false
}

// renderBlocks draws a block-layout form from the document's values and entered sections, then prints entered
// sections that matched nothing under "Additional sections".
func (d *doc) renderBlocks(sc *schema, issued time.Time, title string, client Party, sections []Section, signedBy string) {
	r := &blockRun{d: d, secs: sections, used: make([]bool, len(sections))}
	for _, s := range sections {
		r.norm = append(r.norm, normHeading(s.Heading))
	}
	val := func(key string) string {
		switch key {
		case "date":
			return date(issued)
		case "client":
			return client.Name
		case "subject":
			return title
		case "rccm":
			return client.RCCM
		case "email":
			return client.Email
		}
		return ""
	}
	hasKey := map[string]bool{}
	for _, b := range sc.blocks {
		for _, c := range b.cells {
			if c.key != "" {
				hasKey[c.key] = true
			}
		}
	}
	// Values the pack form has no cell for are kept in one extra row: document number, client, subject.
	var pre []fcell
	if n := d.docNo(); n != "" {
		pre = append(pre, fc("Document no.", "N°", n, 60))
	}
	if !hasKey["client"] && client.Name != "" {
		pre = append(pre, fc("Client", "", client.Name, 60))
	}
	if !hasKey["subject"] && strings.TrimSpace(title) != "" {
		pre = append(pre, fc("Subject", "Objet", title, 60))
	}
	if len(pre) > 0 {
		for i := range pre {
			pre[i].w = width / float64(len(pre))
		}
		d.packFields(pre...)
	}
	no := 0
	signed := false
	for _, b := range sc.blocks {
		switch b.kind {
		case bBar:
			t := ""
			if b.no {
				no++
				t = itoa(no)
			}
			d.packBar(t, b.en, b.fr)
		case bCells:
			d.packFields(r.cells(b.cells, val)...)
		case bUseK:
			cs := b.cells
			for i := 0; i < len(cs); i += 2 {
				end := i + 2
				if end > len(cs) {
					end = len(cs)
				}
				d.packFields(r.cells(cs[i:end], val)...)
			}
		case bChecks:
			body, _ := r.take(b.en, b.fr)
			d.optionsFor(secDef{en: b.en, fr: b.fr, opts: b.opts, multi: b.multi, perRow: b.perRow}, body)
		case bTable:
			body, _ := r.take(b.en, b.fr)
			d.tableBlock(b, body)
		case bBoxK:
			body, _ := r.take(b.en, b.fr)
			d.packBox(b.en, b.fr, body, b.minH)
		case bNoteK:
			d.bilingualNoteOpt(b.en, b.fr)
		case bTextK:
			d.pdf.SetFont("Helvetica", "I", 8.5)
			d.pdf.MultiCell(0, 4.2, d.tr(b.en), "", "L", false)
			d.pdf.Ln(1)
		case bSignK:
			var cols []signCol
			for _, sg := range b.signs {
				c := signCol{title: sg.title}
				if sg.hekate && strings.TrimSpace(signedBy) != "" && !signed {
					c.name = strings.TrimSpace(signedBy)
					signed = true
				}
				cols = append(cols, c)
			}
			d.packSign(cols...)
		}
	}
	if !signed && strings.TrimSpace(signedBy) != "" {
		d.packFields(fc("Signed for HEKATE SYSTEMS SARL by", "", strings.TrimSpace(signedBy), width))
	}
	var extra []Section
	for i, s := range sections {
		if !r.used[i] {
			extra = append(extra, s)
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

// cells turns block cells into field cells, filled from the entered text under the cell's heading or else from the
// document value the cell stands for.
func (r *blockRun) cells(cs []hc, val func(string) string) []fcell {
	out := make([]fcell, len(cs))
	for i, c := range cs {
		v, ok := r.take(c.en, c.fr)
		if !ok || v == "" {
			v = val(c.key)
		}
		out[i] = fc(c.en, c.fr, v, c.w)
	}
	return out
}

// tableBlock draws a table with the pack's fixed rows. Entered lines "row | value | value" fill the row of that
// name (or add one); in a score table the first value is the score, marked in its column. Lines without "|" are
// printed in a box under the table so nothing typed is lost.
func (d *doc) tableBlock(b blk, body string) {
	t := b.tbl
	nc := len(t.cols)
	rows := make([][]string, len(t.rows))
	for i, r := range t.rows {
		rows[i] = append([]string(nil), r...)
		for len(rows[i]) < nc {
			rows[i] = append(rows[i], "")
		}
	}
	var rest []string
	for _, ln := range strings.Split(body, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if !strings.Contains(ln, "|") {
			rest = append(rest, ln)
			continue
		}
		parts := strings.Split(ln, "|")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		if t.fixedCols == 0 {
			r := make([]string, nc)
			copy(r, parts)
			rows = append(rows, r)
			continue
		}
		hit := -1
		for i, r := range rows {
			if r[t.keyCol] != "" && normHeading(r[t.keyCol]) == normHeading(parts[0]) {
				hit = i
				break
			}
		}
		if hit < 0 {
			r := make([]string, nc)
			r[t.keyCol] = parts[0]
			rows = append(rows, r)
			hit = len(rows) - 1
		}
		c := t.fixedCols
		for _, p := range parts[1:] {
			if t.scoreN > 0 && c == t.scoreAt {
				for k := 0; k < t.scoreN; k++ {
					if t.cols[t.scoreAt+k].en == p {
						rows[hit][t.scoreAt+k] = "X"
					}
				}
				c = t.scoreAt + t.scoreN
				continue
			}
			if c < nc {
				rows[hit][c] = p
				c++
			} else {
				rows[hit][nc-1] += " " + p
			}
		}
	}
	n := t.n
	if len(rows) > n {
		n = len(rows)
	}
	d.packGrid(t.cols, rows, n)
	if len(rest) > 0 {
		d.packBox("", "", strings.Join(rest, "\n"), 8)
	}
}
