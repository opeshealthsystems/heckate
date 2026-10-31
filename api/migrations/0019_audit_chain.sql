-- Tamper-evident audit trail. Rows are written by requests without any chaining (so requests never
-- queue behind each other); a background sealer then links them, in the order it sees them, into a
-- hash chain. Rewriting, deleting or inserting into sealed history is detected by platform.verify_audit().
ALTER TABLE platform.audit_log ADD COLUMN chain_seq bigint UNIQUE;
ALTER TABLE platform.audit_log ADD COLUMN prev_hash text;
ALTER TABLE platform.audit_log ADD COLUMN hash text UNIQUE;

CREATE FUNCTION platform.audit_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP <> 'UPDATE' THEN RAISE EXCEPTION 'the audit log is append-only'; END IF;
  -- The only permitted change: an unsealed row receives its chain fields, nothing else.
  IF OLD.hash IS NOT NULL
     OR NEW.id <> OLD.id OR NEW.at <> OLD.at OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id OR NEW.actor <> OLD.actor
     OR NEW.action <> OLD.action OR NEW.entity <> OLD.entity OR NEW.entity_id IS DISTINCT FROM OLD.entity_id
     OR NEW.trace_id IS DISTINCT FROM OLD.trace_id THEN
    RAISE EXCEPTION 'the audit log is append-only';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER audit_guard BEFORE UPDATE OR DELETE ON platform.audit_log FOR EACH ROW EXECUTE FUNCTION platform.audit_guard();
CREATE TRIGGER audit_no_truncate BEFORE TRUNCATE ON platform.audit_log FOR EACH STATEMENT EXECUTE FUNCTION platform.audit_guard();

CREATE FUNCTION platform.audit_digest(p_prev text, p_seq bigint, r platform.audit_log) RETURNS text LANGUAGE sql IMMUTABLE AS $$
  SELECT encode(sha256(convert_to(concat_ws('|', p_prev, p_seq, r.id, extract(epoch FROM r.at)::text, coalesce(r.tenant_id::text,''),
    r.actor, r.action, r.entity, coalesce(r.entity_id,''), coalesce(r.trace_id,'')), 'UTF8')), 'hex')
$$;

-- Links every new row into the chain. One sealer at a time; a second call while one runs does nothing.
CREATE FUNCTION platform.seal_audit() RETURNS integer LANGUAGE plpgsql SECURITY DEFINER SET search_path = platform, pg_temp AS $$
DECLARE r platform.audit_log; prev text; seq bigint; link text; n integer := 0;
BEGIN
  IF NOT pg_try_advisory_xact_lock(727276) THEN RETURN 0; END IF;
  SELECT hash, chain_seq INTO prev, seq FROM platform.audit_log WHERE hash IS NOT NULL ORDER BY chain_seq DESC LIMIT 1;
  prev := coalesce(prev, 'genesis'); seq := coalesce(seq, 0);
  FOR r IN SELECT * FROM platform.audit_log WHERE hash IS NULL ORDER BY id LIMIT 5000 LOOP
    seq := seq + 1;
    link := prev;
    prev := platform.audit_digest(link, seq, r);
    UPDATE platform.audit_log SET chain_seq = seq, prev_hash = link, hash = prev WHERE id = r.id;
    n := n + 1;
  END LOOP;
  RETURN n;
END $$;
REVOKE ALL ON FUNCTION platform.seal_audit FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.seal_audit TO hekate_app;

-- Recomputes the whole chain. Returns how many rows were checked and the first chain_seq that does not match.
CREATE FUNCTION platform.verify_audit() RETURNS TABLE (checked bigint, first_broken bigint, unsealed bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = platform, pg_temp AS $$
DECLARE r platform.audit_log; prev text := 'genesis'; want bigint := 1; broken bigint := NULL; n bigint := 0;
BEGIN
  IF NOT (platform.has_role('ceo') OR platform.has_role('manager') OR platform.has_role('auditor')) THEN
    RAISE EXCEPTION 'not allowed';
  END IF;
  FOR r IN SELECT * FROM platform.audit_log WHERE hash IS NOT NULL ORDER BY chain_seq LOOP
    n := n + 1;
    IF r.chain_seq <> want OR r.prev_hash <> prev OR r.hash <> platform.audit_digest(prev, r.chain_seq, r) THEN
      broken := coalesce(broken, r.chain_seq, want); EXIT;
    END IF;
    prev := r.hash; want := want + 1;
  END LOOP;
  RETURN QUERY SELECT n, broken, (SELECT count(*) FROM platform.audit_log WHERE hash IS NULL);
END $$;
REVOKE ALL ON FUNCTION platform.verify_audit FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.verify_audit TO hekate_app;

-- Reading the trail is for management and the auditor only.
CREATE FUNCTION platform.audit_read(p_limit int, p_before bigint, p_actor text, p_entity text)
RETURNS SETOF platform.audit_log LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = platform, pg_temp AS $$
BEGIN
  IF NOT (platform.has_role('ceo') OR platform.has_role('manager') OR platform.has_role('auditor')) THEN
    RAISE EXCEPTION 'not allowed';
  END IF;
  RETURN QUERY SELECT * FROM platform.audit_log
    WHERE (p_before IS NULL OR id < p_before) AND (p_actor IS NULL OR actor = p_actor) AND (p_entity IS NULL OR entity = p_entity)
    ORDER BY id DESC LIMIT least(greatest(p_limit, 1), 500);
END $$;
REVOKE ALL ON FUNCTION platform.audit_read FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.audit_read TO hekate_app;
