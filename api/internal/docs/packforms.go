package docs

// Renderers for the pack's own forms, laid out like the approved originals (see pack.go): HS-QT, HS-INV, HS-PO,
// HS-FRM-38 and HS-FRM-27 (site visit). Narrative forms (proposal, SOW, order confirmation, change request,
// incident ...) are in packnarr.go.

import (
	"bytes"
	"image/png"
	"strconv"
	"strings"
	"time"
)

// Pack wording copied from the approved templates (HS-QT, HS-INV, HS-PO, HS-FRM-38).
const (
	quoteTermsEN = "Prices in FCFA. TVA 17.5 % and CAC (10 % of TVA) apply unless the client holds a valid exemption. Quotation valid 30 days. Payment: 50 % on order, 50 % on delivery, by bank transfer, within 30 days of invoice. Services are governed by the HEKATE Master Services Agreement (HS-DOC-27)."
	quoteTermsFR = "Prix en FCFA. TVA 17,5 % et CAC (10 % de la TVA) applicables sauf exonération. Offre valable 30 jours. Paiement : 50 % à la commande, 50 % à la livraison, par virement, à 30 jours date de facture. Prestations régies par le Contrat-cadre HEKATE (HS-DOC-27)."
	invTermsEN   = "Payment due within 30 days of the invoice date. Late payment bears interest at the BEAC rate plus 5 points and a fixed recovery fee, as provided by the Master Services Agreement. Please quote the invoice number on every transfer."
	invTermsFR   = "Paiement à 30 jours date de facture. Tout retard porte intérêt au taux BEAC majoré de 5 points, outre une indemnité forfaitaire de recouvrement. Merci de rappeler le numéro de facture sur chaque virement."
	poTermsEN    = "Invoices must quote this PO number and be sent to billing@hekatesystems.com. Goods and services are accepted only against this signed order. Gifts or payments to HEKATE staff are prohibited (HS-DOC-28)."
	poTermsFR    = "Les factures doivent rappeler ce numéro et être adressées à billing@hekatesystems.com. Seule cette commande signée engage HEKATE. Tout cadeau ou paiement au personnel HEKATE est interdit (HS-DOC-28)."
	assetDeclEN  = "I have received the equipment above in good condition. I will use it only for HEKATE work, keep it secure, report loss or theft within one hour to security@hekatesystems.com, and return it on request or when I leave."
	assetDeclFR  = "J’ai reçu le matériel ci-dessus en bon état. Je l’utilise uniquement pour HEKATE, le protège, signale toute perte ou vol dans l’heure et le restitue sur demande ou à mon départ."
)

var feeCols = []gcol{
	{"No.", "", 10, "L"}, {"Description", "Désignation", 70, "L"}, {"Qty", "Qté", 14, "R"}, {"Unit", "Unité", 16, "L"},
	{"Unit price", "PU HT", 28, "R"}, {"Disc.", "Rem.", 14, "R"}, {"Amount", "Montant HT", 28, "R"},
}

func feeRows(lines []Line) [][]string {
	rows := make([][]string, len(lines))
	for i, l := range lines {
		rows[i] = []string{itoa(i + 1), l.Description, qtyText(l.Quantity), "", XAF(l.UnitPrice), "", XAF(LineAmount(l))}
	}
	return rows
}

func itoa(n int) string { return strconv.Itoa(n) }

// clientCells draws the pack's client block (A CLIENT / Client). Only the name, RCCM and e-mail are held; the
// rest stay blank for filling by hand.
func (d *doc) clientBlock(no string, c Party) {
	d.packBar(no, "Client", "Client")
	d.packFields(fc("Company", "Raison sociale", c.Name, 100), fc("Contact person", "Interlocuteur", "", 80))
	d.packFields(fc("Address", "Adresse", "", 100), fc("E-mail", "", c.Email, 80))
	d.packFields(fc("RCCM no.", "N° RCCM", c.RCCM, 60), fc("NIU", "NIU", "", 50), fc("Telephone", "Téléphone", "", 70))
}

func (d *doc) docNo() string {
	if d.draft {
		return ""
	}
	return d.ref
}

func dateOrBlank(t *time.Time) string {
	if t == nil {
		return ""
	}
	return date(*t)
}

// RenderQuote draws a quotation as the pack's HS-QT form: document block, A client, B quoted services, totals,
// C terms and client acceptance.
func RenderQuote(o Options, ref string, sent time.Time, validUntil *time.Time, title string, client Party, lines []Line, m Money, notes string) ([]byte, error) {
	d := newDoc(o, ref, "Quotation", sent)
	d.packTitle("QUOTATION", "Devis / Facture pro forma", "HS-QT", sent)
	d.packFields(fc("Document no.", "N°", d.docNo(), 55), fc("Date", "", date(sent), 35), fc("Client ref.", "Réf. client", "", 45), fc("Valid until", "Échéance", dateOrBlank(validUntil), 45))
	if title != "" {
		d.packFields(fc("Subject", "Objet", title, width))
	}
	d.clientBlock("A", client)
	d.packBar("B", "Quoted services", "Prestations proposées")
	d.packGrid(feeCols, feeRows(lines), 7)
	d.packTotals(m, true)
	d.packBar("C", "Terms", "Conditions")
	d.bilingualNote(quoteTermsEN, quoteTermsFR)
	if strings.TrimSpace(notes) != "" {
		d.packBox("Notes", "", notes, 10)
	}
	d.packBar("", "Client acceptance", "Acceptation du client")
	d.packChecks("", "", []string{"Accepted as quoted / Accepté", "Accepted with changes / Accepté avec réserves", "Declined / Refusé"}, nil, 3)
	return d.bytes()
}

// RenderInvoice draws an issued invoice as the pack's HS-INV form. It deliberately shows nothing that changes after
// issue (payments, balance), so the file, and its recorded hash, stay valid for the invoice's whole life.
func RenderInvoice(o Options, ref string, issued time.Time, due *time.Time, title string, bill Party, lines []Line, m Money) ([]byte, error) {
	d := newDoc(o, ref, "Invoice", issued)
	d.packTitle("INVOICE", "Facture", "HS-INV", issued)
	d.packFields(fc("Document no.", "N°", d.docNo(), 55), fc("Date", "", date(issued), 35), fc("Client ref.", "Réf. client", "", 45), fc("Due date", "Échéance", dateOrBlank(due), 45))
	if title != "" {
		d.packFields(fc("Subject", "Objet", title, width))
	}
	d.clientBlock("A", bill)
	d.packFields(fc("Quotation ref.", "Réf. devis", "", 60), fc("Purchase order", "Bon de commande", "", 60), fc("Delivery note", "Bon de livraison", "", 60))
	d.packBar("B", "Invoiced services", "Prestations facturées")
	d.packGrid(feeCols, feeRows(lines), 5)
	d.packTotals(m, true)
	d.packBar("C", "Payment", "Règlement")
	d.packFields(fc("Bank", "Banque", "", 90), fc("Account holder", "Titulaire", "", 90))
	d.packFields(fc("Account no. (RIB)", "N° de compte (RIB)", "", 70), fc("Payment ref.", "Réf. paiement", "", 55), fc("SWIFT", "BIC", "", 55))
	d.bilingualNote(invTermsEN, invTermsFR)
	d.packSign(signCol{title: "Issued by / Émise par - HEKATE SYSTEMS SARL"})
	d.pdf.Ln(1)
	d.note("Amounts are in CFA francs (XAF). Verify this invoice by scanning the QR code or entering its reference on the HEKATE verification page.")
	return d.bytes()
}

// RenderPurchaseOrder draws an approved purchase order as the pack's HS-PO form. Prices on a PO are net, so only
// the net total is filled in; tax lines stay blank.
func RenderPurchaseOrder(o Options, ref string, issued time.Time, title string, supplier Party, lines []Line, net int64) ([]byte, error) {
	d := newDoc(o, ref, "Purchase order", issued)
	d.packTitle("PURCHASE ORDER", "Bon de commande", "HS-PO", issued)
	d.packFields(fc("PO no.", "N°", d.docNo(), 55), fc("Date", "", date(issued), 40), fc("Requested by", "Demandeur", "", 50), fc("Budget code", "", "", 35))
	if title != "" {
		d.packFields(fc("Subject", "Objet", title, width))
	}
	d.packBar("A", "Supplier", "Fournisseur")
	d.packFields(fc("Supplier", "Fournisseur", supplier.Name, 120), fc("Supplier ID", "", "", 60))
	d.packFields(fc("Address", "Adresse", "", 120), fc("Contact", "", supplier.Email, 60))
	d.packBar("B", "Items", "Articles")
	d.packGrid(feeCols, feeRows(lines), 5)
	d.packTotals(Money{Net: net}, false)
	d.packFields(fc("Delivery address", "Livraison", "", 90), fc("Delivery date", "Date de livraison", "", 90))
	d.packFields(fc("Payment terms", "Conditions de paiement", "", 90), fc("Quote ref.", "Réf. offre fournisseur", "", 90))
	d.bilingualNote(poTermsEN, poTermsFR)
	d.packSign(signCol{title: "Requested by / Demandé par"}, signCol{title: "Approved by / Approuvé par (CEO)"}, signCol{title: "Supplier acceptance / Accord fournisseur"})
	d.note("Prices are net, in CFA francs (XAF); taxes are added as applicable. Please quote the purchase order reference on your delivery note and invoice. Verify this order by scanning the QR code or entering its reference on the HEKATE verification page.")
	return d.bytes()
}

// RenderAssetForm draws the equipment issue and return form as the pack's HS-FRM-38: employee block, asset table,
// security set-up check boxes (left unticked: nothing is recorded about them) and three signature blocks.
func RenderAssetForm(o Options, ref string, a Asset) ([]byte, error) {
	d := newDoc(o, ref, "Asset issue and return", a.IssuedAt)
	d.packTitle("ASSET ISSUE AND RETURN FORM", "Fiche de remise et de restitution de matériel", "HS-FRM-38", a.IssuedAt)
	d.packFields(fc("Employee", "Employé", a.Holder, 100), fc("Staff no.", "Matricule", "", 40), fc("Department", "", "", 40))
	d.pdf.Ln(2)
	d.packGrid([]gcol{
		{"Asset tag", "N° inventaire", 22, "L"}, {"Item", "Matériel", 40, "L"}, {"Serial No.", "N° série", 46, "L"},
		{"Condition", "État", 22, "L"}, {"Issued", "Remis le", 26, "L"}, {"Returned", "Restitué le", 24, "L"},
	}, [][]string{{"", a.Item, a.Serial, "", date(a.IssuedAt), ""}}, 3)
	d.packBar("", "Security set-up", "Sécurisation")
	d.packChecks("", "", []string{"Full-disk encryption", "EDR installed", "MFA enrolled", "Asset registered in inventory", "Acceptable-use policy signed"}, nil, 3)
	d.pdf.Ln(1)
	d.bilingualNote(assetDeclEN, assetDeclFR)
	d.packSign(
		signCol{title: "Employee / Employé", name: a.Holder},
		signCol{title: "IT / Informatique - issue", name: a.IssuedBy, date: date(a.IssuedAt)},
		signCol{title: "IT / Informatique - return"},
	)
	d.note("Report loss or damage to your manager immediately. Equipment is returned to HEKATE stores.")
	return d.bytes()
}

const visitAcceptEN = "Signature of this certificate without reservation constitutes final acceptance of the deliverables and triggers the corresponding invoice. Reservations must be lifted by the agreed deadlines; their lifting is recorded on a new certificate."
const visitAcceptFR = "La signature sans réserve vaut réception définitive et déclenche la facturation correspondante. Les réserves sont levées dans les délais convenus et constatées par un nouveau procès-verbal."

// RenderVisit draws the acceptance document for a completed site visit in the layout of the pack's HS-FRM-27
// (delivery note and acceptance certificate). A visit records work done, not a list of deliverables, so section 1
// is headed "Work carried out"; the decision boxes stay unticked because the platform records only a signature.
func RenderVisit(o Options, ref string, client Party, v Visit) ([]byte, error) {
	d := newDoc(o, ref, "Intervention report and acceptance", v.CheckOut)
	d.packTitle("DELIVERY NOTE AND ACCEPTANCE CERTIFICATE", "Bon de livraison et procès-verbal de réception", "HS-FRM-27", v.CheckOut)
	d.packFields(fc("PV no.", "N° PV", d.docNo(), 60), fc("Project", "Projet", v.Summary, 80), fc("Date", "", date(v.CheckOut), 40))
	d.packFields(fc("Client", "", client.Name, 100), fc("Contract / PO ref.", "Réf. contrat", "", 80))
	d.packBar("1", "Work carried out", "Travaux réalisés")
	d.packFields(fc("Engineer", "Ingénieur", v.Engineer, width))
	d.packFields(fc("Arrived", "Arrivée", v.CheckIn.UTC().Format("02 Jan 2006 15:04 UTC")+"  ("+v.InPos+")", width))
	d.packFields(fc("Completed", "Fin", v.CheckOut.UTC().Format("02 Jan 2006 15:04 UTC")+"  ("+v.OutPos+")", width))
	d.packBox("Report", "Compte rendu", v.Report, 20)
	d.packBar("2", "Acceptance", "Réception")
	d.packChecks("Decision", "", []string{"Accepted without reservation / Réception sans réserve", "Accepted with reservations listed below / Réception avec réserves", "Refused - reasons below / Refus motivé"}, nil, 1)
	d.packGrid([]gcol{{"Reservation", "Réserve", 80, "L"}, {"Agreed correction", "Correction", 70, "L"}, {"Deadline", "Délai", 30, "L"}}, nil, 3)
	d.bilingualNote(visitAcceptEN, visitAcceptFR)
	cl := signCol{title: "For the client / Pour le client"}
	if v.SignedBy != "" {
		cl.name, cl.date = v.SignedBy, date(v.CheckOut)
		if _, err := png.DecodeConfig(bytes.NewReader(v.Signature)); err == nil && len(v.Signature) > 0 {
			cl.sig = v.Signature
		}
	}
	d.packSign(cl, signCol{title: "For HEKATE SYSTEMS SARL", name: v.Engineer})
	d.pdf.SetFont("Helvetica", "I", 9)
	switch {
	case v.SignedBy == "":
		d.pdf.CellFormat(0, 6, d.tr("Completed without a client signature."), "", 1, "L", false, 0, "")
	case cl.sig == nil:
		d.pdf.CellFormat(0, 6, d.tr("(signature image unavailable)"), "", 1, "L", false, 0, "")
	}
	return d.bytes()
}
