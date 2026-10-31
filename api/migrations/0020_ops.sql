-- Operational visibility without exposing the audit trail: how many audit rows still wait to be sealed.
CREATE FUNCTION platform.verify_audit_backlog() RETURNS TABLE (unsealed bigint)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = platform, pg_temp AS $$
  SELECT count(*) FROM platform.audit_log WHERE hash IS NULL
$$;
REVOKE ALL ON FUNCTION platform.verify_audit_backlog FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.verify_audit_backlog TO hekate_app;
