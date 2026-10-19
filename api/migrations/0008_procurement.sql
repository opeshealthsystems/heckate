-- Purchasing & Suppliers: supplier register, RFQs, purchase orders, receipts, supplier invoices.
CREATE SCHEMA IF NOT EXISTS procurement;

ALTER TABLE platform.tenants DROP CONSTRAINT tenants_kind_check;
ALTER TABLE platform.tenants ADD CONSTRAINT tenants_kind_check CHECK (kind IN ('hekate','client','supplier'));

CREATE TABLE procurement.suppliers (
  tenant_id        uuid PRIMARY KEY REFERENCES platform.tenants(id),
  legal_name       text NOT NULL,
  rccm             text,
  email            text,
  phone            text,
  bank_account     text,
  bank_status      text NOT NULL DEFAULT 'none' CHECK (bank_status IN ('none','pending_callback','verified')),
  bank_verified_by text,
  bank_note        text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  CHECK (bank_status = 'none' OR bank_account IS NOT NULL)
);
CREATE TABLE procurement.evaluations (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  supplier   uuid NOT NULL REFERENCES procurement.suppliers(tenant_id),
  evaluator  text NOT NULL,
  quality    int NOT NULL CHECK (quality BETWEEN 1 AND 5),
  delivery   int NOT NULL CHECK (delivery BETWEEN 1 AND 5),
  price      int NOT NULL CHECK (price BETWEEN 1 AND 5),
  comment    text NOT NULL DEFAULT '',
  at         timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE procurement.rfqs (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ref        text UNIQUE,
  title      text NOT NULL,
  status     text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','open','closed','awarded')),
  closes_at  timestamptz,
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE procurement.rfq_lines (
  rfq_id      uuid NOT NULL REFERENCES procurement.rfqs(id) ON DELETE CASCADE,
  position    int NOT NULL,
  description text NOT NULL,
  quantity    numeric(12,2) NOT NULL CHECK (quantity > 0),
  PRIMARY KEY (rfq_id, position)
);
CREATE TABLE procurement.rfq_invites (
  rfq_id   uuid NOT NULL REFERENCES procurement.rfqs(id) ON DELETE CASCADE,
  supplier uuid NOT NULL REFERENCES procurement.suppliers(tenant_id),
  PRIMARY KEY (rfq_id, supplier)
);
CREATE TABLE procurement.rfq_responses (
  rfq_id       uuid NOT NULL REFERENCES procurement.rfqs(id) ON DELETE CASCADE,
  supplier     uuid NOT NULL REFERENCES procurement.suppliers(tenant_id),
  unit_prices  jsonb NOT NULL,          -- [{position, unit_price}]
  total        bigint NOT NULL CHECK (total >= 0),
  note         text NOT NULL DEFAULT '',
  submitted_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (rfq_id, supplier)
);

CREATE TABLE procurement.purchase_orders (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ref          text UNIQUE,
  supplier     uuid NOT NULL REFERENCES procurement.suppliers(tenant_id),
  rfq_id       uuid REFERENCES procurement.rfqs(id),
  title        text NOT NULL,
  status       text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','pending_approval','issued','received','closed')),
  net          bigint NOT NULL DEFAULT 0,
  created_by   text NOT NULL,
  requested_by text,
  approved_by  text,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE procurement.po_lines (
  po_id        uuid NOT NULL REFERENCES procurement.purchase_orders(id) ON DELETE CASCADE,
  position     int NOT NULL,
  description  text NOT NULL,
  quantity     numeric(12,2) NOT NULL CHECK (quantity > 0),
  unit_price   bigint NOT NULL CHECK (unit_price >= 0),
  item_id      uuid,                                       -- stock item received into inventory
  received_qty numeric(12,2) NOT NULL DEFAULT 0 CHECK (received_qty >= 0),
  PRIMARY KEY (po_id, position),
  CHECK (received_qty <= quantity)
);
CREATE TABLE procurement.supplier_invoices (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  po_id        uuid NOT NULL REFERENCES procurement.purchase_orders(id),
  supplier     uuid NOT NULL REFERENCES procurement.suppliers(tenant_id),
  number       text NOT NULL,
  net          bigint NOT NULL CHECK (net > 0),
  vat          bigint NOT NULL CHECK (vat >= 0),
  total        bigint NOT NULL,
  status       text NOT NULL DEFAULT 'submitted' CHECK (status IN ('submitted','approved','rejected','paid')),
  approved_by  text,
  paid_by      text,
  submitted_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (supplier, number),
  CHECK (total = net + vat)
);

-- Issued purchase orders are commitments: terms are frozen once issued.
CREATE FUNCTION procurement.guard_po() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status IN ('issued','received','closed') AND (NEW.net <> OLD.net OR NEW.supplier <> OLD.supplier OR NEW.ref IS DISTINCT FROM OLD.ref OR NEW.title <> OLD.title) THEN
    RAISE EXCEPTION 'issued purchase orders are immutable';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER po_guard BEFORE UPDATE ON procurement.purchase_orders FOR EACH ROW EXECUTE FUNCTION procurement.guard_po();

CREATE FUNCTION procurement.guard_po_lines() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE st text;
BEGIN
  SELECT status INTO st FROM procurement.purchase_orders WHERE id = COALESCE(NEW.po_id, OLD.po_id);
  IF st IN ('issued','received','closed') THEN
    IF TG_OP = 'DELETE' OR TG_OP = 'INSERT' THEN RAISE EXCEPTION 'issued purchase order lines are immutable'; END IF;
    IF NEW.description <> OLD.description OR NEW.quantity <> OLD.quantity OR NEW.unit_price <> OLD.unit_price THEN
      RAISE EXCEPTION 'issued purchase order lines are immutable';
    END IF;
  END IF;
  RETURN COALESCE(NEW, OLD);
END $$;
CREATE TRIGGER po_lines_guard BEFORE INSERT OR UPDATE OR DELETE ON procurement.po_lines FOR EACH ROW EXECUTE FUNCTION procurement.guard_po_lines();

-- Role helper for procurement staff.
CREATE FUNCTION procurement.is_buyer() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT platform.has_role('procurement') OR platform.has_role('manager') OR platform.has_role('ceo') $$;
CREATE FUNCTION procurement.can_pay() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT platform.has_role('finance') OR platform.has_role('manager') OR platform.has_role('ceo') $$;
GRANT EXECUTE ON FUNCTION procurement.is_buyer, procurement.can_pay TO hekate_app;

ALTER TABLE procurement.suppliers ENABLE ROW LEVEL SECURITY;         ALTER TABLE procurement.suppliers FORCE ROW LEVEL SECURITY;
ALTER TABLE procurement.evaluations ENABLE ROW LEVEL SECURITY;       ALTER TABLE procurement.evaluations FORCE ROW LEVEL SECURITY;
ALTER TABLE procurement.rfqs ENABLE ROW LEVEL SECURITY;              ALTER TABLE procurement.rfqs FORCE ROW LEVEL SECURITY;
ALTER TABLE procurement.rfq_lines ENABLE ROW LEVEL SECURITY;         ALTER TABLE procurement.rfq_lines FORCE ROW LEVEL SECURITY;
ALTER TABLE procurement.rfq_invites ENABLE ROW LEVEL SECURITY;       ALTER TABLE procurement.rfq_invites FORCE ROW LEVEL SECURITY;
ALTER TABLE procurement.rfq_responses ENABLE ROW LEVEL SECURITY;     ALTER TABLE procurement.rfq_responses FORCE ROW LEVEL SECURITY;
ALTER TABLE procurement.purchase_orders ENABLE ROW LEVEL SECURITY;   ALTER TABLE procurement.purchase_orders FORCE ROW LEVEL SECURITY;
ALTER TABLE procurement.po_lines ENABLE ROW LEVEL SECURITY;          ALTER TABLE procurement.po_lines FORCE ROW LEVEL SECURITY;
ALTER TABLE procurement.supplier_invoices ENABLE ROW LEVEL SECURITY; ALTER TABLE procurement.supplier_invoices FORCE ROW LEVEL SECURITY;

-- Staff with a buying or paying role see all; a supplier sees only their own records.
CREATE FUNCTION procurement.staff_read() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT procurement.is_buyer() OR procurement.can_pay() $$;
GRANT EXECUTE ON FUNCTION procurement.staff_read TO hekate_app;

CREATE POLICY sup_staff ON procurement.suppliers FOR ALL USING (procurement.staff_read()) WITH CHECK (procurement.is_buyer());
-- Finance verifies bank details by call-back; the guard_bank trigger limits what that can change.
CREATE POLICY sup_finance_update ON procurement.suppliers FOR UPDATE USING (procurement.can_pay()) WITH CHECK (procurement.can_pay());
CREATE POLICY sup_self  ON procurement.suppliers FOR SELECT USING (tenant_id = platform.current_tenant());
CREATE POLICY sup_self_bank ON procurement.suppliers FOR UPDATE USING (tenant_id = platform.current_tenant()) WITH CHECK (tenant_id = platform.current_tenant());
CREATE POLICY eval_staff ON procurement.evaluations FOR ALL USING (procurement.is_buyer()) WITH CHECK (procurement.is_buyer());

CREATE POLICY rfq_staff ON procurement.rfqs FOR ALL USING (procurement.is_buyer()) WITH CHECK (procurement.is_buyer());
CREATE POLICY rfq_supplier ON procurement.rfqs FOR SELECT USING (status = 'open' AND EXISTS (SELECT 1 FROM procurement.rfq_invites i WHERE i.rfq_id = procurement.rfqs.id AND i.supplier = platform.current_tenant()));
CREATE POLICY rfql_staff ON procurement.rfq_lines FOR ALL USING (procurement.is_buyer()) WITH CHECK (procurement.is_buyer());
CREATE POLICY rfql_supplier ON procurement.rfq_lines FOR SELECT USING (EXISTS (SELECT 1 FROM procurement.rfq_invites i JOIN procurement.rfqs r ON r.id = i.rfq_id WHERE i.rfq_id = procurement.rfq_lines.rfq_id AND i.supplier = platform.current_tenant() AND r.status = 'open'));
CREATE POLICY rfqi_staff ON procurement.rfq_invites FOR ALL USING (procurement.is_buyer()) WITH CHECK (procurement.is_buyer());
CREATE POLICY rfqi_supplier ON procurement.rfq_invites FOR SELECT USING (supplier = platform.current_tenant());
CREATE POLICY rfqr_staff ON procurement.rfq_responses FOR SELECT USING (procurement.is_buyer());
CREATE POLICY rfqr_supplier ON procurement.rfq_responses FOR ALL USING (supplier = platform.current_tenant()) WITH CHECK (supplier = platform.current_tenant());

CREATE POLICY po_staff ON procurement.purchase_orders FOR ALL USING (procurement.staff_read()) WITH CHECK (procurement.is_buyer());
CREATE POLICY po_supplier ON procurement.purchase_orders FOR SELECT USING (supplier = platform.current_tenant() AND status IN ('issued','received','closed'));
CREATE POLICY pol_staff ON procurement.po_lines FOR ALL USING (procurement.staff_read()) WITH CHECK (procurement.is_buyer());
CREATE POLICY pol_supplier ON procurement.po_lines FOR SELECT USING (EXISTS (SELECT 1 FROM procurement.purchase_orders p WHERE p.id = procurement.po_lines.po_id AND p.supplier = platform.current_tenant() AND p.status IN ('issued','received','closed')));
CREATE POLICY si_staff ON procurement.supplier_invoices FOR ALL USING (procurement.staff_read()) WITH CHECK (procurement.can_pay() OR procurement.is_buyer());
CREATE POLICY si_supplier ON procurement.supplier_invoices FOR SELECT USING (supplier = platform.current_tenant());
CREATE POLICY si_supplier_ins ON procurement.supplier_invoices FOR INSERT WITH CHECK (supplier = platform.current_tenant() AND status = 'submitted');

GRANT USAGE ON SCHEMA procurement TO hekate_app;
GRANT SELECT, INSERT, UPDATE ON procurement.suppliers, procurement.rfqs, procurement.purchase_orders, procurement.supplier_invoices TO hekate_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON procurement.rfq_lines, procurement.rfq_invites, procurement.rfq_responses, procurement.po_lines TO hekate_app;
GRANT SELECT, INSERT ON procurement.evaluations TO hekate_app;
GRANT DELETE ON procurement.purchase_orders TO hekate_app;

-- Bank details are the classic fraud target. RLS cannot restrict columns, so a trigger does:
-- only finance or management can verify, and any change to the account resets verification.
CREATE FUNCTION procurement.guard_bank() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.bank_account IS DISTINCT FROM OLD.bank_account AND NEW.bank_status = 'verified' THEN
    RAISE EXCEPTION 'changed bank details must be re-verified by call-back';
  END IF;
  IF NEW.bank_status = 'verified' AND OLD.bank_status <> 'verified' AND NOT procurement.can_pay() THEN
    RAISE EXCEPTION 'only finance or management can verify bank details';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER supplier_bank_guard BEFORE UPDATE ON procurement.suppliers FOR EACH ROW EXECUTE FUNCTION procurement.guard_bank();

CREATE TABLE procurement.settings (
  id                 boolean PRIMARY KEY DEFAULT true CHECK (id),
  approval_threshold bigint NOT NULL
);
-- Placeholder: purchase orders at or above this net amount need a second approver (docs/BLOCKERS.md).
INSERT INTO procurement.settings VALUES (true, 5000000);
GRANT SELECT ON procurement.settings TO hekate_app;
