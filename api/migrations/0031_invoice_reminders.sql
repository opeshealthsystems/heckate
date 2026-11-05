-- Payment reminders. One row per invoice per stage (1: overdue, 2: a week late, 3: a month late) so a
-- reminder is sent once however often the scheduler runs or how many instances run it.
CREATE TABLE sales.invoice_reminders (
  invoice_id uuid NOT NULL REFERENCES sales.invoices(id),
  stage      smallint NOT NULL CHECK (stage BETWEEN 1 AND 3),
  days_late  int NOT NULL CHECK (days_late >= 1),
  owed       bigint NOT NULL CHECK (owed > 0),
  sent_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (invoice_id, stage)
);
ALTER TABLE sales.invoice_reminders ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales.invoice_reminders FORCE ROW LEVEL SECURITY;
CREATE POLICY invoice_reminders_staff ON sales.invoice_reminders FOR ALL USING (sales.can_see()) WITH CHECK (sales.can_see());
GRANT SELECT, INSERT ON sales.invoice_reminders TO hekate_app;
