-- File attachments: incident photos, intervention photos and expense receipts.
-- Stored in Postgres (<= 10 MB each) behind the same row-level security as the records they belong to.
-- The storage sits behind internal/modules/files.Store so it can move to MinIO later without API changes.
CREATE SCHEMA IF NOT EXISTS files;

CREATE TABLE files.objects (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid REFERENCES platform.tenants(id),       -- null for staff-only records such as expense receipts
  owner        text NOT NULL,
  entity_type  text NOT NULL CHECK (entity_type IN ('ticket','intervention','expense')),
  entity_id    uuid NOT NULL,
  filename     text NOT NULL,
  content_type text NOT NULL CHECK (content_type IN ('image/jpeg','image/png','image/webp','application/pdf')),
  size         int  NOT NULL CHECK (size > 0 AND size <= 10485760),
  sha256       text NOT NULL,
  data         bytea NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  CHECK (octet_length(data) = size),
  CHECK (entity_type = 'expense' OR tenant_id IS NOT NULL)
);
CREATE INDEX ON files.objects (entity_type, entity_id);

ALTER TABLE files.objects ENABLE ROW LEVEL SECURITY;
ALTER TABLE files.objects FORCE ROW LEVEL SECURITY;

-- Tickets and interventions follow their tenant: staff see all, a client sees its own.
-- Expense receipts are private to the claimant, HR and management, and finance.
CREATE POLICY files_read ON files.objects FOR SELECT USING (
  (entity_type IN ('ticket','intervention') AND (platform.is_staff() OR tenant_id = platform.current_tenant()))
  OR (entity_type = 'expense' AND (owner = current_setting('app.actor', true)
        OR platform.has_role('hr') OR platform.has_role('manager') OR platform.has_role('ceo') OR platform.has_role('finance')))
);
CREATE POLICY files_write ON files.objects FOR INSERT WITH CHECK (
  owner = current_setting('app.actor', true)
  AND ((entity_type IN ('ticket','intervention') AND (platform.is_staff() OR tenant_id = platform.current_tenant()))
       OR entity_type = 'expense')
);

-- Attachments are evidence: no edits, no deletes.
CREATE FUNCTION files.immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'attachments are immutable'; END $$;
CREATE TRIGGER objects_immutable BEFORE UPDATE OR DELETE ON files.objects FOR EACH ROW EXECUTE FUNCTION files.immutable();

GRANT USAGE ON SCHEMA files TO hekate_app;
GRANT SELECT, INSERT ON files.objects TO hekate_app;
