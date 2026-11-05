-- Online payments (mobile money): a payment intent per attempt, and a log of provider callbacks.
-- An intent is created by the client, but only a verified provider answer (a signed callback or a
-- status lookup at the provider) settles it, and that runs as a trusted system actor.
CREATE SCHEMA IF NOT EXISTS payments;

CREATE TABLE payments.intents (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES platform.tenants(id),
  invoice_id   uuid NOT NULL REFERENCES sales.invoices(id),
  amount       bigint NOT NULL CHECK (amount > 0),
  provider     text NOT NULL CHECK (provider IN ('momo','orange_money')),
  msisdn       text NOT NULL CHECK (msisdn ~ '^[0-9]{8,15}$'),
  status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','succeeded','failed','expired','review')),
  provider_ref text,
  reason       text NOT NULL DEFAULT '',
  created_by   text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX one_pending_per_invoice ON payments.intents (invoice_id) WHERE status = 'pending';
CREATE INDEX ON payments.intents (tenant_id, created_at DESC);

CREATE TABLE payments.events (
  id         bigserial PRIMARY KEY,
  provider   text NOT NULL,
  event_id   text NOT NULL,
  intent_id  uuid,
  outcome    text NOT NULL,
  payload    jsonb NOT NULL,
  received_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, event_id)
);

-- A settled intent is a record of fact; a pending or expired one may still be settled.
CREATE FUNCTION payments.guard_intent() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.tenant_id <> OLD.tenant_id OR NEW.invoice_id <> OLD.invoice_id OR NEW.amount <> OLD.amount OR NEW.provider <> OLD.provider OR NEW.msisdn <> OLD.msisdn OR NEW.created_by <> OLD.created_by THEN
    RAISE EXCEPTION 'a payment intent cannot be edited';
  END IF;
  IF OLD.status IN ('succeeded','failed','review') AND NEW.status <> OLD.status THEN
    RAISE EXCEPTION 'a settled payment intent cannot change status';
  END IF;
  NEW.updated_at := now();
  RETURN NEW;
END $$;
CREATE TRIGGER intent_guard BEFORE UPDATE ON payments.intents FOR EACH ROW EXECUTE FUNCTION payments.guard_intent();

ALTER TABLE payments.intents ENABLE ROW LEVEL SECURITY; ALTER TABLE payments.intents FORCE ROW LEVEL SECURITY;
ALTER TABLE payments.events ENABLE ROW LEVEL SECURITY;  ALTER TABLE payments.events FORCE ROW LEVEL SECURITY;
CREATE POLICY intents_staff  ON payments.intents FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
CREATE POLICY intents_client_read ON payments.intents FOR SELECT USING (tenant_id = platform.current_tenant());
CREATE POLICY intents_client_new  ON payments.intents FOR INSERT WITH CHECK (
  tenant_id = platform.current_tenant() AND created_by = current_setting('app.actor', true) AND status = 'pending');
CREATE POLICY events_staff ON payments.events FOR ALL USING (platform.is_staff() AND platform.has_role('finance')) WITH CHECK (platform.is_staff() AND platform.has_role('finance'));

GRANT USAGE ON SCHEMA payments TO hekate_app;
GRANT SELECT, INSERT, UPDATE ON payments.intents TO hekate_app;
GRANT SELECT, INSERT ON payments.events TO hekate_app;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA payments TO hekate_app;
