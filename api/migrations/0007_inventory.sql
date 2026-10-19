-- Inventory & Assets: stock ledger, serial-numbered equipment, issue to staff, client assets under contract.
CREATE SCHEMA IF NOT EXISTS inventory;

CREATE TABLE inventory.items (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  sku        text NOT NULL UNIQUE,
  name       text NOT NULL,
  unit       text NOT NULL DEFAULT 'pcs',
  serialised boolean NOT NULL DEFAULT false,
  min_stock  int NOT NULL DEFAULT 0 CHECK (min_stock >= 0),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE inventory.warehouses (
  id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL UNIQUE
);
-- Stock is the sum of an append-only movement ledger, never an editable counter.
CREATE TABLE inventory.moves (
  id           bigserial PRIMARY KEY,
  item_id      uuid NOT NULL REFERENCES inventory.items(id),
  warehouse_id uuid NOT NULL REFERENCES inventory.warehouses(id),
  qty          int  NOT NULL CHECK (qty <> 0),
  kind         text NOT NULL CHECK (kind IN ('receipt','issue','transfer_in','transfer_out','adjustment')),
  ref          text NOT NULL DEFAULT '',
  serial       text,
  actor        text NOT NULL,
  at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON inventory.moves (item_id, warehouse_id);

CREATE TABLE inventory.serials (
  id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  item_id   uuid NOT NULL REFERENCES inventory.items(id),
  serial    text NOT NULL,
  status    text NOT NULL DEFAULT 'in_stock' CHECK (status IN ('in_stock','issued','client_asset','retired')),
  holder    text,                                  -- staff member when issued
  tenant_id uuid REFERENCES platform.tenants(id),  -- client when installed under contract
  warehouse_id uuid REFERENCES inventory.warehouses(id),
  UNIQUE (item_id, serial),
  CHECK ((status = 'issued') = (holder IS NOT NULL)),
  CHECK ((status = 'client_asset') = (tenant_id IS NOT NULL))
);
CREATE TABLE inventory.assignments (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  serial_id   uuid NOT NULL REFERENCES inventory.serials(id),
  assignee    text NOT NULL,
  issued_by   text NOT NULL,
  issued_at   timestamptz NOT NULL DEFAULT now(),
  document_ref text,
  returned_at timestamptz
);

-- No negative stock, checked at commit so multi-line operations work.
CREATE FUNCTION inventory.check_stock() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE have bigint;
BEGIN
  SELECT coalesce(sum(qty),0) INTO have FROM inventory.moves WHERE item_id = NEW.item_id AND warehouse_id = NEW.warehouse_id;
  IF have < 0 THEN RAISE EXCEPTION 'insufficient stock'; END IF;
  RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER moves_no_negative AFTER INSERT ON inventory.moves DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION inventory.check_stock();

CREATE FUNCTION inventory.moves_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'stock movements are append-only'; END $$;
CREATE TRIGGER moves_immutable BEFORE UPDATE OR DELETE ON inventory.moves FOR EACH ROW EXECUTE FUNCTION inventory.moves_append_only();

ALTER TABLE inventory.items ENABLE ROW LEVEL SECURITY;       ALTER TABLE inventory.items FORCE ROW LEVEL SECURITY;
ALTER TABLE inventory.warehouses ENABLE ROW LEVEL SECURITY;  ALTER TABLE inventory.warehouses FORCE ROW LEVEL SECURITY;
ALTER TABLE inventory.moves ENABLE ROW LEVEL SECURITY;       ALTER TABLE inventory.moves FORCE ROW LEVEL SECURITY;
ALTER TABLE inventory.serials ENABLE ROW LEVEL SECURITY;     ALTER TABLE inventory.serials FORCE ROW LEVEL SECURITY;
ALTER TABLE inventory.assignments ENABLE ROW LEVEL SECURITY; ALTER TABLE inventory.assignments FORCE ROW LEVEL SECURITY;

CREATE POLICY items_staff ON inventory.items FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY wh_staff    ON inventory.warehouses FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY moves_staff ON inventory.moves FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY serials_staff ON inventory.serials FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY serials_client ON inventory.serials FOR SELECT USING (status = 'client_asset' AND tenant_id = platform.current_tenant());
CREATE POLICY items_client ON inventory.items FOR SELECT USING (EXISTS (SELECT 1 FROM inventory.serials s WHERE s.item_id = inventory.items.id AND s.status='client_asset' AND s.tenant_id = platform.current_tenant()));
CREATE POLICY assign_staff ON inventory.assignments FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());

GRANT USAGE ON SCHEMA inventory TO hekate_app;
GRANT SELECT, INSERT, UPDATE ON inventory.items, inventory.warehouses, inventory.serials, inventory.assignments TO hekate_app;
GRANT SELECT, INSERT ON inventory.moves TO hekate_app;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA inventory TO hekate_app;

ALTER TABLE projects.interventions ADD COLUMN parts jsonb NOT NULL DEFAULT '[]'::jsonb;
