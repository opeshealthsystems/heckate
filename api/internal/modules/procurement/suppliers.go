// Package procurement is the Purchasing & Suppliers context.
package procurement

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// Poster receives accounting entries inside the request transaction (see sales.Poster).
type Poster interface {
	SupplierInvoiceApproved(ctx context.Context, s *platform.Scope, e InvoiceEntry) error
	SupplierInvoicePaid(ctx context.Context, s *platform.Scope, e PaidEntry) error
}

type InvoiceEntry struct {
	Number, Supplier string
	Net, VAT, Total  int64
	Date             time.Time
}
type PaidEntry struct {
	Number, Supplier, Method string
	Total                    int64
	Date                     time.Time
}

type module struct{ post Poster }

var (
	buyers   = []string{"ceo", "manager", "procurement"}
	payers   = []string{"ceo", "manager", "finance"}
	approver = []string{"ceo", "manager"}
	receiver = buyers
	anyStaff = []string{"ceo", "manager", "procurement", "finance"}
)

func Register(a *platform.App, p Poster) {
	m := &module{post: p}
	a.Route("POST /v1/suppliers", buyers, platform.Idempotent(createSupplier))
	a.Route("GET /v1/suppliers", anyStaff, listSuppliers)
	a.Route("GET /v1/suppliers/{id}", append([]string{"supplier"}, anyStaff...), getSupplier)
	a.Route("PUT /v1/suppliers/me/bank", []string{"supplier"}, putBank)
	a.Route("POST /v1/suppliers/{id}/bank/verify", payers, platform.Idempotent(verifyBank))
	a.Route("POST /v1/suppliers/{id}/evaluations", buyers, platform.Idempotent(evaluate))

	a.Route("POST /v1/rfqs", buyers, platform.Idempotent(createRFQ))
	a.Route("GET /v1/rfqs", append([]string{"supplier"}, buyers...), listRFQs)
	a.Route("GET /v1/rfqs/{id}", append([]string{"supplier"}, buyers...), getRFQ)
	a.Route("POST /v1/rfqs/{id}/open", buyers, platform.Idempotent(openRFQ))
	a.Route("POST /v1/rfqs/{id}/respond", []string{"supplier"}, platform.Idempotent(respondRFQ))
	a.Route("POST /v1/rfqs/{id}/close", buyers, platform.Idempotent(closeRFQ))
	a.Route("POST /v1/rfqs/{id}/award", buyers, platform.Idempotent(awardRFQ))

	a.Route("POST /v1/purchase-orders", buyers, platform.Idempotent(createPO))
	a.Route("GET /v1/purchase-orders", append([]string{"supplier"}, anyStaff...), listPOs)
	a.Route("GET /v1/purchase-orders/{id}", append([]string{"supplier"}, anyStaff...), getPO)
	a.Route("POST /v1/purchase-orders/{id}/issue", buyers, platform.Idempotent(issuePO))
	a.Route("POST /v1/purchase-orders/{id}/approve", approver, platform.Idempotent(approvePO))
	a.Route("POST /v1/purchase-orders/{id}/receive", receiver, platform.Idempotent(receivePO))
	a.Route("POST /v1/purchase-orders/{id}/invoices", []string{"supplier"}, platform.Idempotent(submitInvoice))
	a.Route("GET /v1/supplier-invoices", append([]string{"supplier"}, anyStaff...), listInvoices)
	a.Route("POST /v1/supplier-invoices/{id}/approve", payers, platform.Idempotent(m.approveInvoice))
	a.Route("POST /v1/supplier-invoices/{id}/reject", payers, platform.Idempotent(rejectInvoice))
	a.Route("POST /v1/supplier-invoices/{id}/pay", payers, platform.Idempotent(m.payInvoice))
}

func nf(what string) error { return platform.Err(http.StatusNotFound, "Not found", what+" not found") }
func bad(msg string) error {
	return platform.Err(http.StatusUnprocessableEntity, "Invalid request", msg)
}
func isNF(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || (err != nil && strings.Contains(err.Error(), "invalid input syntax"))
}

type Supplier struct {
	TenantID   string   `json:"tenant_id"`
	LegalName  string   `json:"legal_name"`
	RCCM       *string  `json:"rccm"`
	Email      *string  `json:"email"`
	Phone      *string  `json:"phone"`
	BankStatus string   `json:"bank_status"`
	BankLast4  *string  `json:"bank_account_last4"`
	VerifiedBy *string  `json:"bank_verified_by"`
	AvgScore   *float64 `json:"avg_score,omitempty"`
}

const supCols = `s.tenant_id, s.legal_name, s.rccm, s.email, s.phone, s.bank_status,
 CASE WHEN s.bank_account IS NULL THEN NULL ELSE right(s.bank_account, 4) END, s.bank_verified_by,
 (SELECT avg((quality+delivery+price)/3.0)::float8 FROM procurement.evaluations e WHERE e.supplier = s.tenant_id)`

func scanSupplier(row pgx.Row) (*Supplier, error) {
	var x Supplier
	err := row.Scan(&x.TenantID, &x.LegalName, &x.RCCM, &x.Email, &x.Phone, &x.BankStatus, &x.BankLast4, &x.VerifiedBy, &x.AvgScore)
	if isNF(err) {
		return nil, nf("supplier")
	}
	return &x, err
}

func createSupplier(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		LegalName string `json:"legal_name"`
		RCCM      string `json:"rccm"`
		Email     string `json:"email"`
		Phone     string `json:"phone"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.LegalName) == "" {
		return nil, bad("legal_name is required")
	}
	ctx := r.Context()
	var id string
	if err := s.Tx.QueryRow(ctx, `INSERT INTO platform.tenants (name, kind) VALUES ($1,'supplier') RETURNING id`, strings.TrimSpace(in.LegalName)).Scan(&id); err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `INSERT INTO procurement.suppliers (tenant_id, legal_name, rccm, email, phone) VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''))`,
		id, strings.TrimSpace(in.LegalName), in.RCCM, in.Email, in.Phone); err != nil {
		return nil, err
	}
	x, err := scanSupplier(s.Tx.QueryRow(ctx, `SELECT `+supCols+` FROM procurement.suppliers s WHERE s.tenant_id=$1`, id))
	if err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "create", "supplier", id); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: x}, nil
}

func listSuppliers(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT `+supCols+` FROM procurement.suppliers s ORDER BY s.legal_name LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Supplier{}
	for rows.Next() {
		x, err := scanSupplier(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return &platform.Result{Status: http.StatusOK, Body: map[string]any{"data": out}}, rows.Err()
}

func getSupplier(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	x, err := scanSupplier(s.Tx.QueryRow(r.Context(), `SELECT `+supCols+` FROM procurement.suppliers s WHERE s.tenant_id::text=$1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: x}, nil
}

// A supplier proposes new bank details. They stay unusable for payment until finance verifies them by call-back.
func putBank(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Account string `json:"account"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	acct := strings.ReplaceAll(strings.TrimSpace(in.Account), " ", "")
	if len(acct) < 8 || len(acct) > 40 {
		return nil, bad("account must be 8 to 40 characters")
	}
	x, err := scanSupplier(s.Tx.QueryRow(r.Context(), `UPDATE procurement.suppliers s SET bank_account=$2, bank_status='pending_callback', bank_verified_by=NULL, bank_note=NULL
		WHERE s.tenant_id::text=$1 RETURNING `+supCols, s.Claims.TenantID, acct))
	if err != nil {
		return nil, err
	}
	if err := s.Emit(r.Context(), "supplier.bank_changed", map[string]any{"supplier": x.TenantID}); err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "bank_change", "supplier", x.TenantID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: x}, nil
}

func verifyBank(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Note string `json:"note"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(in.Note)) < 5 {
		return nil, bad("record who you spoke to and on which number in note")
	}
	var status string
	if err := s.Tx.QueryRow(r.Context(), `SELECT bank_status FROM procurement.suppliers WHERE tenant_id::text=$1 FOR UPDATE`, r.PathValue("id")).Scan(&status); err != nil {
		if isNF(err) {
			return nil, nf("supplier")
		}
		return nil, err
	}
	if status != "pending_callback" {
		return nil, platform.Err(http.StatusConflict, "Nothing to verify", "there are no unverified bank details for this supplier")
	}
	x, err := scanSupplier(s.Tx.QueryRow(r.Context(), `UPDATE procurement.suppliers s SET bank_status='verified', bank_verified_by=$2, bank_note=$3
		WHERE s.tenant_id::text=$1 RETURNING `+supCols, r.PathValue("id"), s.Claims.Sub, strings.TrimSpace(in.Note)))
	if err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "bank_verify", "supplier", x.TenantID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: x}, nil
}

func evaluate(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Quality  int    `json:"quality"`
		Delivery int    `json:"delivery"`
		Price    int    `json:"price"`
		Comment  string `json:"comment"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	for _, v := range []int{in.Quality, in.Delivery, in.Price} {
		if v < 1 || v > 5 {
			return nil, bad("quality, delivery and price are scored 1 to 5")
		}
	}
	if _, err := s.Tx.Exec(r.Context(), `INSERT INTO procurement.evaluations (supplier, evaluator, quality, delivery, price, comment) VALUES ($1,$2,$3,$4,$5,$6)`,
		r.PathValue("id"), s.Claims.Sub, in.Quality, in.Delivery, in.Price, in.Comment); err != nil {
		if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "invalid input syntax") {
			return nil, nf("supplier")
		}
		return nil, err
	}
	x, err := scanSupplier(s.Tx.QueryRow(r.Context(), `SELECT `+supCols+` FROM procurement.suppliers s WHERE s.tenant_id::text=$1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: x}, nil
}
