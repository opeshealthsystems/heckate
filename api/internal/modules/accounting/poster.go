package accounting

import (
	"context"

	"github.com/opeshealthsystems/heckate/api/internal/modules/sales"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// SalesPoster turns sales events into ledger entries in the same transaction as the sale.
type SalesPoster struct{}

var _ sales.Poster = SalesPoster{}

func nz(lines ...Line) []Line {
	out := lines[:0:0]
	for _, l := range lines {
		if l.Debit != 0 || l.Credit != 0 {
			out = append(out, l)
		}
	}
	return out
}

func (SalesPoster) InvoiceIssued(ctx context.Context, s *platform.Scope, e sales.InvoiceEntry) error {
	_, err := Post(ctx, s, Entry{Journal: "VT", Date: e.Date, Ref: e.Ref, Narration: "Invoice " + e.Ref, Source: "invoice", SourceRef: e.Ref,
		Lines: nz(
			Line{Account: "411", Debit: e.Total, Party: e.Tenant},
			Line{Account: "706", Credit: e.Net},
			Line{Account: "4431", Credit: e.TVA},
			Line{Account: "4439", Credit: e.CAC},
		)})
	return err
}

// settlementAccount maps a payment method to the cash or bank account that receives it.
func settlementAccount(method string) (journal, account string) {
	switch method {
	case "cash":
		return "CA", "571"
	case "momo", "orange_money":
		return "BQ", "5211"
	default: // bank, card
		return "BQ", "521"
	}
}

func (SalesPoster) PaymentReceived(ctx context.Context, s *platform.Scope, e sales.PaymentEntry) error {
	journal, account := settlementAccount(e.Method)
	_, err := Post(ctx, s, Entry{Journal: journal, Date: e.Date, Ref: e.InvoiceRef, Narration: "Payment for " + e.InvoiceRef, Source: "payment", SourceRef: e.InvoiceRef,
		Lines: []Line{
			{Account: account, Debit: e.Amount},
			{Account: "411", Credit: e.Amount, Party: e.Tenant},
		}})
	return err
}

func (SalesPoster) CreditNoteIssued(ctx context.Context, s *platform.Scope, e sales.CreditEntry) error {
	_, err := Post(ctx, s, Entry{Journal: "VT", Date: e.Date, Ref: e.Ref, Narration: "Credit note " + e.Ref + " for " + e.InvoiceRef, Source: "credit_note", SourceRef: e.Ref,
		Lines: nz(
			Line{Account: "709", Debit: e.Net},
			Line{Account: "4431", Debit: e.TVA},
			Line{Account: "4439", Debit: e.CAC},
			Line{Account: "411", Credit: e.Total, Party: e.Tenant},
		)})
	return err
}
