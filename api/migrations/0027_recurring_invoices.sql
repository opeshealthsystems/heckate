-- Recurring invoice templates. A run turns each due template into a DRAFT invoice that finance then
-- issues as usual (with its dual control); nothing is ever issued or sent automatically.
CREATE TABLE sales.recurring (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL REFERENCES platform.tenants(id),
  title           text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
  lines           jsonb NOT NULL CHECK (jsonb_typeof(lines) = 'array' AND jsonb_array_length(lines) BETWEEN 1 AND 200),
  interval_months int  NOT NULL CHECK (interval_months IN (1, 3, 6, 12)),
  due_days        int  NOT NULL DEFAULT 30 CHECK (due_days BETWEEN 0 AND 365),
  next_run        date NOT NULL,
  active          boolean NOT NULL DEFAULT true,
  created_by      text NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now()
);
-- At most one draft per template per run date, whatever happens to retries and parallel runs.
CREATE TABLE sales.recurring_runs (
  recurring_id uuid NOT NULL REFERENCES sales.recurring(id),
  run_date     date NOT NULL,
  invoice_id   uuid NOT NULL REFERENCES sales.invoices(id),
  PRIMARY KEY (recurring_id, run_date)
);
ALTER TABLE sales.recurring ENABLE ROW LEVEL SECURITY;      ALTER TABLE sales.recurring FORCE ROW LEVEL SECURITY;
ALTER TABLE sales.recurring_runs ENABLE ROW LEVEL SECURITY; ALTER TABLE sales.recurring_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY recurring_staff ON sales.recurring FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY recurring_runs_staff ON sales.recurring_runs FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
GRANT SELECT, INSERT, UPDATE ON sales.recurring TO hekate_app;
GRANT SELECT, INSERT ON sales.recurring_runs TO hekate_app;
