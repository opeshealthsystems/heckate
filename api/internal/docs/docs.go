// Package docs renders HEKATE's printable documents (invoice, quotation, site-visit acceptance) as PDF.
//
// Output is deterministic: the same inputs produce the same bytes (dates come from the record, not
// the clock), so the SHA-256 recorded in the document register identifies exactly one file.
// A QR code on each page links to the public verification page.
package docs

import (
	"bytes"
	"fmt"
	"image/png"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/skip2/go-qrcode"
)

// Options configure rendering. Letterhead lines (RCCM, address, phone) come from configuration so
// nothing unconfirmed is ever printed.
type Options struct {
	PortalURL  string
	Letterhead []string
	Compress   bool
}

type Party struct{ Name, RCCM, Email string }

type Line struct {
	Description string
	Quantity    float64
	UnitPrice   int64
}

type Money struct{ Net, TVA, CAC, Total int64 }

// XAF formats whole francs with spaces between thousands. Plain spaces, so the core PDF fonts can print it.
func XAF(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ' ')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out) + " XAF"
	}
	return string(out) + " XAF"
}

// LineAmount is quantity x unit price in whole francs, rounded half up (hundredths of a unit in integers).
func LineAmount(l Line) int64 {
	q := int64(l.Quantity*100 + 0.5)
	return (q*l.UnitPrice + 50) / 100
}

// VerifyURL is the address the QR code points to.
func (o Options) VerifyURL(ref string) string {
	return strings.TrimRight(o.PortalURL, "/") + "/verify?ref=" + ref
}

type doc struct {
	pdf *fpdf.Fpdf
	tr  func(string) string
	o   Options
	ref string
}

func newDoc(o Options, ref, kind string, at time.Time) *doc {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(o.Compress)
	pdf.SetCatalogSort(true) // font and image tables are written in sorted order, not map order: deterministic bytes
	pdf.SetCreationDate(at.UTC())
	pdf.SetModificationDate(at.UTC())
	pdf.SetTitle(kind+" "+ref, false)
	pdf.SetAuthor("HEKATE SYSTEMS SARL", false)
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 22)
	pdf.AliasNbPages("{nb}")
	d := &doc{pdf: pdf, tr: pdf.UnicodeTranslatorFromDescriptor(""), o: o, ref: ref}

	var qr []byte
	if o.PortalURL != "" {
		qr, _ = qrcode.Encode(o.VerifyURL(ref), qrcode.Medium, 256)
	}
	pdf.SetHeaderFunc(func() {
		pdf.SetFont("Helvetica", "B", 15)
		pdf.SetXY(15, 12)
		pdf.CellFormat(120, 7, d.tr("HEKATE SYSTEMS SARL"), "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 8.5)
		pdf.SetTextColor(90, 90, 90)
		for _, l := range o.Letterhead {
			pdf.SetX(15)
			pdf.CellFormat(120, 4, d.tr(l), "", 1, "L", false, 0, "")
		}
		pdf.SetTextColor(0, 0, 0)
		if len(qr) > 0 {
			pdf.RegisterImageOptionsReader("qr", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(qr))
			pdf.ImageOptions("qr", 168, 10, 27, 27, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
			pdf.SetFont("Helvetica", "", 6.5)
			pdf.SetXY(160, 37.5)
			pdf.CellFormat(43, 3, d.tr("Scan to verify"), "", 0, "C", false, 0, "")
		}
		pdf.SetY(42)
	})
	pdf.SetFooterFunc(func() {
		pdf.SetY(-16)
		pdf.SetFont("Helvetica", "", 7.5)
		pdf.SetTextColor(90, 90, 90)
		line := "Reference " + ref
		if o.PortalURL != "" {
			line += "  |  Verify at " + o.VerifyURL(ref)
		}
		pdf.CellFormat(150, 4, d.tr(line), "", 0, "L", false, 0, "")
		pdf.CellFormat(30, 4, fmt.Sprintf("Page %d / {nb}", pdf.PageNo()), "", 0, "R", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})
	pdf.AddPage()
	return d
}

func (d *doc) title(kind, sub string) {
	d.pdf.SetFont("Helvetica", "B", 20)
	d.pdf.CellFormat(0, 10, d.tr(kind), "", 1, "L", false, 0, "")
	d.pdf.SetFont("Helvetica", "", 10)
	d.pdf.CellFormat(0, 5, d.tr(sub), "", 1, "L", false, 0, "")
	d.pdf.Ln(3)
}

func (d *doc) kv(k, v string) {
	d.pdf.SetFont("Helvetica", "B", 9.5)
	d.pdf.CellFormat(38, 5.5, d.tr(k), "", 0, "L", false, 0, "")
	d.pdf.SetFont("Helvetica", "", 9.5)
	d.pdf.MultiCell(0, 5.5, d.tr(v), "", "L", false)
}

func (d *doc) party(label string, p Party) {
	d.pdf.SetFont("Helvetica", "B", 9.5)
	d.pdf.CellFormat(0, 6, d.tr(label), "", 1, "L", false, 0, "")
	d.pdf.SetFont("Helvetica", "", 10)
	d.pdf.CellFormat(0, 5, d.tr(p.Name), "", 1, "L", false, 0, "")
	d.pdf.SetFont("Helvetica", "", 9)
	if p.RCCM != "" {
		d.pdf.CellFormat(0, 4.5, d.tr("RCCM "+p.RCCM), "", 1, "L", false, 0, "")
	}
	if p.Email != "" {
		d.pdf.CellFormat(0, 4.5, d.tr(p.Email), "", 1, "L", false, 0, "")
	}
	d.pdf.Ln(3)
}

// table draws the lines with wrapping descriptions that never split a row across pages badly.
func (d *doc) table(lines []Line) {
	p := d.pdf
	w := []float64{88, 20, 35, 37}
	head := func() {
		p.SetFont("Helvetica", "B", 9)
		p.SetFillColor(235, 238, 243)
		for i, h := range []string{"Description", "Qty", "Unit price", "Amount"} {
			al := "R"
			if i == 0 {
				al = "L"
			}
			p.CellFormat(w[i], 7, d.tr(h), "B", 0, al, true, 0, "")
		}
		p.Ln(-1)
	}
	head()
	p.SetFont("Helvetica", "", 9)
	for _, l := range lines {
		wrapped := p.SplitLines([]byte(d.tr(l.Description)), w[0]-2)
		h := float64(len(wrapped))*4.6 + 2
		if p.GetY()+h > 297-22-10 {
			p.AddPage()
			head()
			p.SetFont("Helvetica", "", 9)
		}
		x, y := p.GetX(), p.GetY()
		p.MultiCell(w[0], 4.6, d.tr(l.Description), "", "L", false)
		p.SetXY(x+w[0], y)
		qty := fmt.Sprintf("%g", l.Quantity)
		p.CellFormat(w[1], h-2, qty, "", 0, "R", false, 0, "")
		p.CellFormat(w[2], h-2, XAF(l.UnitPrice), "", 0, "R", false, 0, "")
		p.CellFormat(w[3], h-2, XAF(LineAmount(l)), "", 0, "R", false, 0, "")
		p.SetXY(x, y+h)
		p.SetDrawColor(220, 220, 220)
		p.Line(x, y+h-1, x+180, y+h-1)
	}
	p.Ln(3)
}

func (d *doc) totals(m Money) {
	p := d.pdf
	row := func(label string, v int64, bold bool) {
		if bold {
			p.SetFont("Helvetica", "B", 10.5)
		} else {
			p.SetFont("Helvetica", "", 9.5)
		}
		p.SetX(110)
		p.CellFormat(35, 6, d.tr(label), "", 0, "L", false, 0, "")
		p.CellFormat(50, 6, XAF(v), "", 1, "R", false, 0, "")
	}
	if p.GetY() > 297-22-30 {
		p.AddPage()
	}
	row("Net", m.Net, false)
	row("TVA", m.TVA, false)
	row("CAC", m.CAC, false)
	p.SetDrawColor(120, 120, 120)
	p.Line(110, p.GetY(), 195, p.GetY())
	row("Total", m.Total, true)
}

func (d *doc) bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := d.pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func date(t time.Time) string { return t.UTC().Format("02 Jan 2006") }

// RenderInvoice draws an issued invoice. It deliberately shows nothing that changes after issue
// (payments, balance), so the file, and its recorded hash, stay valid for the invoice's whole life.
func RenderInvoice(o Options, ref string, issued time.Time, due *time.Time, title string, bill Party, lines []Line, m Money) ([]byte, error) {
	d := newDoc(o, ref, "Invoice", issued)
	d.title("INVOICE / FACTURE", ref)
	d.kv("Issued", date(issued))
	if due != nil {
		d.kv("Due", date(*due))
	}
	d.kv("Subject", title)
	d.pdf.Ln(2)
	d.party("Billed to", bill)
	d.table(lines)
	d.totals(m)
	d.pdf.Ln(6)
	d.pdf.SetFont("Helvetica", "", 8.5)
	d.pdf.MultiCell(0, 4.2, d.tr("Amounts are in CFA francs (XAF). TVA and CAC as shown. Verify this invoice by scanning the QR code or entering its reference on the HEKATE verification page."), "", "L", false)
	return d.bytes()
}

// RenderQuote draws a quotation as sent to the client.
func RenderQuote(o Options, ref string, sent time.Time, validUntil *time.Time, title string, client Party, lines []Line, m Money, notes string) ([]byte, error) {
	d := newDoc(o, ref, "Quotation", sent)
	d.title("QUOTATION / DEVIS", ref)
	d.kv("Date", date(sent))
	if validUntil != nil {
		d.kv("Valid until", date(*validUntil))
	}
	d.kv("Subject", title)
	d.pdf.Ln(2)
	d.party("Prepared for", client)
	d.table(lines)
	d.totals(m)
	if strings.TrimSpace(notes) != "" {
		d.pdf.Ln(6)
		d.pdf.SetFont("Helvetica", "B", 9.5)
		d.pdf.CellFormat(0, 6, d.tr("Notes"), "", 1, "L", false, 0, "")
		d.pdf.SetFont("Helvetica", "", 9)
		d.pdf.MultiCell(0, 4.5, d.tr(notes), "", "L", false)
	}
	return d.bytes()
}

// Visit is the content of a completed site visit.
type Visit struct {
	Engineer          string
	Summary           string
	CheckIn, CheckOut time.Time
	InPos, OutPos     string
	Report            string
	SignedBy          string
	Signature         []byte // PNG; ignored if it does not decode
}

// RenderVisit draws the acceptance document for a completed site visit.
func RenderVisit(o Options, ref string, client Party, v Visit) ([]byte, error) {
	d := newDoc(o, ref, "Intervention report and acceptance", v.CheckOut)
	d.title("INTERVENTION REPORT AND ACCEPTANCE", ref)
	d.kv("Client", client.Name)
	d.kv("Engineer", v.Engineer)
	if v.Summary != "" {
		d.kv("Purpose", v.Summary)
	}
	d.kv("Arrived", v.CheckIn.UTC().Format("02 Jan 2006 15:04 UTC")+"  ("+v.InPos+")")
	d.kv("Completed", v.CheckOut.UTC().Format("02 Jan 2006 15:04 UTC")+"  ("+v.OutPos+")")
	d.pdf.Ln(3)
	d.pdf.SetFont("Helvetica", "B", 10)
	d.pdf.CellFormat(0, 6, d.tr("Work carried out"), "", 1, "L", false, 0, "")
	d.pdf.SetFont("Helvetica", "", 9.5)
	d.pdf.MultiCell(0, 5, d.tr(v.Report), "", "L", false)
	d.pdf.Ln(6)
	if v.SignedBy != "" {
		d.pdf.SetFont("Helvetica", "B", 10)
		d.pdf.CellFormat(0, 6, d.tr("Accepted by the client"), "", 1, "L", false, 0, "")
		if d.pdf.GetY() > 297-22-50 {
			d.pdf.AddPage()
		}
		if _, err := png.DecodeConfig(bytes.NewReader(v.Signature)); err == nil && len(v.Signature) > 0 {
			d.pdf.RegisterImageOptionsReader("sig", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(v.Signature))
			d.pdf.ImageOptions("sig", 15, d.pdf.GetY()+1, 60, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
			d.pdf.Ln(34)
		} else {
			d.pdf.SetFont("Helvetica", "I", 9)
			d.pdf.CellFormat(0, 6, d.tr("(signature image unavailable)"), "", 1, "L", false, 0, "")
		}
		d.pdf.SetFont("Helvetica", "", 9.5)
		d.pdf.CellFormat(0, 5, d.tr(v.SignedBy), "", 1, "L", false, 0, "")
	} else {
		d.pdf.SetFont("Helvetica", "I", 9)
		d.pdf.CellFormat(0, 6, d.tr("Completed without a client signature."), "", 1, "L", false, 0, "")
	}
	return d.bytes()
}
