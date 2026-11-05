package sales

import "math"

// Line is a quote or invoice line. Quantity has at most two decimals.
type Line struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   int64   `json:"unit_price"` // XAF
}

// qty100 converts a quantity to hundredths so all arithmetic stays in integers.
func qty100(q float64) int64 { return int64(math.Round(q * 100)) }

// LineAmount is quantity x unit price in XAF, rounded half up.
func LineAmount(l Line) int64 { return (qty100(l.Quantity)*l.UnitPrice + 50) / 100 }

// Taxes computes TVA on the net amount and CAC as a percentage of that TVA, each rounded half up,
// on the document total (not per line) so the figures match what a client can recompute.
func Taxes(net int64, tvaBP, cacPctOfTVA int) (tva, cac int64) {
	tva = (net*int64(tvaBP) + 5000) / 10000
	cac = (tva*int64(cacPctOfTVA) + 50) / 100
	return
}

// Totals returns net, TVA, CAC and the gross total for a set of lines.
func Totals(lines []Line, tvaBP, cacPctOfTVA int) (net, tva, cac, total int64) {
	for _, l := range lines {
		net += LineAmount(l)
	}
	tva, cac = Taxes(net, tvaBP, cacPctOfTVA)
	return net, tva, cac, net + tva + cac
}
