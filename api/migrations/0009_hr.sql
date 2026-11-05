-- HR: employee records, leave and expense claims. Payroll is deliberately not built (needs accountant-supplied rules).
CREATE SCHEMA IF NOT EXISTS hr;

CREATE TABLE hr.employees (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor       text NOT NULL UNIQUE,          -- the token subject
  full_name   text NOT NULL,
  position    text NOT NULL DEFAULT '',
  department  text NOT NULL DEFAULT '',
  hire_date   date NOT NULL,
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','left')),
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE hr.settings (
  id                    boolean PRIMARY KEY DEFAULT true CHECK (id),
  annual_leave_days     int NOT NULL CHECK (annual_leave_days >= 0)
);
-- Placeholder entitlement, to be confirmed against the labour code and contracts (docs/BLOCKERS.md).
INSERT INTO hr.settings VALUES (true, 18);
CREATE TABLE hr.holidays (
  day  date PRIMARY KEY,
  name text NOT NULL
);
CREATE TABLE hr.leave_requests (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor       text NOT NULL,
  kind        text NOT NULL CHECK (kind IN ('annual','sick','unpaid','other')),
  start_date  date NOT NULL,
  end_date    date NOT NULL,
  days        int  NOT NULL CHECK (days > 0),
  reason      text NOT NULL DEFAULT '',
  status      text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','cancelled')),
  decided_by  text,
  decided_at  timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now(),
  CHECK (end_date >= start_date)
);
CREATE TABLE hr.expense_claims (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor        text NOT NULL,
  expense_date date NOT NULL,
  category     text NOT NULL,
  description  text NOT NULL,
  amount       bigint NOT NULL CHECK (amount > 0),
  status       text NOT NULL DEFAULT 'submitted' CHECK (status IN ('submitted','approved','rejected','paid')),
  decided_by   text,
  paid_by      text,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON hr.leave_requests (actor, start_date);
CREATE INDEX ON hr.expense_claims (actor);

-- Decided leave and paid claims are records of fact.
CREATE FUNCTION hr.guard_leave() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  -- Only reviewers decide, and never on their own request, whatever route the update takes.
  IF NEW.status IN ('approved','rejected') AND OLD.status = 'pending' THEN
    IF NOT hr.is_reviewer() THEN RAISE EXCEPTION 'only HR or management can decide leave'; END IF;
    IF NEW.actor = current_setting('app.actor', true) THEN RAISE EXCEPTION 'you cannot decide your own leave'; END IF;
  END IF;
  IF OLD.status IN ('approved','rejected','cancelled') AND NEW.status <> OLD.status THEN RAISE EXCEPTION 'a decided leave request cannot change status'; END IF;
  IF OLD.status IN ('approved','rejected','cancelled') AND (NEW.start_date <> OLD.start_date OR NEW.end_date <> OLD.end_date OR NEW.days <> OLD.days OR NEW.actor <> OLD.actor OR NEW.kind <> OLD.kind) THEN
    RAISE EXCEPTION 'decided leave requests are immutable';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER leave_guard BEFORE UPDATE ON hr.leave_requests FOR EACH ROW EXECUTE FUNCTION hr.guard_leave();
CREATE FUNCTION hr.guard_claim() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.status IN ('approved','rejected') AND OLD.status = 'submitted' THEN
    IF NOT hr.is_reviewer() THEN RAISE EXCEPTION 'only management or HR can decide expense claims'; END IF;
    IF NEW.actor = current_setting('app.actor', true) THEN RAISE EXCEPTION 'you cannot decide your own expense claim'; END IF;
  END IF;
  IF NEW.status = 'paid' AND OLD.status <> 'approved' THEN RAISE EXCEPTION 'only an approved claim can be paid'; END IF;
  IF NEW.status = 'paid' AND NOT (platform.has_role('finance') OR hr.is_reviewer()) THEN RAISE EXCEPTION 'only finance can pay claims'; END IF;
  IF OLD.status IN ('paid') AND (NEW.amount <> OLD.amount OR NEW.status <> OLD.status) THEN RAISE EXCEPTION 'paid claims are immutable'; END IF;
  IF NEW.amount <> OLD.amount OR NEW.actor <> OLD.actor THEN RAISE EXCEPTION 'claim amount and owner cannot change'; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER claim_guard BEFORE UPDATE ON hr.expense_claims FOR EACH ROW EXECUTE FUNCTION hr.guard_claim();

CREATE FUNCTION hr.is_hr() RETURNS boolean LANGUAGE sql STABLE AS $$ SELECT platform.has_role('hr') OR platform.has_role('ceo') $$;
CREATE FUNCTION hr.is_reviewer() RETURNS boolean LANGUAGE sql STABLE AS $$ SELECT platform.has_role('hr') OR platform.has_role('manager') OR platform.has_role('ceo') $$;
CREATE FUNCTION hr.is_me(a text) RETURNS boolean LANGUAGE sql STABLE AS $$ SELECT a = current_setting('app.actor', true) $$;
GRANT EXECUTE ON FUNCTION hr.is_hr, hr.is_reviewer, hr.is_me TO hekate_app;

ALTER TABLE hr.employees ENABLE ROW LEVEL SECURITY;       ALTER TABLE hr.employees FORCE ROW LEVEL SECURITY;
ALTER TABLE hr.settings ENABLE ROW LEVEL SECURITY;        ALTER TABLE hr.settings FORCE ROW LEVEL SECURITY;
ALTER TABLE hr.holidays ENABLE ROW LEVEL SECURITY;        ALTER TABLE hr.holidays FORCE ROW LEVEL SECURITY;
ALTER TABLE hr.leave_requests ENABLE ROW LEVEL SECURITY;  ALTER TABLE hr.leave_requests FORCE ROW LEVEL SECURITY;
ALTER TABLE hr.expense_claims ENABLE ROW LEVEL SECURITY;  ALTER TABLE hr.expense_claims FORCE ROW LEVEL SECURITY;

CREATE POLICY emp_hr   ON hr.employees FOR ALL USING (hr.is_hr()) WITH CHECK (hr.is_hr());
CREATE POLICY emp_view ON hr.employees FOR SELECT USING (hr.is_reviewer() OR hr.is_me(actor));
CREATE POLICY set_read ON hr.settings FOR SELECT USING (platform.is_staff());
CREATE POLICY hol_read ON hr.holidays FOR SELECT USING (platform.is_staff());
CREATE POLICY hol_hr   ON hr.holidays FOR ALL USING (hr.is_hr()) WITH CHECK (hr.is_hr());

CREATE POLICY leave_own    ON hr.leave_requests FOR ALL USING (hr.is_me(actor)) WITH CHECK (hr.is_me(actor));
CREATE POLICY leave_review ON hr.leave_requests FOR ALL USING (hr.is_reviewer()) WITH CHECK (hr.is_reviewer());
CREATE POLICY claim_own    ON hr.expense_claims FOR ALL USING (hr.is_me(actor)) WITH CHECK (hr.is_me(actor));
CREATE POLICY claim_review ON hr.expense_claims FOR ALL USING (hr.is_reviewer() OR platform.has_role('finance')) WITH CHECK (hr.is_reviewer() OR platform.has_role('finance'));

GRANT USAGE ON SCHEMA hr TO hekate_app;
GRANT SELECT, INSERT, UPDATE ON hr.employees, hr.leave_requests, hr.expense_claims TO hekate_app;
GRANT SELECT ON hr.settings TO hekate_app;
GRANT SELECT, INSERT, UPDATE ON hr.holidays TO hekate_app;
