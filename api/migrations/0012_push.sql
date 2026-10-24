-- Push delivery: devices remember the roles of the user who registered them, and the notifier looks
-- recipients up through a definer function (device rows are otherwise visible only to their owner).
ALTER TABLE notify.devices ADD COLUMN roles text[] NOT NULL DEFAULT '{}';

DROP FUNCTION notify.register_device(text, text, uuid, text);
CREATE FUNCTION notify.register_device(p_token text, p_actor text, p_tenant uuid, p_platform text, p_roles text[])
RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path = notify, pg_temp AS $$
  INSERT INTO notify.devices (token, actor, tenant_id, platform, roles) VALUES (p_token, p_actor, p_tenant, p_platform, p_roles)
  ON CONFLICT (token) DO UPDATE SET actor = EXCLUDED.actor, tenant_id = EXCLUDED.tenant_id,
    platform = EXCLUDED.platform, roles = EXCLUDED.roles, last_seen = now()
$$;
REVOKE ALL ON FUNCTION notify.register_device FROM PUBLIC;
GRANT EXECUTE ON FUNCTION notify.register_device TO hekate_app;

-- Tokens to notify: optionally narrowed to one user, one tenant and/or any of some roles.
CREATE FUNCTION notify.tokens_for(p_actor text, p_tenant uuid, p_roles text[])
RETURNS TABLE (token text, actor text) LANGUAGE sql STABLE SECURITY DEFINER SET search_path = notify, pg_temp AS $$
  SELECT d.token, d.actor FROM notify.devices d
  WHERE (p_actor IS NULL OR d.actor = p_actor)
    AND (p_tenant IS NULL OR d.tenant_id = p_tenant)
    AND (cardinality(p_roles) = 0 OR d.roles && p_roles)
$$;
REVOKE ALL ON FUNCTION notify.tokens_for FROM PUBLIC;
GRANT EXECUTE ON FUNCTION notify.tokens_for TO hekate_app;

-- Dead tokens (the push service says the app was uninstalled) are removed by the notifier.
CREATE FUNCTION notify.forget_token(p_token text) RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path = notify, pg_temp AS $$
  DELETE FROM notify.devices WHERE token = p_token
$$;
REVOKE ALL ON FUNCTION notify.forget_token FROM PUBLIC;
GRANT EXECUTE ON FUNCTION notify.forget_token TO hekate_app;
