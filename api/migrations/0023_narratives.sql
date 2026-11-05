-- Text documents addressed to a client: proposals, order confirmations, statements of work, letters
-- and certificates. Drafts are editable; issuing gives the document its reference and freezes it.
CREATE TABLE documents.narratives (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind       text NOT NULL CHECK (kind IN ('proposal','order_confirmation','sow','letter','certificate')),
  tenant_id  uuid NOT NULL REFERENCES platform.tenants(id),
  title      text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
  sections   jsonb NOT NULL CHECK (jsonb_typeof(sections) = 'array' AND jsonb_array_length(sections) BETWEEN 1 AND 60),
  ref        text UNIQUE,
  signed_by  text,
  created_by text NOT NULL,
  issued_by  text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON documents.narratives (tenant_id, created_at DESC);

CREATE FUNCTION documents.narratives_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    IF OLD.ref IS NOT NULL THEN RAISE EXCEPTION 'an issued document cannot be deleted'; END IF;
    RETURN OLD;
  END IF;
  IF OLD.ref IS NOT NULL THEN RAISE EXCEPTION 'an issued document cannot change'; END IF;
  IF NEW.tenant_id <> OLD.tenant_id OR NEW.kind <> OLD.kind OR NEW.created_by <> OLD.created_by THEN
    RAISE EXCEPTION 'kind, client and author cannot change';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER narratives_guard BEFORE UPDATE OR DELETE ON documents.narratives FOR EACH ROW EXECUTE FUNCTION documents.narratives_guard();

ALTER TABLE documents.narratives ENABLE ROW LEVEL SECURITY; ALTER TABLE documents.narratives FORCE ROW LEVEL SECURITY;
CREATE POLICY nar_staff ON documents.narratives FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY nar_client_read ON documents.narratives FOR SELECT USING (tenant_id = platform.current_tenant() AND ref IS NOT NULL);
GRANT SELECT, INSERT, UPDATE, DELETE ON documents.narratives TO hekate_app;
