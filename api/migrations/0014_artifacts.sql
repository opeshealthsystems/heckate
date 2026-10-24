-- The PDF of an issued document is rendered once and stored. Every later download returns exactly
-- those bytes, so the SHA-256 in the public register identifies one file for the document's life, even
-- if the letterhead or the PDF library changes afterwards.
CREATE TABLE documents.artifacts (
  ref        text PRIMARY KEY REFERENCES documents.register(ref),
  sha256     text NOT NULL,
  content    bytea NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (octet_length(content) > 0)
);
CREATE FUNCTION documents.artifacts_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'document artifacts are immutable'; END $$;
CREATE TRIGGER artifacts_guard BEFORE UPDATE OR DELETE ON documents.artifacts FOR EACH ROW EXECUTE FUNCTION documents.artifacts_immutable();

-- The register's hash can be set once and never changed.
CREATE FUNCTION documents.guard_register_hash() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.sha256 IS NOT NULL AND NEW.sha256 IS DISTINCT FROM OLD.sha256 THEN RAISE EXCEPTION 'a registered hash cannot change'; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER register_hash_guard BEFORE UPDATE ON documents.register FOR EACH ROW EXECUTE FUNCTION documents.guard_register_hash();

-- Only reachable through the functions below. The caller (a module that already authorised access to the
-- record behind the reference through its own row-level security) never touches the table directly.
ALTER TABLE documents.artifacts ENABLE ROW LEVEL SECURITY;
CREATE POLICY artifacts_none ON documents.artifacts FOR ALL USING (false) WITH CHECK (false);

CREATE FUNCTION documents.get_artifact(p_ref text) RETURNS bytea LANGUAGE sql STABLE SECURITY DEFINER SET search_path = documents, pg_temp AS $$
  SELECT content FROM documents.artifacts WHERE ref = p_ref
$$;

-- Stores the first rendering (later ones are ignored), records its hash in the register, and returns
-- the canonical bytes: the stored ones if another request got there first.
CREATE FUNCTION documents.put_artifact(p_ref text, p_sha text, p_content bytea) RETURNS bytea LANGUAGE plpgsql SECURITY DEFINER SET search_path = documents, pg_temp AS $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM documents.register WHERE ref = p_ref) THEN
    RAISE EXCEPTION 'unknown document reference %', p_ref;
  END IF;
  INSERT INTO documents.artifacts (ref, sha256, content) VALUES (p_ref, p_sha, p_content) ON CONFLICT (ref) DO NOTHING;
  UPDATE documents.register SET sha256 = (SELECT a.sha256 FROM documents.artifacts a WHERE a.ref = p_ref) WHERE ref = p_ref AND sha256 IS NULL;
  RETURN (SELECT content FROM documents.artifacts WHERE ref = p_ref);
END $$;

REVOKE ALL ON FUNCTION documents.get_artifact, documents.put_artifact FROM PUBLIC;
GRANT EXECUTE ON FUNCTION documents.get_artifact, documents.put_artifact TO hekate_app;

-- Public verification also returns the recorded hash, so anyone can check a file they hold.
DROP FUNCTION documents.verify(text);
CREATE FUNCTION documents.verify(p_ref text)
RETURNS TABLE (ref text, code text, title text, status text, issued_at timestamptz, sha256 text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = documents, pg_temp AS $$
  SELECT r.ref, r.code, r.title, r.status, r.issued_at, r.sha256 FROM documents.register r WHERE r.ref = p_ref
$$;
REVOKE ALL ON FUNCTION documents.verify FROM PUBLIC;
GRANT EXECUTE ON FUNCTION documents.verify TO hekate_app;
