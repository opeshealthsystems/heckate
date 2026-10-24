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

// PayLine is one deduction or contribution on a payslip. RateBP is in basis points; zero means "no rate".
type PayLine struct {
	Label  string
	Base   int64
	RateBP int64
	Amount int64
}

// Payslip is the content of one employee's payslip for one month.
type Payslip struct {
	Period, Employee string
	Gross            int64
	Deductions       []PayLine
	IRPP, CAC, Net   int64
	Employer         []PayLine
}

// RenderPayslip draws a payslip. It is confidential: the reference reveals nothing but the month.
func RenderPayslip(o Options, ref string, issued time.Time, p Payslip) ([]byte, error) {
	d := newDoc(o, ref, "Payslip", issued)
	d.title("PAYSLIP / BULLETIN DE PAIE", ref)
	d.kv("Period", p.Period)
	d.kv("Employee", p.Employee)
	d.pdf.Ln(2)
	row := func(label, base, rate, amount string, bold bool) {
		st := ""
		if bold {
			st = "B"
		}
		d.pdf.SetFont("Helvetica", st, 9.5)
		d.pdf.CellFormat(88, 6, d.tr(label), "", 0, "L", false, 0, "")
		d.pdf.CellFormat(32, 6, d.tr(base), "", 0, "R", false, 0, "")
		d.pdf.CellFormat(20, 6, d.tr(rate), "", 0, "R", false, 0, "")
		d.pdf.CellFormat(40, 6, d.tr(amount), "", 1, "R", false, 0, "")
	}
	rate := func(bp int64) string {
		if bp == 0 {
			return ""
		}
		return fmt.Sprintf("%.2f %%", float64(bp)/100)
	}
	base := func(b int64) string {
		if b == 0 {
			return ""
		}
		return XAF(b)
	}
	row("Description", "Base", "Rate", "Amount", true)
	d.pdf.Line(10, d.pdf.GetY(), 190, d.pdf.GetY())
	row("Gross salary", "", "", XAF(p.Gross), true)
	for _, l := range p.Deductions {
		row(l.Label, base(l.Base), rate(l.RateBP), "-"+XAF(l.Amount), false)
	}
	row("Income tax (IRPP)", "", "", "-"+XAF(p.IRPP), false)
	row("Council tax (CAC)", "", "", "-"+XAF(p.CAC), false)
	d.pdf.Line(10, d.pdf.GetY(), 190, d.pdf.GetY())
	d.pdf.Ln(1)
	d.pdf.SetFont("Helvetica", "B", 13)
	d.pdf.CellFormat(0, 9, d.tr("Net pay: "+XAF(p.Net)), "", 1, "R", false, 0, "")
	if len(p.Employer) > 0 {
		d.pdf.Ln(3)
		d.pdf.SetFont("Helvetica", "B", 9.5)
		d.pdf.CellFormat(0, 6, d.tr("Employer contributions (for information, not deducted)"), "", 1, "L", false, 0, "")
		for _, l := range p.Employer {
			row(l.Label, base(l.Base), rate(l.RateBP), XAF(l.Amount), false)
		}
	}
	d.pdf.Ln(5)
	d.note("Confidential. Calculated from the payroll rules approved for this period; amounts are in CFA francs (XAF). Verify it by entering its reference on the HEKATE verification page.")
	return d.bytes()
}

// Section is one titled block of a narrative document.
type Section struct {
	Heading string
	Body    string
}

// RenderNarrative draws a text document made of titled sections: proposal, order confirmation,
// statement of work, letter or certificate. kind is the heading printed at the top.
func RenderNarrative(o Options, ref, kind string, issued time.Time, title string, client Party, sections []Section, signedBy string) ([]byte, error) {
	return renderNarrative(o, ref, kind, issued, title, client, sections, signedBy, false)
}

// RenderNarrativeDraft shows an unissued document as it will print, marked as a draft that is not valid. It has
// no reference and no QR code, so it can never be mistaken for, or verified as, an issued document.
func RenderNarrativeDraft(o Options, kind string, at time.Time, title string, client Party, sections []Section, signedBy string) ([]byte, error) {
	return renderNarrative(o, "DRAFT", kind, at, title, client, sections, signedBy, true)
}
