package docs

import (
	"strings"
	"time"
)

// A Report is a computed printout (a statement of account, a trial balance, a ledger): clean tables in the HEKATE
// frame, with the classification banner and a footer saying who produced it and when. It is not a numbered,
// verifiable document: it changes whenever the books do, so it carries no reference or QR code.
type Report struct {
	Title       string
	Subtitle    string
	Class       string // PUBLIC, INTERNAL, CONFIDENTIAL or RESTRICTED
	GeneratedBy string
	At          time.Time
	Facts       [][2]string // label and value pairs shown under the title (client, period, as at)
	Sections    []ReportSection
	Note        string
}

type ReportSection struct {
	Heading string
	Columns []ReportColumn
	Rows    [][]string
	Total   []string // optional totals row, same width as Columns
	Empty   string   // shown when there are no rows
}

type ReportColumn struct {
	Header string
	Width  float64 // millimetres; 0 shares the rest equally
	Right  bool    // numbers
}

const reportWidth = 180.0

// RenderReport draws a report. Rows that do not fit move to a new page and the column headings repeat.
func RenderReport(o Options, r Report) ([]byte, error) {
	class := strings.ToUpper(r.Class)
	if class == "" {
		class = "CONFIDENTIAL"
	}
	at := r.At
	if at.IsZero() {
		at = time.Now().UTC()
	}
	note := "Report produced " + at.UTC().Format("02 Jan 2006 15:04") + " UTC"
	if r.GeneratedBy != "" {
		note += " by " + r.GeneratedBy
	}
	note += "  |  not a numbered document"
	d := newDocFrame(o, "REPORT", r.Title, at, false, note, class)
	d.title(strings.ToUpper(r.Title), r.Subtitle)
	for _, f := range r.Facts {
		d.kv(f[0], f[1])
	}
	if len(r.Facts) > 0 {
		d.pdf.Ln(2)
	}
	for _, s := range r.Sections {
		d.reportSection(s)
	}
	if r.Note != "" {
		d.pdf.Ln(2)
		d.note(r.Note)
	}
	return d.bytes()
}

func columnWidths(cols []ReportColumn) []float64 {
	w := make([]float64, len(cols))
	fixed, flex := 0.0, 0
	for i, c := range cols {
		if c.Width > 0 {
			w[i] = c.Width
			fixed += c.Width
		} else {
			flex++
		}
	}
	for i, c := range cols {
		if c.Width == 0 && flex > 0 {
			w[i] = (reportWidth - fixed) / float64(flex)
		}
	}
	return w
}

func (d *doc) reportSection(s ReportSection) {
	p := d.pdf
	w := columnWidths(s.Columns)
	if s.Heading != "" {
		if p.GetY() > 297-22-30 {
			p.AddPage()
		}
		p.SetFont("Helvetica", "B", 11.5)
		p.SetTextColor(navy[0], navy[1], navy[2])
		p.CellFormat(0, 8, d.tr(s.Heading), "", 1, "L", false, 0, "")
		p.SetTextColor(0, 0, 0)
	}
	head := func() {
		p.SetFont("Helvetica", "B", 8.5)
		p.SetFillColor(bandFill[0], bandFill[1], bandFill[2])
		p.SetTextColor(navy[0], navy[1], navy[2])
		for i, c := range s.Columns {
			al := "L"
			if c.Right {
				al = "R"
			}
			p.CellFormat(w[i], 7, d.tr(c.Header), "B", 0, al, true, 0, "")
		}
		p.Ln(-1)
		p.SetTextColor(0, 0, 0)
	}
	head()
	if len(s.Rows) == 0 {
		p.SetFont("Helvetica", "I", 9)
		msg := s.Empty
		if msg == "" {
			msg = "Nothing to show."
		}
		p.CellFormat(0, 8, d.tr(msg), "", 1, "L", false, 0, "")
	}
	for _, row := range s.Rows {
		// Wrapped text columns decide the row height; numbers sit on the first line.
		lines := 1
		p.SetFont("Helvetica", "", 8.8)
		for i, c := range s.Columns {
			if !c.Right && i < len(row) {
				if n := len(p.SplitLines([]byte(d.tr(row[i])), w[i]-1.5)); n > lines {
					lines = n
				}
			}
		}
		h := float64(lines)*4.4 + 1.6
		if p.GetY()+h > 297-22-8 {
			p.AddPage()
			head()
		}
		x, y := p.GetX(), p.GetY()
		p.SetFont("Helvetica", "", 8.8)
		cx := x
		for i, c := range s.Columns {
			txt := ""
			if i < len(row) {
				txt = row[i]
			}
			p.SetXY(cx, y)
			if c.Right {
				p.CellFormat(w[i], 4.4, d.tr(txt), "", 0, "R", false, 0, "")
			} else {
				p.MultiCell(w[i], 4.4, d.tr(txt), "", "L", false)
			}
			cx += w[i]
		}
		p.SetDrawColor(225, 225, 225)
		p.Line(x, y+h-0.8, x+reportWidth, y+h-0.8)
		p.SetXY(x, y+h)
	}
	if len(s.Total) > 0 {
		p.SetFont("Helvetica", "B", 9.2)
		p.SetDrawColor(120, 120, 120)
		p.Line(15, p.GetY(), 15+reportWidth, p.GetY())
		for i, c := range s.Columns {
			al := "L"
			if c.Right {
				al = "R"
			}
			txt := ""
			if i < len(s.Total) {
				txt = s.Total[i]
			}
			p.CellFormat(w[i], 7, d.tr(txt), "", 0, al, false, 0, "")
		}
		p.Ln(-1)
	}
	p.Ln(4)
}

// Day formats a date for a report.
func Day(t time.Time) string { return date(t) }
