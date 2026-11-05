-- Least privilege inside the database. Until now every internal role counted as "staff", so a plain employee, or
-- someone in HR, logistics or procurement, could list every client's invoices, quotes, proposals and tickets, read
-- restricted forms, and see clients' API keys. The external auditor could read individual payslips. Each rule below
-- narrows one of those to the roles that need it. The route role lists in the API say the same thing; this is the
-- layer a bug in a handler cannot get around.

-- Money: quotes, invoices, payments, credit notes and recurring templates belong to management, finance and sales.
CREATE FUNCTION sales.can_see() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT platform.has_role('ceo') OR platform.has_role('manager') OR platform.has_role('finance') OR platform.has_role('sales') $$;
GRANT EXECUTE ON FUNCTION sales.can_see TO hekate_app;

DROP POLICY credit_staff ON sales.credit_notes;
CREATE POLICY credit_staff ON sales.credit_notes FOR ALL USING (sales.can_see()) WITH CHECK (sales.can_see());
DROP POLICY ilines_staff ON sales.invoice_lines;
CREATE POLICY ilines_staff ON sales.invoice_lines FOR ALL USING (sales.can_see()) WITH CHECK (sales.can_see());
DROP POLICY invoices_staff ON sales.invoices;
CREATE POLICY invoices_staff ON sales.invoices FOR ALL USING (sales.can_see()) WITH CHECK (sales.can_see());
DROP POLICY payments_staff ON sales.payments;
CREATE POLICY payments_staff ON sales.payments FOR ALL USING (sales.can_see()) WITH CHECK (sales.can_see());
DROP POLICY qlines_staff ON sales.quote_lines;
CREATE POLICY qlines_staff ON sales.quote_lines FOR ALL USING (sales.can_see()) WITH CHECK (sales.can_see());
DROP POLICY quotes_staff ON sales.quotes;
CREATE POLICY quotes_staff ON sales.quotes FOR ALL USING (sales.can_see()) WITH CHECK (sales.can_see());
DROP POLICY recurring_staff ON sales.recurring;
CREATE POLICY recurring_staff ON sales.recurring FOR ALL USING (sales.can_see()) WITH CHECK (sales.can_see());
DROP POLICY recurring_runs_staff ON sales.recurring_runs;
CREATE POLICY recurring_runs_staff ON sales.recurring_runs FOR ALL USING (sales.can_see()) WITH CHECK (sales.can_see());

-- Tickets: support, engineers and management work every client's tickets. Anyone else on staff sees only the
-- tickets they raised themselves. Clients still see their own company's.
CREATE FUNCTION support.works_all() RETURNS boolean LANGUAGE sql STABLE AS
  $$ SELECT platform.has_role('ceo') OR platform.has_role('manager') OR platform.has_role('support') OR platform.has_role('engineer') $$;
GRANT EXECUTE ON FUNCTION support.works_all TO hekate_app;

DROP POLICY tickets_isolation ON support.tickets;
CREATE POLICY tickets_staff ON support.tickets FOR ALL
  USING (platform.is_staff() AND (support.works_all() OR reporter = current_setting('app.actor', true)))
  WITH CHECK (platform.is_staff() AND (support.works_all() OR reporter = current_setting('app.actor', true)));
CREATE POLICY tickets_client ON support.tickets FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- Comments follow the ticket: staff see comments on tickets they can see; clients see public ones on their own.
DROP POLICY comments_read ON support.ticket_comments;
CREATE POLICY comments_read ON support.ticket_comments FOR SELECT USING (
  (platform.is_staff() AND EXISTS (SELECT 1 FROM support.tickets t WHERE t.id = ticket_id))
  OR (tenant_id = platform.current_tenant() AND NOT internal));
DROP POLICY comments_write ON support.ticket_comments;
CREATE POLICY comments_write ON support.ticket_comments FOR INSERT WITH CHECK (
  (platform.is_staff() AND EXISTS (SELECT 1 FROM support.tickets t WHERE t.id = ticket_id))
  OR (tenant_id = platform.current_tenant() AND NOT internal));

-- Text documents: who may see or write which kind. Restricted forms (background screening, KYC, penetration test
-- reports) stay with management; HR sees hiring forms, support and engineers see incident forms, and so on.
CREATE FUNCTION documents.narrative_staff(k text, f text) RETURNS boolean LANGUAGE sql STABLE AS $$
  SELECT platform.has_role('ceo') OR platform.has_role('manager')
    OR (platform.has_role('sales') AND k IN ('proposal','order_confirmation','sow','letter','contract','guarantee','certificate'))
    OR (platform.has_role('sales') AND coalesce(f,'') IN ('FRM-06','FRM-07','FRM-08','FRM-09','FRM-10','FRM-11','FRM-23','FRM-24','FRM-26','FRM-31'))
    OR (platform.has_role('hr') AND coalesce(f,'') IN ('FRM-32','FRM-33'))
    OR ((platform.has_role('support') OR platform.has_role('engineer')) AND coalesce(f,'') IN ('FRM-14','FRM-15'))
    OR (platform.has_role('engineer') AND k = 'pentest_report')
    OR (platform.has_role('procurement') AND coalesce(f,'') IN ('FRM-39','FRM-41'))
$$;
GRANT EXECUTE ON FUNCTION documents.narrative_staff TO hekate_app;
DROP POLICY nar_staff ON documents.narratives;
CREATE POLICY nar_staff ON documents.narratives FOR ALL
  USING (platform.is_staff() AND documents.narrative_staff(kind, form_code))
  WITH CHECK (platform.is_staff() AND documents.narrative_staff(kind, form_code));

-- API keys: management and support can see any client's keys and usage; a client sees and revokes their own.
DROP POLICY keys_read ON devapi.keys;
CREATE POLICY keys_read ON devapi.keys FOR SELECT USING (
  platform.has_role('ceo') OR platform.has_role('manager') OR platform.has_role('support') OR tenant_id = platform.current_tenant());
DROP POLICY keys_revoke ON devapi.keys;
CREATE POLICY keys_revoke ON devapi.keys FOR UPDATE
  USING (platform.has_role('ceo') OR platform.has_role('manager') OR (tenant_id = platform.current_tenant() AND platform.has_role('client_admin')))
  WITH CHECK (platform.has_role('ceo') OR platform.has_role('manager') OR (tenant_id = platform.current_tenant() AND platform.has_role('client_admin')));
DROP POLICY usage_read ON devapi.usage;
CREATE POLICY usage_read ON devapi.usage FOR SELECT USING (EXISTS (SELECT 1 FROM devapi.keys k WHERE k.id = usage.key_id));

-- Payslips: the external auditor reviews ledgers and totals, not individual pay. Payroll staff and each employee
-- (once their run is approved) keep access.
DROP POLICY slips_read ON payroll.payslips;
CREATE POLICY slips_read ON payroll.payslips FOR SELECT USING (
  payroll.can_prepare() OR (actor = current_setting('app.actor', true) AND payroll.run_released(run_id)));
