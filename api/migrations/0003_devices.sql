-- Push notification device tokens (FCM/APNs/Expo). One row per device; a user can have several.
CREATE SCHEMA IF NOT EXISTS notify;
CREATE TABLE notify.devices (
  token      text PRIMARY KEY,
  actor      text NOT NULL,
  tenant_id  uuid,
  platform   text NOT NULL CHECK (platform IN ('ios','android','expo')),
  created_at timestamptz NOT NULL DEFAULT now(),
  last_seen  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON notify.devices (actor);
ALTER TABLE notify.devices ENABLE ROW LEVEL SECURITY;
-- A device row is visible and writable only to the user who registered it.
CREATE POLICY devices_owner ON notify.devices
  USING (actor = current_setting('app.actor', true))
  WITH CHECK (actor = current_setting('app.actor', true));
-- Registration must be able to take over a token from a previous user (device handed on after
-- sign-out), which row-level security would hide, so it goes through a definer function. The
-- table is deliberately not FORCE RLS so the owning role can run it.
CREATE FUNCTION notify.register_device(p_token text, p_actor text, p_tenant uuid, p_platform text)
RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path = notify, pg_temp AS $$
  INSERT INTO notify.devices (token, actor, tenant_id, platform) VALUES (p_token, p_actor, p_tenant, p_platform)
  ON CONFLICT (token) DO UPDATE SET actor = EXCLUDED.actor, tenant_id = EXCLUDED.tenant_id,
    platform = EXCLUDED.platform, last_seen = now()
$$;
REVOKE ALL ON FUNCTION notify.register_device FROM PUBLIC;
GRANT EXECUTE ON FUNCTION notify.register_device TO hekate_app;
GRANT USAGE ON SCHEMA notify TO hekate_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON notify.devices TO hekate_app;
