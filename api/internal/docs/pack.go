package docs

// Drawing primitives that reproduce the look of the approved pack's forms (HS-QT, HS-SOW, HS-FRM-38 ...):
// a navy title block with the English and French title and the "FORM / FORMULAIRE code - VERSION - date" line,
// field cells with a bilingual label over a ruled value, lettered or numbered section bars, tables with a
// navy header and French sub-labels, check boxes, and signature blocks ("Name / Nom", "Date",
// "Signature & stamp / Signature et cachet"). Everything is drawn with fixed measurements and the clock is never
// read, so output stays byte-stable. Blank stays blank: a cell with no value prints its label and an empty rule.

import (
	"bytes"
	"fmt"
	"image/png"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

var labelBlue = [3]int{40, 90, 150}

const (
	pageBottom = 297 - 22 // lowest y a block may reach before the footer
	left       = 15.0
	width      = 180.0
)

func (d *doc) version() string {
	if d.o.Version != "" {
		return d.o.Version
	}
	return "1.0"
}

// ensure starts a new page when h millimetres do not fit.
func (d *doc) ensure(h float64) {
	if d.pdf.GetY()+h > pageBottom {
		d.pdf.AddPage()
	}
}

// packTitle draws the navy title block: English title, French title, form code, version and date.
func (d *doc) packTitle(en, fr, code string, at time.Time) {
	p := d.pdf
	enL := p.SplitLines([]byte(d.tr(en)), width-8)
	frL := p.SplitLines([]byte(d.tr(fr)), width-8)
	h := 4.0 + float64(len(enL))*7.5 + float64(len(frL))*4.6 + 7
	d.ensure(h + 4)
	x, y := left, p.GetY()
	p.SetFillColor(navy[0], navy[1], navy[2])
	p.Rect(x, y, width, h, "F")
	p.SetTextColor(255, 255, 255)
	p.SetFont("Helvetica", "B", 15)
	p.SetXY(x+4, y+3)
	p.MultiCell(width-8, 7.5, d.tr(en), "", "L", false)
	p.SetFont("Helvetica", "B", 8)
	p.SetTextColor(gold[0], gold[1], gold[2])
	p.SetX(x + 4)
	p.MultiCell(width-8, 4.6, d.tr(strings.ToUpper(fr)), "", "L", false)
	p.SetFont("Helvetica", "", 7)
	p.SetTextColor(255, 255, 255)
	p.SetX(x + 4)
	p.CellFormat(width-8, 5, d.tr("FORM / FORMULAIRE   "+code+"   -   VERSION "+d.version()+"   -   "+strings.ToUpper(date(at))), "", 1, "L", false, 0, "")
	p.SetTextColor(0, 0, 0)
	p.SetY(y + h + 4)
}

// fcell is one field: a bilingual label above a ruled value.
type fcell struct {
	label, fr string
	value     string
	w         float64 // millimetres; cells of a row should add up to 180
}

func fc(label, fr, value string, w float64) fcell { return fcell{label, fr, value, w} }

func (d *doc) labelText(label, fr string) string {
	if fr == "" {
		return label
	}
	return label + " / " + fr
}

// packFields draws a row of field cells side by side.
func (d *doc) packFields(cells ...fcell) {
	p := d.pdf
	lines := 1
	for _, c := range cells {
		if n := len(p.SplitLines([]byte(d.tr(c.value)), c.w-4)); n > lines {
			lines = n
		}
	}
	h := 3.6 + float64(lines)*4.6 + 2
	d.ensure(h)
	x, y := left, p.GetY()
	for _, c := range cells {
		p.SetXY(x, y)
		p.SetFont("Helvetica", "B", 6.5)
		p.SetTextColor(labelBlue[0], labelBlue[1], labelBlue[2])
		p.CellFormat(c.w-2, 3.6, d.tr(strings.ToUpper(c.label)), "", 0, "L", false, 0, "")
		if c.fr != "" {
			p.SetXY(x, y)
			p.SetFont("Helvetica", "I", 6.5)
			p.SetTextColor(110, 110, 110)
			off := p.GetStringWidth(d.tr(strings.ToUpper(c.label))) + 1.2
			p.SetX(x + off)
			p.CellFormat(c.w-2-off, 3.6, d.tr("/ "+c.fr), "", 0, "L", false, 0, "")
		}
		p.SetTextColor(0, 0, 0)
		p.SetXY(x, y+3.6)
		p.SetFont("Helvetica", "", 9.5)
		p.MultiCell(c.w-2, 4.6, d.tr(c.value), "", "L", false)
		p.SetDrawColor(ruleGrey[0], ruleGrey[1], ruleGrey[2])
		p.Line(x, y+h-1, x+c.w-2, y+h-1)
		x += c.w
	}
	p.SetDrawColor(0, 0, 0)
	p.SetY(y + h + 1)
}

// packBar draws a section bar: "A  CLIENT / Client" or "3  DELIVERABLES / Livrables".
func (d *doc) packBar(no, en, fr string) {
	p := d.pdf
	d.ensure(32)
	p.Ln(2)
	y := p.GetY()
	p.SetFillColor(bandFill[0], bandFill[1], bandFill[2])
	p.Rect(left, y, width, 7, "F")
	p.SetDrawColor(navy[0], navy[1], navy[2])
	p.SetLineWidth(0.4)
	p.Line(left, y+7, left+width, y+7)
	p.SetLineWidth(0.2)
	p.SetXY(left+2, y+1)
	p.SetFont("Helvetica", "B", 9.5)
	p.SetTextColor(navy[0], navy[1], navy[2])
	t := strings.ToUpper(en)
	if no != "" {
		t = no + "  " + t
	}
	p.CellFormat(p.GetStringWidth(d.tr(t))+3, 5, d.tr(t), "", 0, "L", false, 0, "")
	if fr != "" {
		p.SetFont("Helvetica", "I", 9)
		p.SetTextColor(110, 110, 110)
		p.CellFormat(0, 5, d.tr("/ "+fr), "", 0, "L", false, 0, "")
	}
	p.SetTextColor(0, 0, 0)
	p.SetDrawColor(0, 0, 0)
	p.SetY(y + 9)
}

// packBox is a labelled, bordered writing area. Long text carries on over the page break, re-boxed on each page.
func (d *doc) packBox(label, fr, body string, minH float64) {
	p := d.pdf
	if label != "" {
		d.ensure(14)
		p.SetFont("Helvetica", "B", 6.5)
		p.SetTextColor(labelBlue[0], labelBlue[1], labelBlue[2])
		t := strings.ToUpper(label)
		p.CellFormat(p.GetStringWidth(d.tr(t))+2, 3.6, d.tr(t), "", 0, "L", false, 0, "")
		if fr != "" {
			p.SetFont("Helvetica", "I", 6.5)
			p.SetTextColor(110, 110, 110)
			p.CellFormat(0, 3.6, d.tr("/ "+fr), "", 0, "L", false, 0, "")
		}
		p.Ln(3.8)
		p.SetTextColor(0, 0, 0)
	}
	p.SetFont("Helvetica", "", 9.5)
	var lines []string
	if strings.TrimSpace(body) != "" {
		for _, b := range p.SplitLines([]byte(d.tr(body)), width-4) {
			lines = append(lines, string(b))
		}
	}
	p.SetDrawColor(150, 160, 175)
	if len(lines) == 0 {
		d.ensure(minH)
		p.Rect(left, p.GetY(), width, minH, "D")
		p.SetY(p.GetY() + minH + 2)
		p.SetDrawColor(0, 0, 0)
		return
	}
	const lh = 4.6
	i := 0
	for i < len(lines) {
		avail := pageBottom - p.GetY() - 2
		if avail < lh*2+2 {
			p.AddPage()
			avail = pageBottom - p.GetY() - 2
		}
		n := int((avail - 2) / lh)
		if n > len(lines)-i {
			n = len(lines) - i
		}
		h := float64(n)*lh + 2
		if i+n >= len(lines) && h < minH {
			h = minH
		}
		y := p.GetY()
		p.Rect(left, y, width, h, "D")
		for k := 0; k < n; k++ {
			p.SetXY(left+2, y+1+float64(k)*lh)
			p.CellFormat(width-4, lh, lines[i+k], "", 0, "L", false, 0, "")
		}
		p.SetY(y + h + 2)
		i += n
	}
	p.SetDrawColor(0, 0, 0)
}

// packChecks draws check boxes in a grid; options listed in ticked are marked.
func (d *doc) packChecks(label, fr string, options []string, ticked map[string]bool, perRow int) {
	p := d.pdf
	if perRow < 1 {
		perRow = 3
	}
	rows := (len(options) + perRow - 1) / perRow
	d.ensure(4.5 + float64(rows)*5.5)
	if label != "" {
		p.SetFont("Helvetica", "B", 6.5)
		p.SetTextColor(labelBlue[0], labelBlue[1], labelBlue[2])
		t := strings.ToUpper(label)
		p.CellFormat(p.GetStringWidth(d.tr(t))+2, 3.6, d.tr(t), "", 0, "L", false, 0, "")
		if fr != "" {
			p.SetFont("Helvetica", "I", 6.5)
			p.SetTextColor(110, 110, 110)
			p.CellFormat(0, 3.6, d.tr("/ "+fr), "", 0, "L", false, 0, "")
		}
		p.Ln(4.2)
		p.SetTextColor(0, 0, 0)
	}
	cw := width / float64(perRow)
	y0 := p.GetY()
	for i, o := range options {
		x := left + float64(i%perRow)*cw
		y := y0 + float64(i/perRow)*5.5
		p.SetDrawColor(navy[0], navy[1], navy[2])
		p.Rect(x+1, y+0.6, 3.4, 3.4, "D")
		if ticked[o] {
			p.SetLineWidth(0.5)
			p.Line(x+1.6, y+2.4, x+2.6, y+3.6)
			p.Line(x+2.6, y+3.6, x+4.1, y+1.0)
			p.SetLineWidth(0.2)
		}
		p.SetXY(x+6, y)
		p.SetFont("Helvetica", "", 8.5)
		p.CellFormat(cw-6, 4.6, d.tr(o), "", 0, "L", false, 0, "")
	}
	p.SetDrawColor(0, 0, 0)
	p.SetY(y0 + float64(rows)*5.5 + 1)
}

// gcol is a table column: bilingual header, width and alignment ("L" or "R").
type gcol struct {
	en, fr string
	w      float64
	al     string
}

// packGrid draws a table with a navy header (English over French) and at least minRows ruled rows. Rows are never
// cut across pages; the header repeats on a new page.
func (d *doc) packGrid(cols []gcol, rows [][]string, minRows int) {
	p := d.pdf
	head := func() {
		d.ensure(20)
		y := p.GetY()
		p.SetFillColor(navy[0], navy[1], navy[2])
		p.Rect(left, y, width, 9, "F")
		x := left
		for _, c := range cols {
			p.SetXY(x+1, y+0.8)
			p.SetFont("Helvetica", "B", 8)
			p.SetTextColor(255, 255, 255)
			p.CellFormat(c.w-2, 3.8, d.tr(c.en), "", 0, c.al, false, 0, "")
			p.SetXY(x+1, y+4.6)
			p.SetFont("Helvetica", "I", 6.5)
			p.SetTextColor(200, 210, 225)
			p.CellFormat(c.w-2, 3.4, d.tr(c.fr), "", 0, c.al, false, 0, "")
			x += c.w
		}
		p.SetTextColor(0, 0, 0)
		p.SetY(y + 9)
	}
	head()
	n := len(rows)
	if n < minRows {
		n = minRows
	}
	for i := 0; i < n; i++ {
		var r []string
		if i < len(rows) {
			r = rows[i]
		}
		lines := 1
		p.SetFont("Helvetica", "", 9)
		for j, c := range cols {
			if j < len(r) {
				if k := len(p.SplitLines([]byte(d.tr(r[j])), c.w-2)); k > lines {
					lines = k
				}
			}
		}
		h := float64(lines)*4.6 + 3.4
		if p.GetY()+h > pageBottom-4 {
			p.AddPage()
			head()
		}
		y := p.GetY()
		x := left
		for j, c := range cols {
			if j < len(r) && r[j] != "" {
				p.SetXY(x+1, y+1.4)
				p.MultiCell(c.w-2, 4.6, d.tr(r[j]), "", c.al, false)
			}
			x += c.w
		}
		p.SetDrawColor(ruleGrey[0], ruleGrey[1], ruleGrey[2])
		p.Line(left, y+h, left+width, y+h)
		p.SetDrawColor(0, 0, 0)
		p.SetY(y + h)
	}
	p.Ln(1)
}

// vatLabel prints the pack's rate in the label only when the amount really is that rate, so a label never
// contradicts the figure beside it.
func vatLabel(m Money) (tva, cac string) {
	tva, cac = "TVA", "CAC"
	if m.Net > 0 {
		if diff := m.TVA - (m.Net*175+500)/1000; diff >= -1 && diff <= 1 {
			tva = "TVA 17.5 %"
		}
	}
	if m.TVA > 0 {
		if diff := m.CAC - (m.TVA+5)/10; diff >= -1 && diff <= 1 {
			cac = "CAC (10 % of TVA)"
		}
	}
	return
}

// packTotals draws the totals block and the amount in words. blank leaves the figures empty (never invented).
func (d *doc) packTotals(m Money, showTax bool) {
	p := d.pdf
	d.ensure(40)
	tva, cac := vatLabel(m)
	row := func(label string, v int64, show bool) {
		p.SetX(left + 90)
		p.SetFont("Helvetica", "", 9)
		p.CellFormat(50, 6, d.tr(label), "", 0, "R", false, 0, "")
		val := ""
		if show {
			val = XAF(v)
		}
		p.CellFormat(40, 6, d.tr(val), "", 1, "R", false, 0, "")
		p.SetDrawColor(ruleGrey[0], ruleGrey[1], ruleGrey[2])
		p.Line(left, p.GetY(), left+width, p.GetY())
	}
	row("Total excl. tax / Total HT", m.Net, showTax || m.Net != 0)
	row(tva, m.TVA, showTax)
	row(cac, m.CAC, showTax)
	y := p.GetY()
	p.SetFillColor(navy[0], navy[1], navy[2])
	p.Rect(left, y, width, 7, "F")
	p.SetTextColor(255, 255, 255)
	p.SetFont("Helvetica", "B", 9.5)
	p.SetXY(left+90, y+0.5)
	p.CellFormat(50, 6, d.tr("TOTAL incl. tax / TOTAL TTC (FCFA)"), "", 0, "R", false, 0, "")
	tot := ""
	if showTax {
		tot = XAF(m.Total)
	}
	p.CellFormat(40, 6, d.tr(tot), "", 1, "R", false, 0, "")
	p.SetTextColor(0, 0, 0)
	p.SetY(y + 9)
	words := ""
	if showTax && m.Total > 0 {
		words = FrenchAmount(m.Total)
	}
	d.packFields(fc("Amount in words", "Arrêté à la somme de", words, width))
}

// signCol is one signature block.
type signCol struct {
	title, name, date string
	sig               []byte // optional PNG drawn in the signature area; ignored if it does not decode
}

// packSign draws signature blocks side by side: title, "Name / Nom", "Date", "Signature & stamp / Signature et cachet".
func (d *doc) packSign(cols ...signCol) {
	p := d.pdf
	const h = 34.0
	d.ensure(h + 2)
	cw := width / float64(len(cols))
	y := p.GetY()
	for i, c := range cols {
		x := left + float64(i)*cw
		p.SetDrawColor(150, 160, 175)
		p.Rect(x+0.5, y, cw-1, h, "D")
		p.SetXY(x+2, y+1)
		p.SetFont("Helvetica", "B", 7.5)
		p.SetTextColor(navy[0], navy[1], navy[2])
		p.CellFormat(cw-4, 4, d.tr(strings.ToUpper(c.title)), "", 1, "L", false, 0, "")
		p.SetTextColor(0, 0, 0)
		p.SetX(x + 2)
		p.SetFont("Helvetica", "", 8)
		p.CellFormat(cw-4, 5, d.tr("Name / Nom: "+c.name), "", 1, "L", false, 0, "")
		p.SetX(x + 2)
		p.CellFormat(cw-4, 5, d.tr("Date: "+c.date), "", 1, "L", false, 0, "")
		p.SetX(x + 2)
		p.SetFont("Helvetica", "I", 7)
		p.SetTextColor(110, 110, 110)
		p.CellFormat(cw-4, 4, d.tr("Signature & stamp / Signature et cachet:"), "", 1, "L", false, 0, "")
		p.SetTextColor(0, 0, 0)
		if len(c.sig) > 0 {
			if cfg, err := png.DecodeConfig(bytes.NewReader(c.sig)); err == nil && cfg.Width > 0 && cfg.Height > 0 {
				iw, ih := cw-8, 13.0
				if r := float64(cfg.Width) / float64(cfg.Height); iw/ih > r {
					iw = ih * r
				} else {
					ih = iw / r
				}
				id := fmt.Sprintf("sig%d", i)
				p.RegisterImageOptionsReader(id, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(c.sig))
				p.ImageOptions(id, x+3, y+h-ih-1.5, iw, ih, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
			}
		}
	}
	p.SetDrawColor(0, 0, 0)
	p.SetY(y + h + 3)
}

// bilingualNote prints the pack's English and French wording of a clause in a shaded panel.
func (d *doc) bilingualNote(en, fr string) {
	p := d.pdf
	p.SetFont("Helvetica", "", 8.5)
	a := p.SplitLines([]byte(d.tr(en)), width-4)
	p.SetFont("Helvetica", "I", 8.5)
	b := p.SplitLines([]byte(d.tr(fr)), width-4)
	h := float64(len(a)+len(b))*4.1 + 4
	d.ensure(h + 2)
	y := p.GetY()
	p.SetFillColor(bandFill[0], bandFill[1], bandFill[2])
	p.Rect(left, y, width, h, "F")
	p.SetXY(left+2, y+2)
	p.SetFont("Helvetica", "", 8.5)
	p.MultiCell(width-4, 4.1, d.tr(en), "", "L", false)
	p.SetX(left + 2)
	p.SetFont("Helvetica", "I", 8.5)
	p.SetTextColor(90, 90, 90)
	p.MultiCell(width-4, 4.1, d.tr(fr), "", "L", false)
	p.SetTextColor(0, 0, 0)
	p.SetY(y + h + 2)
}

// FrenchAmount writes a whole number of francs in French words, for the "Arrêté à la somme de" line.
func FrenchAmount(n int64) string {
	if n == 0 {
		return "Zéro franc CFA"
	}
	s := strings.ToUpper(frWords(n)[:1]) + frWords(n)[1:]
	if n >= 1_000_000 && n%1_000_000 == 0 {
		return s + " de francs CFA"
	}
	if n == 1 {
		return s + " franc CFA"
	}
	return s + " francs CFA"
}

var frUnits = []string{"zéro", "un", "deux", "trois", "quatre", "cinq", "six", "sept", "huit", "neuf", "dix", "onze", "douze", "treize", "quatorze", "quinze", "seize", "dix-sept", "dix-huit", "dix-neuf"}
var frTens = []string{"", "", "vingt", "trente", "quarante", "cinquante", "soixante", "soixante", "quatre-vingt", "quatre-vingt"}

// frBelow100 handles 1..99; last says whether the word closes a number (so "quatre-vingts" takes its s).
func frBelow100(n int64, last bool) string {
	if n < 20 {
		return frUnits[n]
	}
	t, u := n/10, n%10
	switch {
	case t <= 6:
		switch u {
		case 0:
			return frTens[t]
		case 1:
			return frTens[t] + " et un"
		}
		return frTens[t] + "-" + frUnits[u]
	case t == 7:
		if n == 71 {
			return "soixante et onze"
		}
		return "soixante-" + frUnits[n-60]
	case t == 8:
		if u == 0 {
			if last {
				return "quatre-vingts"
			}
			return "quatre-vingt"
		}
		return "quatre-vingt-" + frUnits[u]
	}
	return "quatre-vingt-" + frUnits[n-80]
}

func frBelow1000(n int64, last bool) string {
	h, r := n/100, n%100
	var parts []string
	switch {
	case h == 1:
		parts = append(parts, "cent")
	case h > 1:
		w := frUnits[h] + " cent"
		if r == 0 && last {
			w += "s"
		}
		parts = append(parts, w)
	}
	if r > 0 {
		parts = append(parts, frBelow100(r, last))
	}
	return strings.Join(parts, " ")
}

func frWords(n int64) string {
	type scale struct {
		v        int64
		one, mul string
	}
	var parts []string
	for _, s := range []scale{{1_000_000_000, "un milliard", "milliards"}, {1_000_000, "un million", "millions"}, {1000, "mille", "mille"}} {
		q := n / s.v
		n %= s.v
		if q == 0 {
			continue
		}
		if q == 1 {
			parts = append(parts, s.one)
		} else {
			// "cents" and "vingts" keep their s before million and milliard, not before mille
			parts = append(parts, frBelow1000(q, s.v != 1000)+" "+s.mul)
		}
	}
	if n > 0 {
		parts = append(parts, frBelow1000(n, true))
	}
	return strings.Join(parts, " ")
}

// qtyText prints a quantity without trailing zeros.
func qtyText(q float64) string { return fmt.Sprintf("%g", q) }
