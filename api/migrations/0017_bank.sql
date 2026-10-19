-- Bank reconciliation: statements imported from the bank, matched line by line to ledger lines on the
-- bank account. A ledger line matches at most one bank line, and a statement never changes after import.
CREATE TABLE accounting.bank_statements (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account     text NOT NULL REFERENCES accounting.accounts(code),
  period_from date NOT NULL,
  period_to   date NOT NULL,
  opening     bigint NOT NULL,
  closing     bigint NOT NULL,
  imported_by text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  CHECK (period_to >= period_from)
);
CREATE TABLE accounting.bank_lines (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  statement_id   uuid NOT NULL REFERENCES accounting.bank_statements(id),
  position       int NOT NULL,
  line_date      date NOT NULL,
  description    text NOT NULL,
  reference      text NOT NULL DEFAULT '',
  amount         bigint NOT NULL CHECK (amount <> 0),   -- money in is positive
  entry_line_id  bigint UNIQUE REFERENCES accounting.entry_lines(id),
  matched_by     text,
  matched_at     timestamptz,
  UNIQUE (statement_id, position)
);
CREATE INDEX ON accounting.bank_lines (statement_id);

-- Only the match may change on a bank line; the statement itself is a record of what the bank said.
CREATE FUNCTION accounting.bank_lines_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN RAISE EXCEPTION 'bank statement lines cannot be deleted'; END IF;
  IF NEW.statement_id <> OLD.statement_id OR NEW.position <> OLD.position OR NEW.line_date <> OLD.line_date
     OR NEW.description <> OLD.description OR NEW.reference <> OLD.reference OR NEW.amount <> OLD.amount THEN
    RAISE EXCEPTION 'bank statement lines cannot be edited';
  END IF;
  IF NEW.entry_line_id IS NOT NULL AND NEW.entry_line_id IS DISTINCT FROM OLD.entry_line_id THEN
    IF NOT EXISTS (SELECT 1 FROM accounting.entry_lines l JOIN accounting.bank_statements s ON s.id = NEW.statement_id
                   WHERE l.id = NEW.entry_line_id AND l.account = s.account AND (l.debit - l.credit) = NEW.amount) THEN
      RAISE EXCEPTION 'a bank line can only match a ledger line of the same account and amount';
    END IF;
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER bank_lines_guard BEFORE UPDATE OR DELETE ON accounting.bank_lines FOR EACH ROW EXECUTE FUNCTION accounting.bank_lines_guard();
CREATE FUNCTION accounting.bank_statements_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'bank statements cannot be edited or deleted'; END $$;
CREATE TRIGGER bank_statements_guard BEFORE UPDATE OR DELETE ON accounting.bank_statements FOR EACH ROW EXECUTE FUNCTION accounting.bank_statements_guard();

ALTER TABLE accounting.bank_statements ENABLE ROW LEVEL SECURITY; ALTER TABLE accounting.bank_statements FORCE ROW LEVEL SECURITY;
ALTER TABLE accounting.bank_lines ENABLE ROW LEVEL SECURITY;      ALTER TABLE accounting.bank_lines FORCE ROW LEVEL SECURITY;
CREATE POLICY bst_read  ON accounting.bank_statements FOR SELECT USING (accounting.can_read());
CREATE POLICY bst_write ON accounting.bank_statements FOR INSERT WITH CHECK (accounting.can_write());
CREATE POLICY bln_read  ON accounting.bank_lines FOR SELECT USING (accounting.can_read());
CREATE POLICY bln_write ON accounting.bank_lines FOR INSERT WITH CHECK (accounting.can_write());
CREATE POLICY bln_match ON accounting.bank_lines FOR UPDATE USING (accounting.can_write()) WITH CHECK (accounting.can_write());

GRANT SELECT, INSERT ON accounting.bank_statements TO hekate_app;
GRANT SELECT, INSERT, UPDATE ON accounting.bank_lines TO hekate_app;
