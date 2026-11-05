-- Sales & Invoicing. All money is integer XAF (the franc has no minor unit).
CREATE SCHEMA IF NOT EXISTS sales;

CREATE TABLE sales.settings (
  id                 boolean PRIMARY KEY DEFAULT true CHECK (id),  -- single row
  tva_bp             int NOT NULL,   -- TVA in basis points
  cac_pct_of_tva     int NOT NULL,   -- CAC as a percentage of the TVA amount
  approval_threshold bigint NOT NULL -- invoices at or above this total need a second approver
);
-- Placeholders to be confirmed by the accountant (docs/BLOCKERS.md): TVA 17.5% + CAC 10% of TVA = 19.25%.
INSERT INTO sales.settings VALUES (true, 1750, 10, 5000000);
GRANT SELECT ON sales.settings TO hekate_app;

CREATE TABLE sales.quotes (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES platform.tenants(id),
  ref         text UNIQUE,
  title       text NOT NULL,
  status      text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','sent','approved','rejected')),
  valid_until date,
  notes       text NOT NULL DEFAULT '',
  created_by  text NOT NULL,
  decided_by  text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sales.quote_lines (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  quote_id    uuid NOT NULL REFERENCES sales.quotes(id) ON DELETE CASCADE,
  tenant_id   uuid NOT NULL REFERENCES platform.tenants(id),
  position    int NOT NULL,
  description text NOT NULL,
  quantity    numeric(12,2) NOT NULL CHECK (quantity > 0),
  unit_price  bigint NOT NULL CHECK (unit_price >= 0)
);

CREATE TABLE sales.invoices (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid NOT NULL REFERENCES platform.tenants(id),
  quote_id      uuid REFERENCES sales.quotes(id),
  ref           text UNIQUE,
  title         text NOT NULL,
  status        text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','pending_approval','issued','paid')),
  issue_date    date,
  due_date      date,
  net           bigint NOT NULL DEFAULT 0,
  tva           bigint NOT NULL DEFAULT 0,
  cac           bigint NOT NULL DEFAULT 0,
  total         bigint NOT NULL DEFAULT 0,
  created_by    text NOT NULL,
  requested_by  text,
  approved_by   text,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sales.invoice_lines (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  invoice_id  uuid NOT NULL REFERENCES sales.invoices(id) ON DELETE CASCADE,
  tenant_id   uuid NOT NULL REFERENCES platform.tenants(id),
  position    int NOT NULL,
  description text NOT NULL,
  quantity    numeric(12,2) NOT NULL CHECK (quantity > 0),
  unit_price  bigint NOT NULL CHECK (unit_price >= 0)
);
CREATE TABLE sales.payments (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  invoice_id  uuid NOT NULL REFERENCES sales.invoices(id),
  tenant_id   uuid NOT NULL REFERENCES platform.tenants(id),
  amount      bigint NOT NULL CHECK (amount > 0),
  method      text NOT NULL CHECK (method IN ('momo','orange_money','bank','card','cash')),
  reference   text NOT NULL DEFAULT '',
  paid_at     timestamptz NOT NULL DEFAULT now(),
  recorded_by text NOT NULL
);
CREATE TABLE sales.credit_notes (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  invoice_id  uuid NOT NULL REFERENCES sales.invoices(id),
  tenant_id   uuid NOT NULL REFERENCES platform.tenants(id),
  ref         text UNIQUE NOT NULL,
  reason      text NOT NULL,
  net         bigint NOT NULL CHECK (net > 0),
  tva         bigint NOT NULL,
  cac         bigint NOT NULL,
  total       bigint NOT NULL,
  created_by  text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ON sales.quotes (tenant_id, created_at DESC);
CREATE INDEX ON sales.invoices (tenant_id, created_at DESC);
CREATE INDEX ON sales.payments (invoice_id);

-- Issued invoices are legal records: figures, reference and lines never change; corrections use credit notes.
CREATE FUNCTION sales.guard_invoice() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    IF OLD.status <> 'draft' THEN RAISE EXCEPTION 'issued invoices cannot be deleted'; END IF;
    RETURN OLD;
  END IF;
  IF OLD.status IN ('issued','paid') THEN
    IF NEW.net <> OLD.net OR NEW.tva <> OLD.tva OR NEW.cac <> OLD.cac OR NEW.total <> OLD.total
       OR NEW.ref IS DISTINCT FROM OLD.ref OR NEW.tenant_id <> OLD.tenant_id OR NEW.issue_date IS DISTINCT FROM OLD.issue_date
       OR NEW.due_date IS DISTINCT FROM OLD.due_date OR NEW.title <> OLD.title THEN
      RAISE EXCEPTION 'issued invoices are immutable';
    END IF;
    IF NEW.status NOT IN ('issued','paid') THEN RAISE EXCEPTION 'issued invoices cannot return to draft'; END IF;
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER invoice_guard BEFORE UPDATE OR DELETE ON sales.invoices FOR EACH ROW EXECUTE FUNCTION sales.guard_invoice();

CREATE FUNCTION sales.guard_invoice_lines() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE st text;
BEGIN
  SELECT status INTO st FROM sales.invoices WHERE id = COALESCE(NEW.invoice_id, OLD.invoice_id);
  IF st IS NOT NULL AND st NOT IN ('draft','pending_approval') AND TG_OP <> 'DELETE' THEN RAISE EXCEPTION 'issued invoice lines are immutable'; END IF;
  IF st IS NOT NULL AND st NOT IN ('draft','pending_approval') AND TG_OP = 'DELETE' THEN RAISE EXCEPTION 'issued invoice lines are immutable'; END IF;
  RETURN COALESCE(NEW, OLD);
END $$;
CREATE TRIGGER invoice_lines_guard BEFORE INSERT OR UPDATE OR DELETE ON sales.invoice_lines FOR EACH ROW EXECUTE FUNCTION sales.guard_invoice_lines();

-- Row-level security: staff see everything; clients only their own tenant and only what was sent to them.
ALTER TABLE sales.quotes ENABLE ROW LEVEL SECURITY;         ALTER TABLE sales.quotes FORCE ROW LEVEL SECURITY;
ALTER TABLE sales.quote_lines ENABLE ROW LEVEL SECURITY;    ALTER TABLE sales.quote_lines FORCE ROW LEVEL SECURITY;
ALTER TABLE sales.invoices ENABLE ROW LEVEL SECURITY;       ALTER TABLE sales.invoices FORCE ROW LEVEL SECURITY;
ALTER TABLE sales.invoice_lines ENABLE ROW LEVEL SECURITY;  ALTER TABLE sales.invoice_lines FORCE ROW LEVEL SECURITY;
ALTER TABLE sales.payments ENABLE ROW LEVEL SECURITY;       ALTER TABLE sales.payments FORCE ROW LEVEL SECURITY;
ALTER TABLE sales.credit_notes ENABLE ROW LEVEL SECURITY;   ALTER TABLE sales.credit_notes FORCE ROW LEVEL SECURITY;

CREATE POLICY quotes_staff ON sales.quotes FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY quotes_client_read ON sales.quotes FOR SELECT USING (tenant_id = platform.current_tenant() AND status <> 'draft');
CREATE POLICY quotes_client_decide ON sales.quotes FOR UPDATE
  USING (tenant_id = platform.current_tenant() AND status = 'sent')
  WITH CHECK (tenant_id = platform.current_tenant() AND status IN ('approved','rejected'));
CREATE POLICY qlines_staff ON sales.quote_lines FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY qlines_client_read ON sales.quote_lines FOR SELECT USING (
  tenant_id = platform.current_tenant() AND EXISTS (SELECT 1 FROM sales.quotes q WHERE q.id = quote_id AND q.status <> 'draft'));

CREATE POLICY invoices_staff ON sales.invoices FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY invoices_client_read ON sales.invoices FOR SELECT USING (tenant_id = platform.current_tenant() AND status IN ('issued','paid'));
CREATE POLICY ilines_staff ON sales.invoice_lines FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY ilines_client_read ON sales.invoice_lines FOR SELECT USING (
  tenant_id = platform.current_tenant() AND EXISTS (SELECT 1 FROM sales.invoices i WHERE i.id = invoice_id AND i.status IN ('issued','paid')));
CREATE POLICY payments_staff ON sales.payments FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY payments_client_read ON sales.payments FOR SELECT USING (tenant_id = platform.current_tenant());
CREATE POLICY credit_staff ON sales.credit_notes FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY credit_client_read ON sales.credit_notes FOR SELECT USING (tenant_id = platform.current_tenant());

GRANT USAGE ON SCHEMA sales TO hekate_app;
GRANT SELECT, INSERT, UPDATE ON sales.quotes, sales.invoices TO hekate_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON sales.quote_lines, sales.invoice_lines TO hekate_app;
GRANT SELECT, INSERT ON sales.payments, sales.credit_notes TO hekate_app;
GRANT DELETE ON sales.invoices TO hekate_app;
