-- A numbered, verifiable document reference for each payslip, issued when the run is approved.
-- Kept apart from payroll.payslips because payslips of an approved run are frozen.
CREATE TABLE payroll.payslip_docs (
  payslip_id uuid PRIMARY KEY REFERENCES payroll.payslips(id),
  ref        text NOT NULL UNIQUE
);
ALTER TABLE payroll.payslip_docs ENABLE ROW LEVEL SECURITY; ALTER TABLE payroll.payslip_docs FORCE ROW LEVEL SECURITY;
-- Whoever can read the payslip (finance/HR/CEO, or the employee once released) can read its reference.
CREATE POLICY docs_read ON payroll.payslip_docs FOR SELECT USING (EXISTS (SELECT 1 FROM payroll.payslips p WHERE p.id = payslip_id));
CREATE POLICY docs_ins  ON payroll.payslip_docs FOR INSERT WITH CHECK (payroll.can_decide());
GRANT SELECT, INSERT ON payroll.payslip_docs TO hekate_app;
