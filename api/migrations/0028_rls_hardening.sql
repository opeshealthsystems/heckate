-- Close the tables that had no row-level security. No endpoint reads these for a caller, but a bug or an
-- injection in any request would otherwise have been able to. Each rule keeps exactly what the code needs.

-- Event outbox: any request may append an event; only the background relay (a connection with no request
-- role set) may read or mark events, so a request can never read another tenant's event payloads.
ALTER TABLE platform.outbox ENABLE ROW LEVEL SECURITY; ALTER TABLE platform.outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY outbox_append ON platform.outbox FOR INSERT WITH CHECK (true);
CREATE POLICY outbox_relay_read ON platform.outbox FOR SELECT USING (coalesce(current_setting('app.role', true), '') = '');
CREATE POLICY outbox_relay_mark ON platform.outbox FOR UPDATE USING (coalesce(current_setting('app.role', true), '') = '') WITH CHECK (coalesce(current_setting('app.role', true), '') = '');

-- Idempotency keys: a caller can only see and store their own (anonymous callers share one actor name).
ALTER TABLE platform.idempotency_keys ENABLE ROW LEVEL SECURITY; ALTER TABLE platform.idempotency_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY idem_own_read ON platform.idempotency_keys FOR SELECT USING (actor = coalesce(nullif(current_setting('app.actor', true), ''), 'anonymous'));
CREATE POLICY idem_own_write ON platform.idempotency_keys FOR INSERT WITH CHECK (actor = coalesce(nullif(current_setting('app.actor', true), ''), 'anonymous'));

-- Reference-number counters: only staff issue documents.
ALTER TABLE documents.counters ENABLE ROW LEVEL SECURITY; ALTER TABLE documents.counters FORCE ROW LEVEL SECURITY;
CREATE POLICY counters_staff ON documents.counters FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());

-- Sales settings hold the tax rates, which clients need to see the totals of their own quotes, and the invoice
-- approval threshold. Any signed-in caller may read the row (not anonymous or public callers). The threshold is
-- therefore not secret from a signed-in client who can run raw SQL; splitting it out is on the gap register.
ALTER TABLE sales.settings ENABLE ROW LEVEL SECURITY; ALTER TABLE sales.settings FORCE ROW LEVEL SECURITY;
CREATE POLICY sales_settings_signed_in ON sales.settings FOR SELECT USING (coalesce(current_setting('app.role', true), '') NOT IN ('', 'public'));

-- Procurement settings (the purchase-order approval threshold): staff only.
ALTER TABLE procurement.settings ENABLE ROW LEVEL SECURITY; ALTER TABLE procurement.settings FORCE ROW LEVEL SECURITY;
CREATE POLICY procurement_settings_staff ON procurement.settings FOR SELECT USING (platform.is_staff());

-- Service-level times are promised to clients in the guarantee policies, so everyone signed in may read them.
ALTER TABLE support.sla_policies ENABLE ROW LEVEL SECURITY; ALTER TABLE support.sla_policies FORCE ROW LEVEL SECURITY;
CREATE POLICY sla_policies_read ON support.sla_policies FOR SELECT USING (true);

-- Audit log: requests may only append. It is read and sealed by SECURITY DEFINER functions owned by the
-- schema owner, so it is not FORCEd (the owner must still reach it); the application role has INSERT only.
ALTER TABLE platform.audit_log ENABLE ROW LEVEL SECURITY;
CREATE POLICY audit_append ON platform.audit_log FOR INSERT WITH CHECK (true);
