package payroll

import (
	"context"
	"net/http"

	"github.com/opeshealthsystems/heckate/api/internal/docs"
	"github.com/opeshealthsystems/heckate/api/internal/modules/documents"
	"github.com/opeshealthsystems/heckate/api/internal/platform"
)

// issueSlipDocs gives every payslip of a just-approved run its document reference. The register title
// carries only the month, never a name or an amount.
func issueSlipDocs(ctx context.Context, s *platform.Scope, run *Run) error {
	slips, err := slipsOf(ctx, s, run.ID)
	if err != nil {
		return err
	}
	for _, p := range slips {
		doc, err := documents.Issue(ctx, s, "PAY", "Payslip "+run.Period, "", "")
		if err != nil {
			return err
		}
		if _, err := s.Tx.Exec(ctx, `INSERT INTO payroll.payslip_docs (payslip_id, ref) VALUES ($1,$2)`, p.ID, doc.Ref); err != nil {
			return err
		}
	}
	return nil
}

func toPayLines(in []Line) []docs.PayLine {
	out := make([]docs.PayLine, len(in))
	for i, l := range in {
		out[i] = docs.PayLine{Label: l.Label, Base: l.Base, RateBP: l.RateBP, Amount: l.Amount}
	}
	return out
}

// slipPDF serves one payslip as the canonical PDF. Row-level security decides who can see it: payroll
// staff, or the employee themselves once the run is approved.
func (m *mod) slipPDF(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	p, err := scanSlip(s.Tx.QueryRow(ctx, `SELECT `+slipCols+` FROM payroll.payslips p WHERE p.id::text = $1`, r.PathValue("id")))
	if isNF(err) {
		return nil, nf("payslip")
	}
	if err != nil {
		return nil, err
	}
	var ref string
	if err := s.Tx.QueryRow(ctx, `SELECT ref FROM payroll.payslip_docs WHERE payslip_id = $1`, p.ID).Scan(&ref); err != nil {
		if isNF(err) {
			return nil, platform.Err(http.StatusConflict, "Not released", "this payslip has no document yet")
		}
		return nil, err
	}
	return documents.ServePDF(ctx, s, ref, func() ([]byte, error) {
		entry, err := documents.Lookup(ctx, s, ref)
		if err != nil {
			return nil, err
		}
		return docs.RenderPayslip(m.docs, ref, entry.IssuedAt, docs.Payslip{
			Period: p.Period, Employee: p.FullName, Gross: p.Gross, Deductions: toPayLines(p.Deductions),
			IRPP: p.IRPP, CAC: p.CAC, Net: p.Net, Employer: toPayLines(p.Employer),
		})
	})
}
