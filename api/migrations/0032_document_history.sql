-- Version history for text documents. Every save of a draft keeps a numbered snapshot with who changed it, when and
-- why, plus a fingerprint of the content; the snapshots can never be changed or deleted. An issued document is frozen,
-- so a change after issue is a REVISION: a new document that supersedes the old one, which stays valid and verifiable
-- but is shown as replaced.
CREATE TABLE documents.narrative_versions (
  narrative_id   uuid NOT NULL REFERENCES documents.narratives(id),
  version        int  NOT NULL CHECK (version >= 1),
  event          text NOT NULL CHECK (event IN ('created','edited','issued')),
  title          text NOT NULL,
  sections       jsonb NOT NULL,
  signed_by      text,
  change_note    text NOT NULL DEFAULT '' CHECK (length(change_note) <= 500),
  changed_by     text NOT NULL,
  changed_at     timestamptz NOT NULL DEFAULT now(),
  content_sha256 text NOT NULL CHECK (content_sha256 ~ '^[0-9a-f]{64}$'),
  PRIMARY KEY (narrative_id, version)
);
CREATE FUNCTION documents.versions_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'document history cannot be changed or deleted'; END $$;
CREATE TRIGGER versions_guard BEFORE UPDATE OR DELETE ON documents.narrative_versions FOR EACH ROW EXECUTE FUNCTION documents.versions_immutable();
ALTER TABLE documents.narrative_versions ENABLE ROW LEVEL SECURITY; ALTER TABLE documents.narrative_versions FORCE ROW LEVEL SECURITY;
-- Staff who may see the document may see its history; the subquery is itself filtered by the narratives policies.
CREATE POLICY nv_staff ON documents.narrative_versions FOR ALL
  USING (platform.is_staff() AND EXISTS (SELECT 1 FROM documents.narratives n WHERE n.id = narrative_id))
  WITH CHECK (platform.is_staff() AND EXISTS (SELECT 1 FROM documents.narratives n WHERE n.id = narrative_id));
GRANT SELECT, INSERT ON documents.narrative_versions TO hekate_app;

-- A revision points at the document it replaces.
ALTER TABLE documents.narratives ADD COLUMN supersedes uuid REFERENCES documents.narratives(id);
ALTER TABLE documents.narratives ADD COLUMN revision int NOT NULL DEFAULT 1 CHECK (revision >= 1);
CREATE UNIQUE INDEX narratives_one_revision ON documents.narratives (supersedes) WHERE supersedes IS NOT NULL;

CREATE OR REPLACE FUNCTION documents.narratives_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    IF OLD.ref IS NOT NULL THEN RAISE EXCEPTION 'an issued document cannot be deleted'; END IF;
    RETURN OLD;
  END IF;
  IF OLD.ref IS NOT NULL THEN RAISE EXCEPTION 'an issued document cannot change'; END IF;
  IF NEW.tenant_id <> OLD.tenant_id OR NEW.kind <> OLD.kind OR NEW.created_by <> OLD.created_by
     OR NEW.supersedes IS DISTINCT FROM OLD.supersedes OR NEW.revision <> OLD.revision THEN
    RAISE EXCEPTION 'kind, client, author and revision link cannot change';
  END IF;
  RETURN NEW;
END $$;

-- Which reference replaced this one, if any. Public (verification shows it) and exposes nothing else.
CREATE TABLE documents.supersessions (
  old_ref text PRIMARY KEY REFERENCES documents.register(ref),
  new_ref text NOT NULL UNIQUE REFERENCES documents.register(ref),
  at      timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION documents.supersessions_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'a replacement cannot be undone'; END $$;
CREATE TRIGGER supersessions_guard BEFORE UPDATE OR DELETE ON documents.supersessions FOR EACH ROW EXECUTE FUNCTION documents.supersessions_immutable();
ALTER TABLE documents.supersessions ENABLE ROW LEVEL SECURITY; ALTER TABLE documents.supersessions FORCE ROW LEVEL SECURITY;
CREATE POLICY sup_staff ON documents.supersessions FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
GRANT SELECT, INSERT ON documents.supersessions TO hekate_app;
CREATE FUNCTION documents.superseded_by(p_ref text) RETURNS text
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = documents, pg_temp AS $$
  SELECT s.new_ref FROM documents.supersessions s WHERE s.old_ref = p_ref
$$;
REVOKE ALL ON FUNCTION documents.superseded_by FROM PUBLIC;
GRANT EXECUTE ON FUNCTION documents.superseded_by TO hekate_app;
