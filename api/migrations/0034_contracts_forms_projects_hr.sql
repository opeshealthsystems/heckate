-- Contracts (multi-year service agreements), client online forms, project progress, employee files and the
-- company bank-account register.

-- ===== Contracts =====
CREATE SCHEMA IF NOT EXISTS contracts;
GRANT USAGE ON SCHEMA contracts TO hekate_app;

CREATE FUNCTION contracts.can_see() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT platform.has_role('ceo') OR platform.has_role('manager') OR platform.has_role('sales') OR platform.has_role('finance')
         OR platform.has_role('support') OR platform.has_role('engineer') $$;
CREATE FUNCTION contracts.can_write() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT platform.has_role('ceo') OR platform.has_role('manager') OR platform.has_role('sales') $$;
GRANT EXECUTE ON FUNCTION contracts.can_see, contracts.can_write TO hekate_app;

CREATE SEQUENCE contracts.number_seq;
GRANT USAGE ON SEQUENCE contracts.number_seq TO hekate_app;

CREATE TABLE contracts.contracts (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id            uuid NOT NULL REFERENCES platform.tenants(id),
  number               text NOT NULL UNIQUE,
  title                text NOT NULL,
  kind                 text NOT NULL DEFAULT 'maintenance' CHECK (kind IN ('maintenance','support','supply','service','licence')),
  status               text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','suspended','expired','terminated')),
  start_date           date NOT NULL,
  end_date             date NOT NULL,
  value                bigint NOT NULL DEFAULT 0 CHECK (value >= 0),          -- total over the term, XAF
  billing              text NOT NULL DEFAULT 'monthly' CHECK (billing IN ('monthly','quarterly','annual','one_off')),
  period_amount        bigint NOT NULL DEFAULT 0 CHECK (period_amount >= 0),  -- per billing period, XAF
  auto_renew           boolean NOT NULL DEFAULT false,
  renewal_term_months  int NOT NULL DEFAULT 12 CHECK (renewal_term_months BETWEEN 1 AND 120),
  renewal_notice_days  int NOT NULL DEFAULT 60 CHECK (renewal_notice_days BETWEEN 0 AND 365),
  notes                text NOT NULL DEFAULT '',
  ended_reason         text,
  created_by           text NOT NULL,
  created_at           timestamptz NOT NULL DEFAULT now(),
  CHECK (end_date >= start_date)
);
CREATE INDEX ON contracts.contracts (tenant_id, status);
CREATE INDEX ON contracts.contracts (status, end_date);

CREATE TABLE contracts.clauses (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  contract_id uuid NOT NULL REFERENCES contracts.contracts(id) ON DELETE CASCADE,
  tenant_id   uuid NOT NULL REFERENCES platform.tenants(id),
  position    int NOT NULL,
  title       text NOT NULL,
  body        text NOT NULL,
  UNIQUE (contract_id, position)
);
CREATE TABLE contracts.sla (
  contract_id     uuid NOT NULL REFERENCES contracts.contracts(id) ON DELETE CASCADE,
  tenant_id       uuid NOT NULL REFERENCES platform.tenants(id),
  severity        text NOT NULL CHECK (severity IN ('P1','P2','P3','P4')),
  respond_minutes int NOT NULL CHECK (respond_minutes > 0),
  resolve_minutes int NOT NULL CHECK (resolve_minutes >= respond_minutes),
  PRIMARY KEY (contract_id, severity)
);
CREATE TABLE contracts.links (
  contract_id uuid NOT NULL REFERENCES contracts.contracts(id) ON DELETE CASCADE,
  tenant_id   uuid NOT NULL REFERENCES platform.tenants(id),
  kind        text NOT NULL CHECK (kind IN ('asset','project','invoice','ticket')),
  ref_id      uuid NOT NULL,
  label       text NOT NULL DEFAULT '',
  linked_by   text NOT NULL,
  linked_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (contract_id, kind, ref_id)
);
CREATE TABLE contracts.events (
  id          bigserial PRIMARY KEY,
  contract_id uuid NOT NULL REFERENCES contracts.contracts(id) ON DELETE CASCADE,
  tenant_id   uuid NOT NULL REFERENCES platform.tenants(id),
  action      text NOT NULL,
  actor       text NOT NULL,
  detail      jsonb NOT NULL DEFAULT '{}',
  at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON contracts.events (contract_id, id);
CREATE TABLE contracts.renewal_alerts (
  contract_id uuid NOT NULL REFERENCES contracts.contracts(id) ON DELETE CASCADE,
  end_date    date NOT NULL,                 -- the term the alert was for, so a renewal gets fresh alerts
  days_before int NOT NULL CHECK (days_before IN (90,60,30)),
  sent_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (contract_id, end_date, days_before)
);

CREATE FUNCTION contracts.events_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'the contract history is append-only'; END $$;
CREATE TRIGGER contract_events_immutable BEFORE UPDATE OR DELETE ON contracts.events FOR EACH ROW EXECUTE FUNCTION contracts.events_append_only();

-- Clauses are agreed text: they change only while the contract is a draft.
CREATE FUNCTION contracts.clauses_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE st text;
BEGIN
  SELECT status INTO st FROM contracts.contracts WHERE id = COALESCE(NEW.contract_id, OLD.contract_id);
  IF st IS NOT NULL AND st <> 'draft' THEN RAISE EXCEPTION 'clauses of a contract that is not a draft cannot change'; END IF;
  IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER clauses_guard BEFORE INSERT OR UPDATE OR DELETE ON contracts.clauses FOR EACH ROW EXECUTE FUNCTION contracts.clauses_guard();

-- The tightest SLA an active contract of this client promises for a severity (used when a ticket is raised).
CREATE FUNCTION contracts.active_sla(p_tenant uuid, p_severity text)
RETURNS TABLE (contract_id uuid, respond_minutes int, resolve_minutes int) LANGUAGE sql STABLE AS $$
  SELECT c.id, s.respond_minutes, s.resolve_minutes
  FROM contracts.contracts c JOIN contracts.sla s ON s.contract_id = c.id
  WHERE c.tenant_id = p_tenant AND c.status = 'active' AND s.severity = p_severity
    AND current_date BETWEEN c.start_date AND c.end_date
  ORDER BY s.resolve_minutes, c.created_at LIMIT 1
$$;
GRANT EXECUTE ON FUNCTION contracts.active_sla TO hekate_app;
ALTER TABLE support.tickets ADD COLUMN contract_id uuid;
CREATE INDEX ON support.tickets (contract_id) WHERE contract_id IS NOT NULL;

ALTER TABLE contracts.contracts ENABLE ROW LEVEL SECURITY;      ALTER TABLE contracts.contracts FORCE ROW LEVEL SECURITY;
ALTER TABLE contracts.clauses ENABLE ROW LEVEL SECURITY;        ALTER TABLE contracts.clauses FORCE ROW LEVEL SECURITY;
ALTER TABLE contracts.sla ENABLE ROW LEVEL SECURITY;            ALTER TABLE contracts.sla FORCE ROW LEVEL SECURITY;
ALTER TABLE contracts.links ENABLE ROW LEVEL SECURITY;          ALTER TABLE contracts.links FORCE ROW LEVEL SECURITY;
ALTER TABLE contracts.events ENABLE ROW LEVEL SECURITY;         ALTER TABLE contracts.events FORCE ROW LEVEL SECURITY;
ALTER TABLE contracts.renewal_alerts ENABLE ROW LEVEL SECURITY; ALTER TABLE contracts.renewal_alerts FORCE ROW LEVEL SECURITY;

CREATE POLICY contracts_read   ON contracts.contracts FOR SELECT USING (contracts.can_see());
CREATE POLICY contracts_write  ON contracts.contracts FOR INSERT WITH CHECK (contracts.can_write());
CREATE POLICY contracts_update ON contracts.contracts FOR UPDATE USING (contracts.can_write()) WITH CHECK (contracts.can_write());
CREATE POLICY contracts_client ON contracts.contracts FOR SELECT USING (tenant_id = platform.current_tenant() AND status <> 'draft');
CREATE POLICY clauses_read   ON contracts.clauses FOR SELECT USING (contracts.can_see());
CREATE POLICY clauses_write  ON contracts.clauses FOR ALL USING (contracts.can_write()) WITH CHECK (contracts.can_write());
CREATE POLICY clauses_client ON contracts.clauses FOR SELECT USING (tenant_id = platform.current_tenant() AND EXISTS (SELECT 1 FROM contracts.contracts c WHERE c.id = contract_id AND c.status <> 'draft'));
CREATE POLICY sla_read   ON contracts.sla FOR SELECT USING (contracts.can_see());
CREATE POLICY sla_write  ON contracts.sla FOR ALL USING (contracts.can_write()) WITH CHECK (contracts.can_write());
CREATE POLICY sla_client ON contracts.sla FOR SELECT USING (tenant_id = platform.current_tenant() AND EXISTS (SELECT 1 FROM contracts.contracts c WHERE c.id = contract_id AND c.status <> 'draft'));
CREATE POLICY links_read  ON contracts.links FOR SELECT USING (contracts.can_see());
CREATE POLICY links_write ON contracts.links FOR ALL USING (contracts.can_write()) WITH CHECK (contracts.can_write());
CREATE POLICY cevents_read  ON contracts.events FOR SELECT USING (contracts.can_see());
CREATE POLICY cevents_write ON contracts.events FOR INSERT WITH CHECK (contracts.can_write() OR platform.has_role('finance'));
CREATE POLICY alerts_all ON contracts.renewal_alerts FOR ALL USING (contracts.can_see()) WITH CHECK (contracts.can_see());

GRANT SELECT, INSERT, UPDATE ON contracts.contracts TO hekate_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON contracts.clauses, contracts.sla, contracts.links TO hekate_app;
GRANT SELECT, INSERT ON contracts.events, contracts.renewal_alerts TO hekate_app;
GRANT USAGE ON SEQUENCE contracts.events_id_seq TO hekate_app;

-- ===== Client online forms =====
CREATE SCHEMA IF NOT EXISTS forms;
GRANT USAGE ON SCHEMA forms TO hekate_app;

CREATE FUNCTION forms.can_send() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT platform.has_role('ceo') OR platform.has_role('manager') OR platform.has_role('sales') OR platform.has_role('support') OR platform.has_role('engineer') $$;
GRANT EXECUTE ON FUNCTION forms.can_send TO hekate_app;

CREATE TABLE forms.requests (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL REFERENCES platform.tenants(id),
  form_code       text NOT NULL DEFAULT '',          -- e.g. FRM-14 when the form follows a pack form
  title           text NOT NULL,
  message         text NOT NULL DEFAULT '',
  fields          jsonb NOT NULL,                    -- [{key,label,type,required,options}]
  answers         jsonb NOT NULL DEFAULT '{}',
  status          text NOT NULL DEFAULT 'sent' CHECK (status IN ('sent','submitted','accepted','returned','cancelled')),
  due_date        date,
  project_id      uuid,
  contract_id     uuid,
  sent_by         text NOT NULL,
  sent_at         timestamptz NOT NULL DEFAULT now(),
  submitted_by    text,
  submitted_at    timestamptz,
  on_behalf       boolean NOT NULL DEFAULT false,    -- a paper form keyed in by staff
  reviewed_by     text,
  reviewed_at     timestamptz,
  review_note     text,
  CHECK (jsonb_typeof(fields) = 'array' AND jsonb_array_length(fields) BETWEEN 1 AND 100),
  CHECK (jsonb_typeof(answers) = 'object')
);
CREATE INDEX ON forms.requests (tenant_id, status);
CREATE TABLE forms.events (
  id         bigserial PRIMARY KEY,
  request_id uuid NOT NULL REFERENCES forms.requests(id) ON DELETE CASCADE,
  tenant_id  uuid NOT NULL REFERENCES platform.tenants(id),
  action     text NOT NULL,
  actor      text NOT NULL,
  answers    jsonb,
  note       text,
  at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON forms.events (request_id, id);
CREATE FUNCTION forms.events_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'the form history is append-only'; END $$;
CREATE TRIGGER form_events_immutable BEFORE UPDATE OR DELETE ON forms.events FOR EACH ROW EXECUTE FUNCTION forms.events_append_only();

ALTER TABLE forms.requests ENABLE ROW LEVEL SECURITY; ALTER TABLE forms.requests FORCE ROW LEVEL SECURITY;
ALTER TABLE forms.events ENABLE ROW LEVEL SECURITY;   ALTER TABLE forms.events FORCE ROW LEVEL SECURITY;
CREATE POLICY freq_staff  ON forms.requests FOR ALL USING (forms.can_send()) WITH CHECK (forms.can_send());
CREATE POLICY freq_client ON forms.requests FOR SELECT USING (tenant_id = platform.current_tenant());
CREATE POLICY freq_fill   ON forms.requests FOR UPDATE USING (tenant_id = platform.current_tenant() AND status IN ('sent','returned'))
                                                       WITH CHECK (tenant_id = platform.current_tenant());
CREATE POLICY fev_staff  ON forms.events FOR SELECT USING (forms.can_send());
CREATE POLICY fev_client ON forms.events FOR SELECT USING (tenant_id = platform.current_tenant());
CREATE POLICY fev_ins    ON forms.events FOR INSERT WITH CHECK (forms.can_send() OR tenant_id = platform.current_tenant());
GRANT SELECT, INSERT, UPDATE ON forms.requests TO hekate_app;
GRANT SELECT, INSERT ON forms.events TO hekate_app;
GRANT USAGE ON SEQUENCE forms.events_id_seq TO hekate_app;

-- ===== Project progress =====
CREATE FUNCTION projects.can_cost() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT platform.has_role('ceo') OR platform.has_role('manager') OR platform.has_role('finance') $$;
GRANT EXECUTE ON FUNCTION projects.can_cost TO hekate_app;
CREATE TABLE projects.milestones (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects.projects(id) ON DELETE CASCADE,
  tenant_id  uuid NOT NULL REFERENCES platform.tenants(id),
  title      text NOT NULL,
  due_date   date,
  status     text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','done')),
  done_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK ((status = 'done') = (done_at IS NOT NULL))
);
CREATE INDEX ON projects.milestones (project_id);
CREATE TABLE projects.costs (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  uuid NOT NULL REFERENCES projects.projects(id) ON DELETE CASCADE,
  tenant_id   uuid NOT NULL REFERENCES platform.tenants(id),
  cost_date   date NOT NULL,
  category    text NOT NULL DEFAULT 'other' CHECK (category IN ('labour','materials','travel','subcontract','other')),
  description text NOT NULL,
  amount      bigint NOT NULL CHECK (amount > 0),
  created_by  text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON projects.costs (project_id);
ALTER TABLE projects.projects ADD COLUMN quote_id uuid;
ALTER TABLE projects.projects ADD COLUMN contract_id uuid;

ALTER TABLE projects.milestones ENABLE ROW LEVEL SECURITY; ALTER TABLE projects.milestones FORCE ROW LEVEL SECURITY;
ALTER TABLE projects.costs ENABLE ROW LEVEL SECURITY;      ALTER TABLE projects.costs FORCE ROW LEVEL SECURITY;
CREATE POLICY ms_staff  ON projects.milestones FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY ms_client ON projects.milestones FOR SELECT USING (tenant_id = platform.current_tenant());
CREATE POLICY costs_all ON projects.costs FOR ALL USING (projects.can_cost()) WITH CHECK (projects.can_cost());
GRANT SELECT, INSERT, UPDATE, DELETE ON projects.milestones TO hekate_app;
GRANT SELECT, INSERT, DELETE ON projects.costs TO hekate_app;

-- ===== Employee files =====
-- Personal data sits apart from the work record so it can be shown to fewer people.
CREATE TABLE hr.employee_private (
  employee_id        uuid PRIMARY KEY REFERENCES hr.employees(id) ON DELETE CASCADE,
  date_of_birth      date,
  national_id        text NOT NULL DEFAULT '',
  cnps_number        text NOT NULL DEFAULT '',
  phone              text NOT NULL DEFAULT '',
  personal_email     text NOT NULL DEFAULT '',
  address            text NOT NULL DEFAULT '',
  emergency_name     text NOT NULL DEFAULT '',
  emergency_phone    text NOT NULL DEFAULT '',
  bank_name          text NOT NULL DEFAULT '',
  bank_account       text NOT NULL DEFAULT '',   -- where the salary is paid
  updated_by         text NOT NULL,
  updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE hr.employment_contracts (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id    uuid NOT NULL REFERENCES hr.employees(id),
  type           text NOT NULL CHECK (type IN ('CDI','CDD','internship','consultant')),
  position       text NOT NULL,
  start_date     date NOT NULL,
  end_date       date,
  probation_end  date,
  notice_days    int NOT NULL DEFAULT 30 CHECK (notice_days >= 0),
  weekly_hours   numeric(4,1) NOT NULL DEFAULT 40 CHECK (weekly_hours > 0 AND weekly_hours <= 80),
  status         text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','ended')),
  signed_on      date,
  document_ref   text,
  note           text NOT NULL DEFAULT '',
  created_by     text NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  CHECK (end_date IS NULL OR end_date >= start_date),
  CHECK (type <> 'CDD' OR end_date IS NOT NULL)
);
CREATE INDEX ON hr.employment_contracts (employee_id);
-- One live contract per employee at a time.
CREATE UNIQUE INDEX employment_one_active ON hr.employment_contracts (employee_id) WHERE status = 'active';

ALTER TABLE hr.employee_private ENABLE ROW LEVEL SECURITY;     ALTER TABLE hr.employee_private FORCE ROW LEVEL SECURITY;
ALTER TABLE hr.employment_contracts ENABLE ROW LEVEL SECURITY; ALTER TABLE hr.employment_contracts FORCE ROW LEVEL SECURITY;
CREATE POLICY priv_hr   ON hr.employee_private FOR ALL USING (hr.is_hr()) WITH CHECK (hr.is_hr());
CREATE POLICY priv_me   ON hr.employee_private FOR SELECT USING (EXISTS (SELECT 1 FROM hr.employees e WHERE e.id = employee_id AND hr.is_me(e.actor)));
CREATE POLICY empc_hr   ON hr.employment_contracts FOR ALL USING (hr.is_hr()) WITH CHECK (hr.is_hr());
CREATE POLICY empc_view ON hr.employment_contracts FOR SELECT USING (hr.is_reviewer() OR EXISTS (SELECT 1 FROM hr.employees e WHERE e.id = employee_id AND hr.is_me(e.actor)));
GRANT SELECT, INSERT, UPDATE ON hr.employee_private, hr.employment_contracts TO hekate_app;

-- ===== Company bank-account register =====
CREATE TABLE accounting.bank_accounts (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ledger_account text NOT NULL UNIQUE REFERENCES accounting.accounts(code),
  bank_name      text NOT NULL,
  account_name   text NOT NULL,
  account_number text NOT NULL,
  iban           text NOT NULL DEFAULT '',
  swift          text NOT NULL DEFAULT '',
  branch         text NOT NULL DEFAULT '',
  currency       text NOT NULL DEFAULT 'XAF',
  signatories    text NOT NULL DEFAULT '',
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','closed')),
  opened_on      date,
  closed_on      date,
  note           text NOT NULL DEFAULT '',
  created_by     text NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  CHECK ((status = 'closed') = (closed_on IS NOT NULL))
);
ALTER TABLE accounting.bank_accounts ENABLE ROW LEVEL SECURITY; ALTER TABLE accounting.bank_accounts FORCE ROW LEVEL SECURITY;
CREATE POLICY bacc_read  ON accounting.bank_accounts FOR SELECT USING (accounting.can_read());
CREATE POLICY bacc_write ON accounting.bank_accounts FOR INSERT WITH CHECK (accounting.can_write());
CREATE POLICY bacc_upd   ON accounting.bank_accounts FOR UPDATE USING (accounting.can_write()) WITH CHECK (accounting.can_write());
GRANT SELECT, INSERT, UPDATE ON accounting.bank_accounts TO hekate_app;

-- Every foreign key is indexed (the per-client row filters and joins depend on it).
CREATE INDEX ON contracts.clauses (tenant_id);
CREATE INDEX ON contracts.sla (tenant_id);
CREATE INDEX ON contracts.links (tenant_id);
CREATE INDEX ON contracts.events (tenant_id);
CREATE INDEX ON forms.events (tenant_id);
CREATE INDEX ON projects.milestones (tenant_id);
CREATE INDEX ON projects.costs (tenant_id);
