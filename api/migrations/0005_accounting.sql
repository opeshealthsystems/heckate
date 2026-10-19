-- Accounting: double-entry, append-only, hash-chained ledger on a starter OHADA-style chart.
CREATE SCHEMA IF NOT EXISTS accounting;

-- Fine-grained role check (the coarse staff/client split is not enough for finance data).
CREATE FUNCTION platform.has_role(r text) RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT r = ANY (string_to_array(coalesce(current_setting('app.roles', true), ''), ',')) $$;
GRANT EXECUTE ON FUNCTION platform.has_role TO hekate_app;

CREATE TABLE accounting.accounts (
  code  text PRIMARY KEY,
  name  text NOT NULL,
  class int  NOT NULL CHECK (class BETWEEN 1 AND 9),
  note  text NOT NULL DEFAULT ''
);
-- STARTER SET ONLY. The accountant must validate codes and names against SYSCOHADA (docs/BLOCKERS.md).
INSERT INTO accounting.accounts (code, name, class, note) VALUES
  ('101',  'Capital social', 1, ''),
  ('401',  'Fournisseurs', 4, ''),
  ('411',  'Clients', 4, ''),
  ('4431', 'TVA facturée sur ventes', 4, ''),
  ('4439', 'CAC facturés sur ventes', 4, 'mapping of the CAC to be confirmed by the accountant'),
  ('4452', 'TVA récupérable sur achats', 4, ''),
  ('521',  'Banques', 5, ''),
  ('5211', 'Mobile money', 5, 'mapping to be confirmed by the accountant'),
  ('571',  'Caisse', 5, ''),
  ('601',  'Achats de marchandises', 6, ''),
  ('661',  'Rémunérations du personnel', 6, ''),
  ('701',  'Ventes de marchandises', 7, ''),
  ('706',  'Services vendus', 7, ''),
  ('709',  'Rabais, remises et ristournes accordés', 7, '');

CREATE TABLE accounting.journals (code text PRIMARY KEY, name text NOT NULL);
INSERT INTO accounting.journals VALUES ('VT','Ventes'),('AC','Achats'),('BQ','Banque'),('CA','Caisse'),('OD','Opérations diverses');

CREATE TABLE accounting.entries (
  seq        bigserial PRIMARY KEY,
  journal    text NOT NULL REFERENCES accounting.journals(code),
  entry_date date NOT NULL,
  ref        text NOT NULL DEFAULT '',
  narration  text NOT NULL,
  source     text NOT NULL DEFAULT 'manual',
  source_ref text NOT NULL DEFAULT '',
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  prev_hash  text NOT NULL,
  hash       text NOT NULL UNIQUE
);
CREATE TABLE accounting.entry_lines (
  id        bigserial PRIMARY KEY,
  entry_seq bigint NOT NULL REFERENCES accounting.entries(seq),
  position  int NOT NULL,
  account   text NOT NULL REFERENCES accounting.accounts(code),
  debit     bigint NOT NULL DEFAULT 0 CHECK (debit >= 0),
  credit    bigint NOT NULL DEFAULT 0 CHECK (credit >= 0),
  party     uuid,
  CHECK ((debit = 0) <> (credit = 0))   -- exactly one side per line
);
CREATE INDEX ON accounting.entry_lines (account);
CREATE INDEX ON accounting.entry_lines (entry_seq);
CREATE INDEX ON accounting.entries (entry_date);

-- Append-only: no update, delete or truncate, for any role.
CREATE FUNCTION accounting.forbid_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'the ledger is append-only'; END $$;
CREATE TRIGGER entries_append_only BEFORE UPDATE OR DELETE ON accounting.entries FOR EACH ROW EXECUTE FUNCTION accounting.forbid_change();
CREATE TRIGGER lines_append_only BEFORE UPDATE OR DELETE ON accounting.entry_lines FOR EACH ROW EXECUTE FUNCTION accounting.forbid_change();
CREATE TRIGGER entries_no_truncate BEFORE TRUNCATE ON accounting.entries FOR EACH STATEMENT EXECUTE FUNCTION accounting.forbid_change();
CREATE TRIGGER lines_no_truncate BEFORE TRUNCATE ON accounting.entry_lines FOR EACH STATEMENT EXECUTE FUNCTION accounting.forbid_change();

-- Every entry must balance, checked at commit so lines can be inserted one by one.
CREATE FUNCTION accounting.check_balanced() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE d bigint; c bigint; n int;
BEGIN
  SELECT coalesce(sum(debit),0), coalesce(sum(credit),0), count(*) INTO d, c, n FROM accounting.entry_lines WHERE entry_seq = NEW.entry_seq;
  IF d <> c OR n < 2 THEN RAISE EXCEPTION 'entry % is not balanced (debit %, credit %, % lines)', NEW.entry_seq, d, c, n; END IF;
  RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER lines_balanced AFTER INSERT ON accounting.entry_lines DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION accounting.check_balanced();

ALTER TABLE accounting.accounts ENABLE ROW LEVEL SECURITY;    ALTER TABLE accounting.accounts FORCE ROW LEVEL SECURITY;
ALTER TABLE accounting.entries ENABLE ROW LEVEL SECURITY;     ALTER TABLE accounting.entries FORCE ROW LEVEL SECURITY;
ALTER TABLE accounting.entry_lines ENABLE ROW LEVEL SECURITY; ALTER TABLE accounting.entry_lines FORCE ROW LEVEL SECURITY;
ALTER TABLE accounting.journals ENABLE ROW LEVEL SECURITY;    ALTER TABLE accounting.journals FORCE ROW LEVEL SECURITY;

-- Read: finance, management and the (time-limited) external auditor. Write: finance and management.
CREATE FUNCTION accounting.can_read() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT platform.has_role('finance') OR platform.has_role('manager') OR platform.has_role('ceo') OR platform.has_role('auditor') $$;
CREATE FUNCTION accounting.can_write() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT platform.has_role('finance') OR platform.has_role('manager') OR platform.has_role('ceo') $$;
GRANT EXECUTE ON FUNCTION accounting.can_read, accounting.can_write TO hekate_app;

CREATE POLICY accounts_read  ON accounting.accounts FOR SELECT USING (accounting.can_read());
CREATE POLICY accounts_write ON accounting.accounts FOR INSERT WITH CHECK (accounting.can_write());
CREATE POLICY journals_read  ON accounting.journals FOR SELECT USING (accounting.can_read());
CREATE POLICY entries_read   ON accounting.entries  FOR SELECT USING (accounting.can_read());
CREATE POLICY entries_write  ON accounting.entries  FOR INSERT WITH CHECK (accounting.can_write());
CREATE POLICY lines_read     ON accounting.entry_lines FOR SELECT USING (accounting.can_read());
CREATE POLICY lines_write    ON accounting.entry_lines FOR INSERT WITH CHECK (accounting.can_write());

GRANT USAGE ON SCHEMA accounting TO hekate_app;
GRANT SELECT, INSERT ON accounting.accounts, accounting.entries, accounting.entry_lines TO hekate_app;
GRANT SELECT ON accounting.journals TO hekate_app;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA accounting TO hekate_app;
