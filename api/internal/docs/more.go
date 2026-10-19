package docs

import (
	"fmt"
	"strings"
	"time"
)

// RenderCreditNote draws a credit note against an issued invoice.
func RenderCreditNote(o Options, ref string, issued time.Time, invoiceRef, reason string, client Party, m Money) ([]byte, error) {
	d := newDoc(o, ref, "Credit note", issued)
	d.title("CREDIT NOTE / AVOIR", ref)
	d.kv("Issued", date(issued))
	d.kv("Against invoice", invoiceRef)
	d.kv("Reason", reason)
	d.pdf.Ln(2)
	d.party("Credited to", client)
	d.totals(m)
	d.pdf.Ln(6)
	d.note("This credit note reduces the amount owed on the invoice named above. Amounts are in CFA francs (XAF). Verify it by scanning the QR code or entering its reference on the HEKATE verification page.")
	return d.bytes()
}

// RenderReceipt draws the receipt for one payment. It shows only facts of that payment, never a running
// balance, so it stays correct for ever.
func RenderReceipt(o Options, ref string, paid time.Time, client Party, invoiceRef string, invoiceTotal, amount int64, method, reference string) ([]byte, error) {
	d := newDoc(o, ref, "Receipt", paid)
	d.title("RECEIPT / REÇU", ref)
	d.kv("Date received", date(paid))
	d.pdf.Ln(2)
	d.party("Received from", client)
	d.kv("For invoice", invoiceRef+"  (total "+XAF(invoiceTotal)+")")
	d.kv("Method", methodLabel(method))
	if strings.TrimSpace(reference) != "" {
		d.kv("Payment reference", reference)
	}
	d.pdf.Ln(4)
	d.pdf.SetFont("Helvetica", "B", 15)
	d.pdf.CellFormat(0, 10, d.tr("Amount received: "+XAF(amount)), "", 1, "L", false, 0, "")
	d.pdf.Ln(4)
	d.note("Thank you. This receipt confirms the payment above only; it is not an invoice. Verify it by scanning the QR code or entering its reference on the HEKATE verification page.")
	return d.bytes()
}

func methodLabel(m string) string {
	switch m {
	case "momo":
		return "MTN Mobile Money"
	case "orange_money":
		return "Orange Money"
	case "bank":
		return "Bank transfer"
	case "card":
		return "Card"
	case "cash":
		return "Cash"
	}
	return m
}

// RenderPurchaseOrder draws an approved purchase order addressed to a supplier.
func RenderPurchaseOrder(o Options, ref string, issued time.Time, title string, supplier Party, lines []Line, net int64) ([]byte, error) {
	d := newDoc(o, ref, "Purchase order", issued)
	d.title("PURCHASE ORDER / BON DE COMMANDE", ref)
	d.kv("Date", date(issued))
	d.kv("Subject", title)
	d.pdf.Ln(2)
	d.party("Supplier", supplier)
	d.table(lines)
	d.netTotal(net)
	d.pdf.Ln(6)
	d.note("Prices are net, in CFA francs (XAF); taxes are added as applicable. Please quote the purchase order reference on your delivery note and invoice. Verify this order by scanning the QR code or entering its reference on the HEKATE verification page.")
	return d.bytes()
}

// QtyLine is an item with a quantity and no price (a request for quotation).
type QtyLine struct {
	Description string
	Quantity    float64
}

// RenderRFQ draws a request for quotation, as sent to invited suppliers.
func RenderRFQ(o Options, ref string, issued time.Time, title string, closes *time.Time, lines []QtyLine) ([]byte, error) {
	d := newDoc(o, ref, "Request for quotation", issued)
	d.title("REQUEST FOR QUOTATION / DEMANDE DE PRIX", ref)
	d.kv("Date", date(issued))
	if closes != nil {
		d.kv("Reply by", closes.UTC().Format("02 Jan 2006 15:04 UTC"))
	}
	d.kv("Subject", title)
	d.pdf.Ln(3)
	p := d.pdf
	p.SetFont("Helvetica", "B", 9)
	p.SetFillColor(235, 238, 243)
	p.CellFormat(150, 7, "Item", "B", 0, "L", true, 0, "")
	p.CellFormat(30, 7, "Quantity", "B", 1, "R", true, 0, "")
	p.SetFont("Helvetica", "", 9.5)
	for i, l := range lines {
		wrapped := p.SplitLines([]byte(d.tr(l.Description)), 148)
		h := float64(len(wrapped))*4.8 + 2
		if p.GetY()+h > 297-22-10 {
			p.AddPage()
		}
		x, y := p.GetX(), p.GetY()
		p.MultiCell(150, 4.8, d.tr(fmt.Sprintf("%d. %s", i+1, l.Description)), "", "L", false)
		p.SetXY(x+150, y)
		p.CellFormat(30, 4.8, fmt.Sprintf("%g", l.Quantity), "", 0, "R", false, 0, "")
		p.SetXY(x, y+h)
	}
	p.Ln(4)
	d.note("Submit your unit prices through the HEKATE supplier portal before the closing time. Replies are sealed: other suppliers cannot see them. Verify this request by scanning the QR code or entering its reference on the HEKATE verification page.")
	return d.bytes()
}

// Asset is the equipment on an issue and return form.
type Asset struct {
	Item, Serial, Holder, IssuedBy string
	IssuedAt                       time.Time
}

// RenderAssetForm draws the equipment issue and return form with blank lines to sign by hand.
func RenderAssetForm(o Options, ref string, a Asset) ([]byte, error) {
	d := newDoc(o, ref, "Asset issue and return", a.IssuedAt)
	d.title("ASSET ISSUE AND RETURN FORM", ref)
	d.kv("Equipment", a.Item)
	d.kv("Serial number", a.Serial)
	d.kv("Issued to", a.Holder)
	d.kv("Issued by", a.IssuedBy)
	d.kv("Date issued", date(a.IssuedAt))
	d.pdf.Ln(4)
	d.note("The holder is responsible for this equipment until it is returned to HEKATE stores. Report loss or damage to your manager immediately.")
	d.pdf.Ln(6)
	d.signatureLine("Holder: received in good working order", 15)
	d.signatureLine("Issued by (stores)", 110)
	d.pdf.Ln(18)
	d.pdf.SetFont("Helvetica", "B", 10)
	d.pdf.CellFormat(0, 7, d.tr("Return (completed by hand)"), "", 1, "L", false, 0, "")
	d.pdf.SetFont("Helvetica", "", 9.5)
	d.pdf.CellFormat(0, 7, d.tr("Date returned: ____ / ____ / ________      Condition: _______________________________"), "", 1, "L", false, 0, "")
	d.pdf.Ln(10)
	d.signatureLine("Holder", 15)
	d.signatureLine("Received by (stores)", 110)
	return d.bytes()
}

func (d *doc) signatureLine(label string, x float64) {
	y := d.pdf.GetY()
	d.pdf.Line(x, y+12, x+75, y+12)
	d.pdf.SetXY(x, y+13)
	d.pdf.SetFont("Helvetica", "", 8.5)
	d.pdf.CellFormat(75, 4, d.tr(label), "", 0, "L", false, 0, "")
	d.pdf.SetXY(x, y)
}

func (d *doc) note(text string) {
	d.pdf.SetFont("Helvetica", "", 8.5)
	d.pdf.MultiCell(0, 4.2, d.tr(text), "", "L", false)
}

func (d *doc) netTotal(net int64) {
	p := d.pdf
	if p.GetY() > 297-22-20 {
		p.AddPage()
	}
	p.SetDrawColor(120, 120, 120)
	p.Line(110, p.GetY(), 195, p.GetY())
	p.SetFont("Helvetica", "B", 10.5)
	p.SetX(110)
	p.CellFormat(35, 6, "Total (net)", "", 0, "L", false, 0, "")
	p.CellFormat(50, 6, XAF(net), "", 1, "R", false, 0, "")
}
