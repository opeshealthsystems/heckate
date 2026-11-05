-- The external auditor (read-only, time-limited by token expiry) can view issued documents as well as the ledger.
DROP POLICY register_read ON documents.register;
CREATE POLICY register_read ON documents.register FOR SELECT
  USING (platform.is_staff() OR tenant_id = platform.current_tenant() OR platform.has_role('auditor'));
