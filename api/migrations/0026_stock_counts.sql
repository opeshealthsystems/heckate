-- Stock counts: someone counts a warehouse, a different person approves, and only then do the
-- differences become adjustment movements. The expected quantity is a snapshot taken at count time.
CREATE TABLE inventory.counts (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  warehouse_id uuid NOT NULL REFERENCES inventory.warehouses(id),
  status       text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','approved')),
  counted_by   text NOT NULL,
  approved_by  text,
  created_at   timestamptz NOT NULL DEFAULT now(),
  approved_at  timestamptz,
  CHECK ((status = 'approved') = (approved_by IS NOT NULL))
);
CREATE TABLE inventory.count_lines (
  count_id uuid NOT NULL REFERENCES inventory.counts(id) ON DELETE CASCADE,
  item_id  uuid NOT NULL REFERENCES inventory.items(id),
  expected int NOT NULL CHECK (expected >= 0),
  counted  int NOT NULL CHECK (counted >= 0),
  PRIMARY KEY (count_id, item_id)
);

-- The counter cannot approve their own count, and an approved count never changes.
CREATE FUNCTION inventory.counts_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    IF OLD.status = 'approved' THEN RAISE EXCEPTION 'an approved stock count cannot be deleted'; END IF;
    RETURN OLD;
  END IF;
  IF OLD.status = 'approved' THEN RAISE EXCEPTION 'an approved stock count cannot change'; END IF;
  IF NEW.warehouse_id <> OLD.warehouse_id OR NEW.counted_by <> OLD.counted_by THEN RAISE EXCEPTION 'the warehouse and counter cannot change'; END IF;
  IF NEW.status = 'approved' AND NEW.approved_by = OLD.counted_by THEN RAISE EXCEPTION 'a stock count must be approved by someone other than the counter'; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER counts_guard BEFORE UPDATE OR DELETE ON inventory.counts FOR EACH ROW EXECUTE FUNCTION inventory.counts_guard();

CREATE FUNCTION inventory.count_lines_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM inventory.counts WHERE id = COALESCE(NEW.count_id, OLD.count_id) AND status = 'approved') THEN
    RAISE EXCEPTION 'the lines of an approved stock count cannot change';
  END IF;
  RETURN COALESCE(NEW, OLD);
END $$;
CREATE TRIGGER count_lines_guard BEFORE UPDATE OR DELETE ON inventory.count_lines FOR EACH ROW EXECUTE FUNCTION inventory.count_lines_guard();

ALTER TABLE inventory.counts ENABLE ROW LEVEL SECURITY;      ALTER TABLE inventory.counts FORCE ROW LEVEL SECURITY;
ALTER TABLE inventory.count_lines ENABLE ROW LEVEL SECURITY; ALTER TABLE inventory.count_lines FORCE ROW LEVEL SECURITY;
CREATE POLICY counts_staff ON inventory.counts FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY count_lines_staff ON inventory.count_lines FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
GRANT SELECT, INSERT, UPDATE, DELETE ON inventory.counts, inventory.count_lines TO hekate_app;
