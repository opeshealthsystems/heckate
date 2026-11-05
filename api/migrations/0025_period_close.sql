-- Closing a month locks it: no entry can be dated inside a closed month, whoever posts and however
-- (the rule is a trigger, so raw SQL is stopped too). A close is permanent; a correction goes into
-- the first open period with a narration that refers to the original.
CREATE TABLE accounting.closed_periods (
  period    date PRIMARY KEY CHECK (period = date_trunc('month', period)::date),
  closed_by text NOT NULL,
  closed_at timestamptz NOT NULL DEFAULT now(),
  note      text NOT NULL DEFAULT ''
);
CREATE TRIGGER closed_periods_append_only BEFORE UPDATE OR DELETE ON accounting.closed_periods FOR EACH ROW EXECUTE FUNCTION accounting.forbid_change();
CREATE TRIGGER closed_periods_no_truncate BEFORE TRUNCATE ON accounting.closed_periods FOR EACH STATEMENT EXECUTE FUNCTION accounting.forbid_change();

CREATE FUNCTION accounting.check_open_period() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM accounting.closed_periods WHERE period = date_trunc('month', NEW.entry_date)::date) THEN
    RAISE EXCEPTION 'accounting period % is closed', to_char(NEW.entry_date, 'YYYY-MM');
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER entries_open_period BEFORE INSERT ON accounting.entries FOR EACH ROW EXECUTE FUNCTION accounting.check_open_period();

ALTER TABLE accounting.closed_periods ENABLE ROW LEVEL SECURITY; ALTER TABLE accounting.closed_periods FORCE ROW LEVEL SECURITY;
CREATE POLICY closed_read  ON accounting.closed_periods FOR SELECT USING (accounting.can_read());
CREATE POLICY closed_write ON accounting.closed_periods FOR INSERT WITH CHECK (platform.has_role('ceo') OR platform.has_role('manager'));
GRANT SELECT, INSERT ON accounting.closed_periods TO hekate_app;
