package procurement

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

type SupplierInvoice struct {
	ID          string    `json:"id"`
	POID        string    `json:"po_id"`
	Supplier    string    `json:"supplier"`
	Number      string    `json:"number"`
	Net         int64     `json:"net"`
	VAT         int64     `json:"vat"`
	Total       int64     `json:"total"`
	Status      string    `json:"status"`
	ApprovedBy  *string   `json:"approved_by"`
	PaidBy      *string   `json:"paid_by"`
	SubmittedAt time.Time `json:"submitted_at"`
}

const siCols = `id, po_id, supplier, number, net, vat, total, status, approved_by, paid_by, submitted_at`

func scanSI(row pgx.Row) (*SupplierInvoice, error) {
	var x SupplierInvoice
	err := row.Scan(&x.ID, &x.POID, &x.Supplier, &x.Number, &x.Net, &x.VAT, &x.Total, &x.Status, &x.ApprovedBy, &x.PaidBy, &x.SubmittedAt)
	if isNF(err) {
		return nil, nf("supplier invoice")
	}
	return &x, err
}

func submitInvoice(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Number string `json:"number"`
		Net    int64  `json:"net"`
		VAT    int64  `json:"vat"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Number) == "" || in.Net <= 0 || in.VAT < 0 {
		return nil, bad("number, a positive net amount and a non-negative VAT are required")
	}
	ctx := r.Context()
	x, err := loadPO(ctx, s, r.PathValue("id"), false) // RLS shows a supplier only their own issued orders
	if err != nil {
		return nil, err
	}
	if x.Supplier != s.Claims.TenantID || (x.Status != "issued" && x.Status != "received") {
		return nil, nf("purchase order")
	}
	si, err := scanSI(s.Tx.QueryRow(ctx, `INSERT INTO procurement.supplier_invoices (po_id, supplier, number, net, vat, total) VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+siCols,
		x.ID, s.Claims.TenantID, strings.TrimSpace(in.Number), in.Net, in.VAT, in.Net+in.VAT))
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return nil, platform.Err(http.StatusConflict, "Duplicate invoice", "this invoice number was already submitted")
		}
		return nil, err
	}
	if err := s.Emit(ctx, "supplier_invoice.submitted", map[string]any{"id": si.ID, "po_id": x.ID, "total": si.Total}); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: si}, nil
}

func listInvoices(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+siCols+` FROM procurement.supplier_invoices ORDER BY submitted_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*SupplierInvoice{}
	for rows.Next() {
		x, err := scanSI(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func lockSI(r *http.Request, s *platform.Scope) (*SupplierInvoice, error) {
	return scanSI(s.Tx.QueryRow(r.Context(), `SELECT `+siCols+` FROM procurement.supplier_invoices WHERE id::text=$1 FOR UPDATE`, r.PathValue("id")))
}

// approveInvoice is the three-way match: the order, the goods received and the invoice must agree.
func (m *module) approveInvoice(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	si, err := lockSI(r, s)
	if err != nil {
		return nil, err
	}
	if si.Status != "submitted" {
		return nil, platform.Err(http.StatusConflict, "Cannot approve", "only a submitted invoice can be approved")
	}
	po, err := loadPO(ctx, s, si.POID, true)
	if err != nil {
		return nil, err
	}
	if po.Status != "received" {
		return nil, platform.Err(http.StatusConflict, "Match failed", "the goods have not all been received against the purchase order")
	}
	var already int64
	if err := s.Tx.QueryRow(ctx, `SELECT coalesce(sum(net),0) FROM procurement.supplier_invoices WHERE po_id=$1 AND status IN ('approved','paid') AND id<>$2`, po.ID, si.ID).Scan(&already); err != nil {
		return nil, err
	}
	if already+si.Net > po.Net {
		return nil, platform.Err(http.StatusUnprocessableEntity, "Match failed", "invoiced amount exceeds the purchase order")
	}
	out, err := scanSI(s.Tx.QueryRow(ctx, `UPDATE procurement.supplier_invoices SET status='approved', approved_by=$2 WHERE id=$1 RETURNING `+siCols, si.ID, s.Claims.Sub))
	if err != nil {
		return nil, err
	}
	if m.post != nil {
		if err := m.post.SupplierInvoiceApproved(ctx, s, InvoiceEntry{Number: out.Number, Supplier: out.Supplier, Net: out.Net, VAT: out.VAT, Total: out.Total, Date: time.Now().UTC()}); err != nil {
			return nil, err
		}
	}
	if err := s.Audit(ctx, "approve", "supplier_invoice", out.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}

func rejectInvoice(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	si, err := lockSI(r, s)
	if err != nil {
		return nil, err
	}
	if si.Status != "submitted" {
		return nil, platform.Err(http.StatusConflict, "Cannot reject", "only a submitted invoice can be rejected")
	}
	out, err := scanSI(s.Tx.QueryRow(r.Context(), `UPDATE procurement.supplier_invoices SET status='rejected' WHERE id=$1 RETURNING `+siCols, si.ID))
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}

// payInvoice needs a different person from the approver, and verified bank details.
func (m *module) payInvoice(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Method string `json:"method"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if in.Method != "bank" && in.Method != "cash" {
		return nil, bad("method must be bank or cash")
	}
	ctx := r.Context()
	si, err := lockSI(r, s)
	if err != nil {
		return nil, err
	}
	if si.Status != "approved" {
		return nil, platform.Err(http.StatusConflict, "Cannot pay", "only an approved invoice can be paid")
	}
	if si.ApprovedBy != nil && *si.ApprovedBy == s.Claims.Sub {
		return nil, platform.Err(http.StatusForbidden, "Second person required", "the person who approved this invoice cannot also pay it")
	}
	var bank string
	if err := s.Tx.QueryRow(ctx, `SELECT bank_status FROM procurement.suppliers WHERE tenant_id=$1`, si.Supplier).Scan(&bank); err != nil {
		return nil, err
	}
	if in.Method == "bank" && bank != "verified" {
		return nil, platform.Err(http.StatusConflict, "Bank details not verified", "verify the supplier's bank details by call-back before paying")
	}
	out, err := scanSI(s.Tx.QueryRow(ctx, `UPDATE procurement.supplier_invoices SET status='paid', paid_by=$2 WHERE id=$1 RETURNING `+siCols, si.ID, s.Claims.Sub))
	if err != nil {
		return nil, err
	}
	if m.post != nil {
		if err := m.post.SupplierInvoicePaid(ctx, s, PaidEntry{Number: out.Number, Supplier: out.Supplier, Method: in.Method, Total: out.Total, Date: time.Now().UTC()}); err != nil {
			return nil, err
		}
	}
	if err := s.Emit(ctx, "supplier_invoice.paid", map[string]any{"id": out.ID, "total": out.Total}); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "pay", "supplier_invoice", out.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: out}, nil
}
