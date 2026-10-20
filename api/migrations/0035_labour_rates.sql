-- Hourly labour cost rate per employee (XAF per hour), effective-dated. Used to price approved timesheet hours in a
-- project's actual cost. There is no default: an employee with no rate on a work date is unpriced, never guessed.
-- Only the CEO, managers and finance can read or write rates (they are pay information).
CREATE TABLE projects.labour_rates (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor          text NOT NULL,
  effective_from date NOT NULL,
  rate           bigint NOT NULL CHECK (rate > 0),
  set_by         text NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (actor, effective_from)
);
ALTER TABLE projects.labour_rates ENABLE ROW LEVEL SECURITY; ALTER TABLE projects.labour_rates FORCE ROW LEVEL SECURITY;
CREATE POLICY labour_rates_all ON projects.labour_rates FOR ALL USING (projects.can_cost()) WITH CHECK (projects.can_cost());
GRANT SELECT, INSERT, UPDATE ON projects.labour_rates TO hekate_app;
CREATE INDEX ix_projects_timesheets_approved ON projects.timesheets (project_id) WHERE status = 'approved';
