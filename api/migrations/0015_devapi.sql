-- API as a product: plans, tenant-scoped read-only API keys and monthly usage metering.
-- Keys are stored only as a SHA-256 hash; the secret is shown once at creation.
CREATE SCHEMA IF NOT EXISTS devapi;

CREATE TABLE devapi.plans (
  code          text PRIMARY KEY CHECK (code ~ '^[a-z][a-z0-9_-]{1,30}$'),
  name          text NOT NULL,
  rate_per_min  integer NOT NULL CHECK (rate_per_min > 0),
  monthly_quota bigint NOT NULL CHECK (monthly_quota > 0),
  price_xaf     bigint NOT NULL DEFAULT 0 CHECK (price_xaf >= 0),
  active        boolean NOT NULL DEFAULT true
);
INSERT INTO devapi.plans (code, name, rate_per_min, monthly_quota, price_xaf) VALUES
  ('free', 'Free', 60, 10000, 0),
  ('standard', 'Standard', 300, 200000, 0);

CREATE TABLE devapi.keys (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES platform.tenants(id),
  name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
  prefix       text NOT NULL,
  hash         text NOT NULL UNIQUE,
  scopes       text[] NOT NULL CHECK (cardinality(scopes) > 0),
  plan         text NOT NULL REFERENCES devapi.plans(code),
  created_by   text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  expires_at   timestamptz,
  revoked_at   timestamptz,
  last_used_at timestamptz
);
CREATE INDEX ON devapi.keys (tenant_id);

CREATE TABLE devapi.usage (
  key_id uuid NOT NULL REFERENCES devapi.keys(id),
  month  date NOT NULL,
  calls  bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (key_id, month)
);

ALTER TABLE devapi.plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE devapi.keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE devapi.usage ENABLE ROW LEVEL SECURITY;
CREATE POLICY plans_read ON devapi.plans FOR SELECT USING (true);
CREATE POLICY plans_write ON devapi.plans FOR ALL
  USING (platform.has_role('ceo') OR platform.has_role('manager'))
  WITH CHECK (platform.has_role('ceo') OR platform.has_role('manager'));
CREATE POLICY keys_read ON devapi.keys FOR SELECT USING (platform.is_staff() OR tenant_id = platform.current_tenant());
CREATE POLICY keys_insert ON devapi.keys FOR INSERT WITH CHECK (tenant_id = platform.current_tenant() AND platform.has_role('client_admin'));
CREATE POLICY keys_revoke ON devapi.keys FOR UPDATE
  USING (platform.is_staff() OR (tenant_id = platform.current_tenant() AND platform.has_role('client_admin')))
  WITH CHECK (platform.is_staff() OR (tenant_id = platform.current_tenant() AND platform.has_role('client_admin')));
CREATE POLICY usage_read ON devapi.usage FOR SELECT USING (
  EXISTS (SELECT 1 FROM devapi.keys k WHERE k.id = key_id AND (platform.is_staff() OR k.tenant_id = platform.current_tenant())));

-- A key may only be revoked, never edited: everything but revoked_at is frozen.
CREATE FUNCTION devapi.keys_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.id <> OLD.id OR NEW.tenant_id <> OLD.tenant_id OR NEW.hash <> OLD.hash OR NEW.scopes <> OLD.scopes
     OR NEW.name <> OLD.name OR NEW.created_by <> OLD.created_by OR NEW.prefix <> OLD.prefix THEN
    RAISE EXCEPTION 'api keys can only be revoked or have their plan changed';
  END IF;
  IF NEW.plan <> OLD.plan AND NOT platform.is_staff() THEN
    RAISE EXCEPTION 'only staff can change an api key plan';
  END IF;
  IF OLD.revoked_at IS NOT NULL AND NEW.revoked_at IS DISTINCT FROM OLD.revoked_at THEN
    RAISE EXCEPTION 'api key already revoked';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER keys_guard BEFORE UPDATE ON devapi.keys FOR EACH ROW EXECUTE FUNCTION devapi.keys_guard();

-- Authentication happens before any tenant is known, so it goes through definer functions.
CREATE FUNCTION devapi.authenticate(p_hash text)
RETURNS TABLE (key_id uuid, tenant_id uuid, scopes text[], rate_per_min integer, monthly_quota bigint)
LANGUAGE sql SECURITY DEFINER SET search_path = devapi, pg_temp AS $$
  SELECT k.id, k.tenant_id, k.scopes, p.rate_per_min, p.monthly_quota
  FROM devapi.keys k JOIN devapi.plans p ON p.code = k.plan
  WHERE k.hash = p_hash AND k.revoked_at IS NULL AND (k.expires_at IS NULL OR k.expires_at > now()) AND p.active
$$;
REVOKE ALL ON FUNCTION devapi.authenticate FROM PUBLIC;
GRANT EXECUTE ON FUNCTION devapi.authenticate TO hekate_app;

-- Counts one call against this month and returns the month's total so far.
CREATE FUNCTION devapi.meter(p_key uuid) RETURNS bigint LANGUAGE sql SECURITY DEFINER SET search_path = devapi, pg_temp AS $$
  WITH u AS (
    INSERT INTO devapi.usage (key_id, month, calls) VALUES (p_key, date_trunc('month', now())::date, 1)
    ON CONFLICT (key_id, month) DO UPDATE SET calls = devapi.usage.calls + 1
    RETURNING calls
  ), t AS (UPDATE devapi.keys SET last_used_at = now() WHERE id = p_key)
  SELECT calls FROM u
$$;
REVOKE ALL ON FUNCTION devapi.meter FROM PUBLIC;
GRANT EXECUTE ON FUNCTION devapi.meter TO hekate_app;

GRANT USAGE ON SCHEMA devapi TO hekate_app;
GRANT SELECT, INSERT, UPDATE ON devapi.plans TO hekate_app;
GRANT SELECT, INSERT ON devapi.keys TO hekate_app;
GRANT UPDATE (revoked_at, plan) ON devapi.keys TO hekate_app;
GRANT SELECT ON devapi.usage TO hekate_app;
