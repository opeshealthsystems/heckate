-- Payroll: compensation, a versioned rules configuration that must be validated by a second person
-- (the accountant gate) before any run, monthly runs with dual control, and ledger posting.
-- The seeded rates are a starting point taken from public Cameroon rules and are NOT validated.
CREATE SCHEMA IF NOT EXISTS payroll;

CREATE FUNCTION payroll.can_prepare() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT platform.has_role('hr') OR platform.has_role('finance') OR platform.has_role('manager') OR platform.has_role('ceo') $$;
CREATE FUNCTION payroll.can_read() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT payroll.can_prepare() OR platform.has_role('auditor') $$;
CREATE FUNCTION payroll.can_decide() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT platform.has_role('finance') OR platform.has_role('manager') OR platform.has_role('ceo') $$;
GRANT EXECUTE ON FUNCTION payroll.can_prepare, payroll.can_read, payroll.can_decide TO hekate_app;

CREATE TABLE payroll.compensation (
  employee_id    uuid NOT NULL REFERENCES hr.employees(id),
  effective_from date NOT NULL,
  base_salary    bigint NOT NULL CHECK (base_salary >= 0),
  allowances     bigint NOT NULL DEFAULT 0 CHECK (allowances >= 0),
  set_by         text NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (employee_id, effective_from)
);

CREATE TABLE payroll.config (
  id           boolean PRIMARY KEY DEFAULT true CHECK (id),
  config       jsonb NOT NULL,
  edited_by    text NOT NULL,
  edited_at    timestamptz NOT NULL DEFAULT now(),
  validated_by text,
  validated_at timestamptz
);
INSERT INTO payroll.config (config, edited_by) VALUES ($c$
{
  "abatement_bp": 3000,
  "personal_deduction": 500000,
  "cac_bp": 1000,
  "rules": [
    {"code":"CNPS_PVID_EMP","label":"Pension (employee)","party":"employee","rate_bp":280,"ceiling":750000,"tax_deductible":true},
    {"code":"CFC_EMP","label":"Housing fund (employee)","party":"employee","rate_bp":100,"ceiling":0,"tax_deductible":false},
    {"code":"CNPS_PVID_ER","label":"Pension (employer)","party":"employer","rate_bp":420,"ceiling":750000,"tax_deductible":false},
    {"code":"CNPS_PF_ER","label":"Family allowances (employer)","party":"employer","rate_bp":700,"ceiling":750000,"tax_deductible":false},
    {"code":"CNPS_AT_ER","label":"Work accidents (employer)","party":"employer","rate_bp":175,"ceiling":750000,"tax_deductible":false},
    {"code":"CFC_ER","label":"Housing fund (employer)","party":"employer","rate_bp":150,"ceiling":0,"tax_deductible":false},
    {"code":"FNE_ER","label":"Employment fund (employer)","party":"employer","rate_bp":100,"ceiling":0,"tax_deductible":false}
  ],
  "brackets": [
    {"upto":2000000,"rate_bp":1000},
    {"upto":3000000,"rate_bp":1500},
    {"upto":5000000,"rate_bp":2500},
    {"upto":null,"rate_bp":3500}
  ]
}
$c$::jsonb, 'system');

-- Any change to the rules clears the validation; a validation needs someone other than the editor.
CREATE FUNCTION payroll.config_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.config IS DISTINCT FROM OLD.config THEN
    NEW.validated_by := NULL; NEW.validated_at := NULL; NEW.edited_at := now();
  ELSIF NEW.validated_by IS NOT NULL AND NEW.validated_by IS DISTINCT FROM OLD.validated_by THEN
    IF NEW.validated_by = OLD.edited_by THEN RAISE EXCEPTION 'payroll rules must be validated by someone other than the person who edited them'; END IF;
    NEW.validated_at := now();
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER config_guard BEFORE UPDATE ON payroll.config FOR EACH ROW EXECUTE FUNCTION payroll.config_guard();

CREATE TABLE payroll.runs (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  period        date NOT NULL UNIQUE CHECK (period = date_trunc('month', period)::date),
  status        text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','approved','paid')),
  config        jsonb NOT NULL,
  prepared_by   text NOT NULL,
  approved_by   text,
  paid_by       text,
  total_gross   bigint NOT NULL DEFAULT 0,
  total_net     bigint NOT NULL DEFAULT 0,
  total_cost    bigint NOT NULL DEFAULT 0,
  created_at    timestamptz NOT NULL DEFAULT now(),
  approved_at   timestamptz,
  paid_at       timestamptz
);
CREATE TABLE payroll.payslips (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id            uuid NOT NULL REFERENCES payroll.runs(id),
  period            date NOT NULL,
  employee_id       uuid NOT NULL REFERENCES hr.employees(id),
  actor             text NOT NULL,
  full_name         text NOT NULL,
  gross             bigint NOT NULL,
  deductions        jsonb NOT NULL,
  irpp              bigint NOT NULL,
  cac               bigint NOT NULL,
  net               bigint NOT NULL,
  employer_lines    jsonb NOT NULL,
  employer_cost     bigint NOT NULL,
  UNIQUE (run_id, employee_id)
);
CREATE INDEX ON payroll.payslips (actor);

-- Who may move a run on, and what is frozen once it has moved.
CREATE FUNCTION payroll.runs_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.period <> OLD.period OR NEW.prepared_by <> OLD.prepared_by OR NEW.config IS DISTINCT FROM OLD.config THEN
    RAISE EXCEPTION 'a payroll run cannot change its period, preparer or rules';
  END IF;
  IF OLD.status <> 'draft' AND (NEW.total_gross <> OLD.total_gross OR NEW.total_net <> OLD.total_net OR NEW.total_cost <> OLD.total_cost) THEN
    RAISE EXCEPTION 'an approved payroll run is immutable';
  END IF;
  IF NEW.status <> OLD.status THEN
    IF NOT (OLD.status = 'draft' AND NEW.status = 'approved' OR OLD.status = 'approved' AND NEW.status = 'paid') THEN
      RAISE EXCEPTION 'invalid payroll status change % -> %', OLD.status, NEW.status;
    END IF;
    IF NOT payroll.can_decide() THEN RAISE EXCEPTION 'only finance or management can approve and pay payroll'; END IF;
    IF NEW.status = 'approved' THEN
      IF NEW.approved_by IS NULL OR NEW.approved_by = OLD.prepared_by THEN RAISE EXCEPTION 'payroll must be approved by someone other than the preparer'; END IF;
      IF NOT EXISTS (SELECT 1 FROM payroll.config WHERE validated_by IS NOT NULL AND config = OLD.config) THEN
        RAISE EXCEPTION 'the payroll rules used by this run are not validated';
      END IF;
    END IF;
    IF NEW.status = 'paid' AND (NEW.paid_by IS NULL OR NEW.paid_by = OLD.approved_by OR NEW.paid_by = OLD.prepared_by) THEN
      RAISE EXCEPTION 'payroll must be paid by someone other than the preparer and the approver';
    END IF;
  ELSIF NEW.approved_by IS DISTINCT FROM OLD.approved_by OR NEW.paid_by IS DISTINCT FROM OLD.paid_by THEN
    RAISE EXCEPTION 'approval and payment are recorded only when the status moves';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER runs_guard BEFORE UPDATE ON payroll.runs FOR EACH ROW EXECUTE FUNCTION payroll.runs_guard();

CREATE FUNCTION payroll.payslips_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE st text;
BEGIN
  IF TG_OP = 'DELETE' THEN
    SELECT status INTO st FROM payroll.runs WHERE id = OLD.run_id;
  ELSE
    SELECT status INTO st FROM payroll.runs WHERE id = NEW.run_id;
  END IF;
  IF st IS DISTINCT FROM 'draft' THEN RAISE EXCEPTION 'payslips of an approved run cannot change'; END IF;
  IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER payslips_guard BEFORE INSERT OR UPDATE OR DELETE ON payroll.payslips FOR EACH ROW EXECUTE FUNCTION payroll.payslips_guard();

ALTER TABLE payroll.compensation ENABLE ROW LEVEL SECURITY; ALTER TABLE payroll.compensation FORCE ROW LEVEL SECURITY;
ALTER TABLE payroll.config ENABLE ROW LEVEL SECURITY;       ALTER TABLE payroll.config FORCE ROW LEVEL SECURITY;
ALTER TABLE payroll.runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE payroll.payslips ENABLE ROW LEVEL SECURITY;     ALTER TABLE payroll.payslips FORCE ROW LEVEL SECURITY;

CREATE POLICY comp_read  ON payroll.compensation FOR SELECT USING (platform.has_role('hr') OR platform.has_role('finance') OR platform.has_role('ceo'));
CREATE POLICY comp_write ON payroll.compensation FOR INSERT WITH CHECK (platform.has_role('hr') OR platform.has_role('ceo'));
CREATE POLICY cfg_read   ON payroll.config FOR SELECT USING (payroll.can_read());
CREATE POLICY cfg_write  ON payroll.config FOR UPDATE USING (platform.has_role('finance') OR platform.has_role('ceo')) WITH CHECK (platform.has_role('finance') OR platform.has_role('ceo'));
CREATE POLICY runs_read  ON payroll.runs FOR SELECT USING (payroll.can_read());
CREATE POLICY runs_ins   ON payroll.runs FOR INSERT WITH CHECK (payroll.can_prepare());
CREATE POLICY runs_upd   ON payroll.runs FOR UPDATE USING (payroll.can_prepare()) WITH CHECK (payroll.can_prepare());
CREATE POLICY runs_del   ON payroll.runs FOR DELETE USING (payroll.can_prepare() AND status = 'draft');
-- An employee reads their own payslip once the run is approved. The run row itself (company totals)
-- stays hidden from them, so the release check goes through a definer function; runs is not FORCE RLS
-- so the function's owner can read it.
CREATE FUNCTION payroll.run_released(p_run uuid) RETURNS boolean LANGUAGE sql STABLE SECURITY DEFINER SET search_path = payroll, pg_temp AS $$
  SELECT EXISTS (SELECT 1 FROM payroll.runs WHERE id = p_run AND status IN ('approved','paid'))
$$;
REVOKE ALL ON FUNCTION payroll.run_released FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payroll.run_released TO hekate_app;
CREATE POLICY slips_read ON payroll.payslips FOR SELECT USING (
  payroll.can_read() OR (actor = current_setting('app.actor', true) AND payroll.run_released(run_id)));
CREATE POLICY slips_ins  ON payroll.payslips FOR INSERT WITH CHECK (payroll.can_prepare());
CREATE POLICY slips_upd  ON payroll.payslips FOR UPDATE USING (payroll.can_prepare()) WITH CHECK (payroll.can_prepare());

-- Finance prepares and pays payroll, so it can read the employee register.
CREATE POLICY emp_finance ON hr.employees FOR SELECT USING (platform.has_role('finance'));

-- Accounts used by payroll postings (mapping to be confirmed by the accountant).
ALTER TABLE accounting.accounts DISABLE ROW LEVEL SECURITY;
INSERT INTO accounting.accounts (code, name, class, note) VALUES
  ('422', 'Personnel, rémunérations dues', 4, 'mapping to be confirmed by the accountant'),
  ('431', 'Sécurité sociale', 4, 'mapping to be confirmed by the accountant'),
  ('447', 'État, impôts retenus sur salaires', 4, 'mapping to be confirmed by the accountant'),
  ('664', 'Charges sociales', 6, 'mapping to be confirmed by the accountant')
ON CONFLICT DO NOTHING;
ALTER TABLE accounting.accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounting.accounts FORCE ROW LEVEL SECURITY;

GRANT USAGE ON SCHEMA payroll TO hekate_app;
GRANT SELECT, INSERT ON payroll.compensation, payroll.payslips TO hekate_app;
GRANT UPDATE ON payroll.payslips TO hekate_app;
GRANT SELECT, UPDATE ON payroll.config TO hekate_app;
GRANT SELECT, INSERT, UPDATE ON payroll.runs TO hekate_app;
GRANT DELETE ON payroll.payslips, payroll.runs TO hekate_app;
