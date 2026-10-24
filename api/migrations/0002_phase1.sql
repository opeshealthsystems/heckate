-- Phase 1: clients, CRM, support extras, documents register.

CREATE SCHEMA IF NOT EXISTS clients;
CREATE SCHEMA IF NOT EXISTS crm;
CREATE SCHEMA IF NOT EXISTS documents;

CREATE FUNCTION platform.is_staff() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT coalesce(current_setting('app.role', true), '') = 'staff' $$;
CREATE FUNCTION platform.current_tenant() RETURNS uuid LANGUAGE sql STABLE AS
  $$ SELECT nullif(current_setting('app.tenant_id', true), '')::uuid $$;

-- tenants: staff see all, everyone else only their own
ALTER TABLE platform.tenants ADD COLUMN kind text NOT NULL DEFAULT 'client' CHECK (kind IN ('hekate','client'));
ALTER TABLE platform.tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform.tenants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenants_read ON platform.tenants FOR SELECT USING (platform.is_staff() OR id = platform.current_tenant());
CREATE POLICY tenants_write ON platform.tenants FOR INSERT WITH CHECK (platform.is_staff());
GRANT SELECT, INSERT ON platform.tenants TO hekate_app;
GRANT SELECT, UPDATE ON platform.outbox TO hekate_app;

CREATE TABLE platform.idempotency_keys (
  key          text NOT NULL,
  actor        text NOT NULL,
  request_hash text NOT NULL,
  status       int  NOT NULL,
  body         jsonb,
  created_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (key, actor)
);
GRANT SELECT, INSERT ON platform.idempotency_keys TO hekate_app;

-- clients
CREATE TABLE clients.accounts (
  tenant_id  uuid PRIMARY KEY REFERENCES platform.tenants(id),
  legal_name text NOT NULL,
  rccm       text,
  email      text,
  phone      text,
  kyc_status text NOT NULL DEFAULT 'pending' CHECK (kyc_status IN ('pending','verified','rejected')),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE clients.contacts (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  uuid NOT NULL REFERENCES platform.tenants(id),
  name       text NOT NULL,
  email      text NOT NULL,
  phone      text,
  is_admin   boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE clients.accounts ENABLE ROW LEVEL SECURITY; ALTER TABLE clients.accounts FORCE ROW LEVEL SECURITY;
ALTER TABLE clients.contacts ENABLE ROW LEVEL SECURITY; ALTER TABLE clients.contacts FORCE ROW LEVEL SECURITY;
CREATE POLICY accounts_read  ON clients.accounts FOR SELECT USING (platform.is_staff() OR tenant_id = platform.current_tenant());
CREATE POLICY accounts_write ON clients.accounts FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY contacts_read  ON clients.contacts FOR SELECT USING (platform.is_staff() OR tenant_id = platform.current_tenant());
CREATE POLICY contacts_write ON clients.contacts FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());

-- crm (staff only; public enquiries enter through the security-definer function below)
CREATE TABLE crm.leads (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  email      text NOT NULL,
  phone      text,
  company    text,
  message    text NOT NULL DEFAULT '',
  source     text NOT NULL DEFAULT 'website',
  stage      text NOT NULL DEFAULT 'new' CHECK (stage IN ('new','contacted','qualified','proposal','won','lost')),
  owner      text,
  created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE crm.leads ENABLE ROW LEVEL SECURITY; ALTER TABLE crm.leads FORCE ROW LEVEL SECURITY;
CREATE POLICY leads_staff ON crm.leads FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());

CREATE FUNCTION crm.submit_enquiry(p_name text, p_email text, p_phone text, p_company text, p_message text)
RETURNS uuid LANGUAGE sql SECURITY DEFINER SET search_path = crm, pg_temp AS $$
  INSERT INTO crm.leads (name, email, phone, company, message, source)
  VALUES (p_name, p_email, p_phone, p_company, p_message, 'website') RETURNING id
$$;
REVOKE ALL ON FUNCTION crm.submit_enquiry FROM PUBLIC;

-- support extras
CREATE TABLE support.sla_policies (
  severity        text PRIMARY KEY CHECK (severity IN ('P1','P2','P3','P4')),
  respond_minutes int NOT NULL,
  resolve_minutes int NOT NULL
);
-- Placeholder targets until the SLA levels are decided (open decision in the spec).
INSERT INTO support.sla_policies VALUES ('P1',15,240),('P2',60,480),('P3',240,1440),('P4',480,4320);

ALTER TABLE support.tickets
  ADD COLUMN reporter          text NOT NULL DEFAULT '',
  ADD COLUMN assignee          text,
  ADD COLUMN version           int  NOT NULL DEFAULT 1,
  ADD COLUMN respond_due_at    timestamptz,
  ADD COLUMN first_response_at timestamptz,
  ADD COLUMN resolved_at       timestamptz,
  ADD COLUMN rating            int CHECK (rating BETWEEN 1 AND 5),
  ADD COLUMN rating_comment    text;

CREATE TABLE support.ticket_comments (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ticket_id  uuid NOT NULL REFERENCES support.tickets(id),
  tenant_id  uuid NOT NULL REFERENCES platform.tenants(id),
  author     text NOT NULL,
  body       text NOT NULL,
  internal   boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON support.ticket_comments (ticket_id, created_at);
ALTER TABLE support.ticket_comments ENABLE ROW LEVEL SECURITY; ALTER TABLE support.ticket_comments FORCE ROW LEVEL SECURITY;
CREATE POLICY comments_read ON support.ticket_comments FOR SELECT
  USING (platform.is_staff() OR (tenant_id = platform.current_tenant() AND NOT internal));
CREATE POLICY comments_write ON support.ticket_comments FOR INSERT
  WITH CHECK (platform.is_staff() OR (tenant_id = platform.current_tenant() AND NOT internal));
CREATE INDEX ON support.tickets (tenant_id, created_at DESC, id DESC);

GRANT SELECT ON support.sla_policies TO hekate_app;
GRANT SELECT, INSERT ON support.ticket_comments TO hekate_app;
GRANT SELECT, INSERT, UPDATE ON clients.accounts, clients.contacts TO hekate_app;
GRANT SELECT, INSERT, UPDATE ON crm.leads TO hekate_app;
GRANT USAGE ON SCHEMA clients, crm, documents TO hekate_app;
GRANT EXECUTE ON FUNCTION crm.submit_enquiry TO hekate_app;
GRANT EXECUTE ON FUNCTION platform.is_staff, platform.current_tenant TO hekate_app;

-- documents register
CREATE TABLE documents.counters (
  code text NOT NULL, year int NOT NULL, last int NOT NULL DEFAULT 0,
  PRIMARY KEY (code, year)
);
CREATE TABLE documents.register (
  ref       text PRIMARY KEY,
  code      text NOT NULL,
  title     text NOT NULL,
  tenant_id uuid REFERENCES platform.tenants(id),
  status    text NOT NULL DEFAULT 'valid' CHECK (status IN ('valid','revoked')),
  sha256    text,
  issued_by text NOT NULL,
  issued_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE documents.register ENABLE ROW LEVEL SECURITY; ALTER TABLE documents.register FORCE ROW LEVEL SECURITY;
CREATE POLICY register_read  ON documents.register FOR SELECT USING (platform.is_staff() OR tenant_id = platform.current_tenant());
CREATE POLICY register_write ON documents.register FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
GRANT SELECT, INSERT, UPDATE ON documents.register TO hekate_app;
GRANT SELECT, INSERT, UPDATE ON documents.counters TO hekate_app;

-- Public verification: returns only non-sensitive fields, no tenant access needed.
CREATE FUNCTION documents.verify(p_ref text)
RETURNS TABLE (ref text, code text, title text, status text, issued_at timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = documents, pg_temp AS $$
  SELECT r.ref, r.code, r.title, r.status, r.issued_at FROM documents.register r WHERE r.ref = p_ref
$$;
REVOKE ALL ON FUNCTION documents.verify FROM PUBLIC;
GRANT EXECUTE ON FUNCTION documents.verify TO hekate_app;
