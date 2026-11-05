-- Projects & Field Service: projects, tasks, on-site interventions and timesheets.
CREATE SCHEMA IF NOT EXISTS projects;

CREATE TABLE projects.projects (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES platform.tenants(id),
  name        text NOT NULL,
  status      text NOT NULL DEFAULT 'planning' CHECK (status IN ('planning','active','on_hold','done','cancelled')),
  manager     text,
  start_date  date,
  end_date    date,
  budget      bigint CHECK (budget IS NULL OR budget >= 0),
  created_by  text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  CHECK (end_date IS NULL OR start_date IS NULL OR end_date >= start_date)
);
CREATE TABLE projects.tasks (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  uuid NOT NULL REFERENCES projects.projects(id) ON DELETE CASCADE,
  tenant_id   uuid NOT NULL REFERENCES platform.tenants(id),
  title       text NOT NULL,
  status      text NOT NULL DEFAULT 'todo' CHECK (status IN ('todo','doing','done')),
  assignee    text,
  due_date    date,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE projects.interventions (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL REFERENCES platform.tenants(id),
  project_id       uuid REFERENCES projects.projects(id),
  ticket_id        uuid,             -- support ticket; plain id, no cross-module foreign key
  engineer         text NOT NULL,
  status           text NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled','in_progress','completed','cancelled')),
  scheduled_for    timestamptz,
  summary          text NOT NULL DEFAULT '',
  checkin_at       timestamptz,      -- device time, so offline work keeps its true time
  checkin_lat      double precision CHECK (checkin_lat BETWEEN -90 AND 90),
  checkin_lng      double precision CHECK (checkin_lng BETWEEN -180 AND 180),
  checkout_at      timestamptz,
  checkout_lat     double precision CHECK (checkout_lat BETWEEN -90 AND 90),
  checkout_lng     double precision CHECK (checkout_lng BETWEEN -180 AND 180),
  report           text,
  signed_by        text,
  signature        text,             -- PNG data URL drawn on screen by the client
  document_ref     text,
  created_by       text NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  CHECK (checkout_at IS NULL OR checkin_at IS NULL OR checkout_at >= checkin_at)
);
CREATE TABLE projects.timesheets (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor       text NOT NULL,
  work_date   date NOT NULL,
  hours       numeric(4,2) NOT NULL CHECK (hours > 0 AND hours <= 24),
  project_id  uuid REFERENCES projects.projects(id),
  ticket_id   uuid,
  note        text NOT NULL DEFAULT '',
  status      text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','submitted','approved')),
  approved_by text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON projects.tasks (project_id);
CREATE INDEX ON projects.interventions (tenant_id, created_at DESC);
CREATE INDEX ON projects.interventions (engineer);
CREATE INDEX ON projects.timesheets (actor, work_date);

-- Approved timesheets are payroll and billing inputs: freeze them.
CREATE FUNCTION projects.guard_timesheet() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status = 'approved' THEN RAISE EXCEPTION 'approved timesheets are immutable'; END IF;
  IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER timesheet_guard BEFORE UPDATE OR DELETE ON projects.timesheets FOR EACH ROW EXECUTE FUNCTION projects.guard_timesheet();

-- A completed intervention is a signed record.
CREATE FUNCTION projects.guard_intervention() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status = 'completed' THEN RAISE EXCEPTION 'completed interventions are immutable'; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER intervention_guard BEFORE UPDATE ON projects.interventions FOR EACH ROW EXECUTE FUNCTION projects.guard_intervention();

ALTER TABLE projects.projects ENABLE ROW LEVEL SECURITY;      ALTER TABLE projects.projects FORCE ROW LEVEL SECURITY;
ALTER TABLE projects.tasks ENABLE ROW LEVEL SECURITY;         ALTER TABLE projects.tasks FORCE ROW LEVEL SECURITY;
ALTER TABLE projects.interventions ENABLE ROW LEVEL SECURITY; ALTER TABLE projects.interventions FORCE ROW LEVEL SECURITY;
ALTER TABLE projects.timesheets ENABLE ROW LEVEL SECURITY;    ALTER TABLE projects.timesheets FORCE ROW LEVEL SECURITY;

CREATE POLICY projects_staff  ON projects.projects FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY projects_client ON projects.projects FOR SELECT USING (tenant_id = platform.current_tenant());
CREATE POLICY tasks_staff     ON projects.tasks FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY tasks_client    ON projects.tasks FOR SELECT USING (tenant_id = platform.current_tenant());
CREATE POLICY interv_staff    ON projects.interventions FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY interv_client   ON projects.interventions FOR SELECT USING (tenant_id = platform.current_tenant() AND status = 'completed');

-- Timesheets: everyone sees and edits their own; management, HR and finance see all.
CREATE POLICY ts_own ON projects.timesheets FOR ALL USING (actor = current_setting('app.actor', true)) WITH CHECK (actor = current_setting('app.actor', true));
CREATE POLICY ts_review ON projects.timesheets FOR SELECT USING (platform.has_role('manager') OR platform.has_role('ceo') OR platform.has_role('hr') OR platform.has_role('finance'));
CREATE POLICY ts_approve ON projects.timesheets FOR UPDATE USING (platform.has_role('manager') OR platform.has_role('ceo')) WITH CHECK (platform.has_role('manager') OR platform.has_role('ceo'));

GRANT USAGE ON SCHEMA projects TO hekate_app;
GRANT SELECT, INSERT, UPDATE ON projects.projects, projects.tasks, projects.interventions TO hekate_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON projects.timesheets TO hekate_app;
