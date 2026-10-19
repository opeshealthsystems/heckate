-- Phase 0 baseline: per-module schemas, tenancy, audit log, outbox, and the
-- Support & Incidents tickets table with row-level security.
-- The API connects as role hekate_app (no BYPASSRLS) and sets
--   SET LOCAL app.tenant_id = '<uuid>';  SET LOCAL app.role = 'staff'|'client';
-- at the start of every transaction.

CREATE SCHEMA IF NOT EXISTS platform;
CREATE SCHEMA IF NOT EXISTS support;

DO $$ BEGIN
  CREATE ROLE hekate_app NOLOGIN;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE platform.tenants (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Every create/change/delete/login lands here (sent on to HCDP). Append-only.
CREATE TABLE platform.audit_log (
  id         bigserial PRIMARY KEY,
  at         timestamptz NOT NULL DEFAULT now(),
  tenant_id  uuid,
  actor      text NOT NULL,
  action     text NOT NULL,
  entity     text NOT NULL,
  entity_id  text,
  trace_id   text
);

-- Transactional outbox, relayed to NATS JetStream (ticket.created, invoice.paid, ...).
CREATE TABLE platform.outbox (
  id           bigserial PRIMARY KEY,
  created_at   timestamptz NOT NULL DEFAULT now(),
  subject      text NOT NULL,
  payload      jsonb NOT NULL,
  published_at timestamptz
);

CREATE TABLE support.tickets (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES platform.tenants(id),
  subject     text NOT NULL,
  description text NOT NULL DEFAULT '',
  severity    text NOT NULL CHECK (severity IN ('P1','P2','P3','P4')),
  channel     text NOT NULL CHECK (channel IN ('app','portal','email','phone')),
  status      text NOT NULL DEFAULT 'open' CHECK (status IN ('open','in_progress','resolved','closed')),
  sla_due_at  timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE support.tickets ENABLE ROW LEVEL SECURITY;
ALTER TABLE support.tickets FORCE ROW LEVEL SECURITY;

-- Clients see only their own tenant's rows; staff see all.
CREATE POLICY tickets_isolation ON support.tickets
  USING (
    current_setting('app.role', true) = 'staff'
    OR tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
  )
  WITH CHECK (
    current_setting('app.role', true) = 'staff'
    OR tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
  );

GRANT USAGE ON SCHEMA platform, support TO hekate_app;
GRANT SELECT, INSERT, UPDATE ON support.tickets TO hekate_app;
GRANT INSERT ON platform.audit_log, platform.outbox TO hekate_app;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA platform TO hekate_app;
