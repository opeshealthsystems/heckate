package docs

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/skip2/go-qrcode"
)

// A Label is one sticker: a QR code that opens the record in the platform, and the words a person reads.
type Label struct {
	Kind  string // ASSET or ITEM, printed small above the name
	Name  string
	Code  string // serial number or SKU
	Extra string // optional third line (client, location)
	URL   string // what the QR code opens
}

const (
	labelCols = 3
	labelRows = 8
	labelW    = 63.5
	labelH    = 33.9
	labelLeft = 9.75
	labelTop  = 12.9
	labelQR   = 24.0
	maxLabels = labelCols * labelRows * 20
)

// RenderLabels lays labels out on A4 sheets of 3 x 8 (a common 24-up sticker size), so the page can go
// straight into a label printer. The same inputs always produce the same bytes.
func RenderLabels(o Options, labels []Label, at time.Time) ([]byte, error) {
	if len(labels) == 0 {
		return nil, fmt.Errorf("no labels")
	}
	if len(labels) > maxLabels {
		return nil, fmt.Errorf("at most %d labels at once", maxLabels)
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(o.Compress)
	pdf.SetCatalogSort(true)
	pdf.SetCreationDate(at.UTC())
	pdf.SetModificationDate(at.UTC())
	pdf.SetTitle("HEKATE labels", false)
	pdf.SetAuthor("HEKATE SYSTEMS SARL", false)
	pdf.SetAutoPageBreak(false, 0)
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	per := labelCols * labelRows
	for i, l := range labels {
		if i%per == 0 {
			pdf.AddPage()
		}
		n := i % per
		x := labelLeft + float64(n%labelCols)*labelW
		y := labelTop + float64(n/labelCols)*labelH
		pdf.SetDrawColor(210, 210, 210)
		pdf.SetLineWidth(0.15)
		pdf.Rect(x+0.5, y+0.5, labelW-1, labelH-1, "D")
		png, err := qrcode.Encode(l.URL, qrcode.Medium, 256)
		if err != nil {
			return nil, err
		}
		name := fmt.Sprintf("qr%d", i)
		pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(png))
		pdf.ImageOptions(name, x+2.5, y+(labelH-labelQR)/2, labelQR, labelQR, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		tx := x + 2.5 + labelQR + 2
		tw := labelW - (tx - x) - 2
		pdf.SetXY(tx, y+3)
		pdf.SetFont("Helvetica", "B", 6.5)
		pdf.SetTextColor(navy[0], navy[1], navy[2])
		pdf.CellFormat(tw, 3, tr("HEKATE SYSTEMS  "+strings.ToUpper(l.Kind)), "", 1, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Helvetica", "B", 8.5)
		pdf.SetX(tx)
		lines := pdf.SplitLines([]byte(tr(l.Name)), tw)
		if len(lines) > 2 {
			lines = lines[:2]
		}
		for _, ln := range lines {
			pdf.SetX(tx)
			pdf.CellFormat(tw, 4, string(ln), "", 1, "L", false, 0, "")
		}
		pdf.SetFont("Courier", "B", 8.5)
		pdf.SetX(tx)
		pdf.CellFormat(tw, 4.4, tr(clip(l.Code, 22)), "", 1, "L", false, 0, "")
		if l.Extra != "" {
			pdf.SetFont("Helvetica", "", 6.8)
			pdf.SetTextColor(90, 90, 90)
			pdf.SetX(tx)
			pdf.CellFormat(tw, 3.4, tr(clip(l.Extra, 34)), "", 1, "L", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
		}
		pdf.SetFont("Helvetica", "I", 5.8)
		pdf.SetTextColor(120, 120, 120)
		pdf.SetXY(tx, y+labelH-6)
		pdf.CellFormat(tw, 3, tr("Scan to open the record"), "", 1, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "."
}
