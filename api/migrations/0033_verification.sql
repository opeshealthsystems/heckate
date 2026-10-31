-- Public verification: a log of every successful lookup, access requests from people who verified a document and
-- want to see it, and time-limited download links that staff grant. Everything the public can reach goes through a
-- narrow SECURITY DEFINER function; the tables themselves are for staff only.

-- The public pages read the register through definer functions, so what they need lives in the register itself:
-- the revision number is recorded there when a document is issued (it was only on the text-document table).
ALTER TABLE documents.register ADD COLUMN revision int NOT NULL DEFAULT 1 CHECK (revision >= 1);
UPDATE documents.register r SET revision = n.revision FROM documents.narratives n WHERE n.ref = r.ref AND n.revision <> 1;
-- The replacement table is read and written by definer functions too; staff policies still apply to everyone else.
ALTER TABLE documents.supersessions NO FORCE ROW LEVEL SECURITY;

-- 1. Verification log. Only references that exist are logged, so a flood of made-up references cannot fill it.
CREATE TABLE documents.verifications (
  id      bigserial PRIMARY KEY,
  ref     text NOT NULL REFERENCES documents.register(ref),
  at      timestamptz NOT NULL DEFAULT now(),
  channel text NOT NULL CHECK (channel IN ('reference','qr','file')),
  result  text NOT NULL CHECK (result IN ('authentic','replaced','revoked','signature_invalid'))
);
CREATE INDEX ON documents.verifications (ref, at DESC);
CREATE FUNCTION documents.log_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'this log cannot be changed or deleted'; END $$;
CREATE TRIGGER verifications_guard BEFORE UPDATE OR DELETE ON documents.verifications FOR EACH ROW EXECUTE FUNCTION documents.log_immutable();
-- Not FORCEd: the SECURITY DEFINER functions below run as the table owner and are the only way in.
ALTER TABLE documents.verifications ENABLE ROW LEVEL SECURITY;
CREATE POLICY verifications_staff ON documents.verifications FOR SELECT USING (platform.is_staff());
GRANT SELECT ON documents.verifications TO hekate_app;

CREATE FUNCTION documents.log_verification(p_ref text, p_channel text, p_result text) RETURNS void
LANGUAGE sql SECURITY DEFINER SET search_path = documents, pg_temp AS $$
  INSERT INTO documents.verifications (ref, channel, result)
  SELECT p_ref, p_channel, p_result WHERE EXISTS (SELECT 1 FROM documents.register r WHERE r.ref = p_ref)
$$;

-- What the public verification page shows beyond the register entry: the revision, what replaced it, how often it was checked.
CREATE FUNCTION documents.public_info(p_ref text)
RETURNS TABLE (revision int, superseded_by text, verified_count bigint, last_verified timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = documents, pg_temp AS $$
  SELECT COALESCE((SELECT r.revision FROM documents.register r WHERE r.ref = p_ref), 1),
         (SELECT s.new_ref FROM documents.supersessions s WHERE s.old_ref = p_ref),
         (SELECT count(*) FROM documents.verifications v WHERE v.ref = p_ref),
         (SELECT max(v.at) FROM documents.verifications v WHERE v.ref = p_ref)
$$;
REVOKE ALL ON FUNCTION documents.log_verification, documents.public_info FROM PUBLIC;
GRANT EXECUTE ON FUNCTION documents.log_verification, documents.public_info TO hekate_app;

-- 2. Access requests: someone who verified a document asks to see it, and why.
CREATE TABLE documents.access_requests (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ref          text NOT NULL REFERENCES documents.register(ref),
  name         text NOT NULL CHECK (length(name) BETWEEN 2 AND 120),
  email        text NOT NULL CHECK (length(email) <= 200 AND email ~ '^[^@\s]+@[^@\s]+\.[^@\s]+$'),
  organisation text NOT NULL DEFAULT '' CHECK (length(organisation) <= 200),
  reason       text NOT NULL CHECK (length(reason) BETWEEN 20 AND 1000),
  status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','denied')),
  requested_at timestamptz NOT NULL DEFAULT now(),
  decided_by   text,
  decided_at   timestamptz,
  decision_note text NOT NULL DEFAULT '' CHECK (length(decision_note) <= 500)
);
CREATE INDEX ON documents.access_requests (status, requested_at DESC);
CREATE INDEX ON documents.access_requests (ref);
CREATE FUNCTION documents.can_decide_access() RETURNS boolean LANGUAGE sql STABLE AS $$
  SELECT platform.has_role('ceo') OR platform.has_role('manager')
$$;
GRANT EXECUTE ON FUNCTION documents.can_decide_access TO hekate_app;
ALTER TABLE documents.access_requests ENABLE ROW LEVEL SECURITY;
CREATE POLICY access_requests_read ON documents.access_requests FOR SELECT USING (platform.is_staff() AND documents.can_decide_access());
CREATE POLICY access_requests_decide ON documents.access_requests FOR UPDATE USING (platform.is_staff() AND documents.can_decide_access()) WITH CHECK (platform.is_staff() AND documents.can_decide_access());
GRANT SELECT, UPDATE ON documents.access_requests TO hekate_app;

-- A decision is final and only the decision fields may change.
CREATE FUNCTION documents.access_requests_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status <> 'pending' THEN RAISE EXCEPTION 'this request has already been decided'; END IF;
  IF NEW.ref <> OLD.ref OR NEW.name <> OLD.name OR NEW.email <> OLD.email OR NEW.organisation <> OLD.organisation
     OR NEW.reason <> OLD.reason OR NEW.requested_at <> OLD.requested_at THEN
    RAISE EXCEPTION 'a request cannot be edited, only decided';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER access_requests_guard BEFORE UPDATE ON documents.access_requests FOR EACH ROW EXECUTE FUNCTION documents.access_requests_guard();

-- Public entry point. Limits: three open requests per person per document, thirty per document per day.
CREATE FUNCTION documents.request_access(p_ref text, p_name text, p_email text, p_org text, p_reason text) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path = documents, pg_temp AS $$
DECLARE v_id uuid;
BEGIN
  IF NOT EXISTS (SELECT 1 FROM documents.register r WHERE r.ref = p_ref) THEN RAISE EXCEPTION 'unknown document' USING ERRCODE = 'P0002'; END IF;
  IF (SELECT count(*) FROM documents.access_requests a WHERE a.ref = p_ref AND lower(a.email) = lower(p_email) AND a.status = 'pending') >= 3
     OR (SELECT count(*) FROM documents.access_requests a WHERE a.ref = p_ref AND a.requested_at > now() - interval '1 day') >= 30 THEN
    RAISE EXCEPTION 'too many requests' USING ERRCODE = 'P0003';
  END IF;
  INSERT INTO documents.access_requests (ref, name, email, organisation, reason) VALUES (p_ref, btrim(p_name), btrim(p_email), btrim(p_org), btrim(p_reason)) RETURNING id INTO v_id;
  RETURN v_id;
END $$;
REVOKE ALL ON FUNCTION documents.request_access FROM PUBLIC;
GRANT EXECUTE ON FUNCTION documents.request_access TO hekate_app;

-- 3. Download links granted to an approved request. Only a hash of the token is stored.
CREATE TABLE documents.access_grants (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id    uuid NOT NULL UNIQUE REFERENCES documents.access_requests(id),
  ref           text NOT NULL REFERENCES documents.register(ref),
  token_hash    text NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'),
  expires_at    timestamptz NOT NULL,
  max_downloads int NOT NULL CHECK (max_downloads BETWEEN 1 AND 10),
  downloads     int NOT NULL DEFAULT 0 CHECK (downloads >= 0),
  created_by    text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  CHECK (downloads <= max_downloads)
);
CREATE INDEX ON documents.access_grants (ref);
ALTER TABLE documents.access_grants ENABLE ROW LEVEL SECURITY;
CREATE POLICY access_grants_staff ON documents.access_grants FOR SELECT USING (platform.is_staff() AND documents.can_decide_access());
CREATE POLICY access_grants_insert ON documents.access_grants FOR INSERT WITH CHECK (platform.is_staff() AND documents.can_decide_access());
GRANT SELECT, INSERT ON documents.access_grants TO hekate_app;

-- Redeeming a link: counts the download and returns the reference, or nothing if the link is unknown, used up,
-- expired, or the document has been revoked.
CREATE FUNCTION documents.redeem_grant(p_token_hash text) RETURNS text
LANGUAGE sql SECURITY DEFINER SET search_path = documents, pg_temp AS $$
  WITH g AS (
    UPDATE documents.access_grants x SET downloads = downloads + 1
    WHERE x.token_hash = p_token_hash AND x.expires_at > now() AND x.downloads < x.max_downloads
      AND EXISTS (SELECT 1 FROM documents.register r WHERE r.ref = x.ref AND r.status = 'valid')
    RETURNING x.ref)
  SELECT ref FROM g
$$;
REVOKE ALL ON FUNCTION documents.redeem_grant FROM PUBLIC;
GRANT EXECUTE ON FUNCTION documents.redeem_grant TO hekate_app;
