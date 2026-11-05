package procurement

import (
	"net/http"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
	"github.com/opeshealthsystems/heckate/api/internal/modules/documents"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// poPDF serves the canonical PDF of an issued purchase order. Suppliers reach only their own orders (RLS).
func (m *module) poPDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	x, err := loadPO(ctx, s, r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	if x.Ref == nil {
		return nil, platform.Err(http.StatusConflict, "Not issued", "only an issued purchase order has a PDF")
	}
	return documents.ServePDF(ctx, s, *x.Ref, func() ([]byte, error) {
		var sup docs.Party
		var rccm, email *string
		if err := s.Tx.QueryRow(ctx, `SELECT legal_name, rccm, email FROM procurement.suppliers WHERE tenant_id::text = $1`, x.Supplier).Scan(&sup.Name, &rccm, &email); err != nil {
			return nil, err
		}
		if rccm != nil {
			sup.RCCM = *rccm
		}
		if email != nil {
			sup.Email = *email
		}
		entry, err := documents.Lookup(ctx, s, *x.Ref)
		if err != nil {
			return nil, err
		}
		lines := make([]docs.Line, len(x.Lines))
		for i, l := range x.Lines {
			lines[i] = docs.Line{Description: l.Description, Quantity: l.Quantity, UnitPrice: l.UnitPrice}
		}
		return docs.RenderPurchaseOrder(m.docs, *x.Ref, entry.IssuedAt, x.Title, sup, lines, x.Net)
	})
}

// rfqPDF serves the canonical PDF of an opened request for quotation. It never prints bids.
func (m *module) rfqPDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	x, err := readRFQ(r, s)
	if err != nil {
		return nil, err
	}
	if x.Ref == nil {
		return nil, platform.Err(http.StatusConflict, "Not opened", "only an opened RFQ has a PDF")
	}
	return documents.ServePDF(ctx, s, *x.Ref, func() ([]byte, error) {
		rows, err := s.Tx.Query(ctx, `SELECT description, quantity::float8 FROM procurement.rfq_lines WHERE rfq_id=$1 ORDER BY position`, x.ID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var lines []docs.QtyLine
		for rows.Next() {
			var l docs.QtyLine
			if err := rows.Scan(&l.Description, &l.Quantity); err != nil {
				return nil, err
			}
			lines = append(lines, l)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		entry, err := documents.Lookup(ctx, s, *x.Ref)
		if err != nil {
			return nil, err
		}
		return docs.RenderRFQ(m.docs, *x.Ref, entry.IssuedAt, x.Title, x.ClosesAt, lines)
	})
}
