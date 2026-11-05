-- Account requests: people without an account ask for one from the sign-in screen; a manager approves or rejects.
-- The public reaches this table only through platform.submit_account_request (SECURITY DEFINER); staff read and decide.
CREATE TABLE platform.account_requests (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name        text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
  email       text NOT NULL CHECK (length(email) <= 254),
  phone       text CHECK (length(phone) <= 40),
  company     text CHECK (length(company) <= 200),
  reason      text NOT NULL DEFAULT '' CHECK (length(reason) <= 1000),
  status      text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
  role        text,
  decided_by  text,
  decided_at  timestamptz,
  note        text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
-- One open request per address: a second submission while one is pending is quietly absorbed.
CREATE UNIQUE INDEX account_requests_one_pending ON platform.account_requests (lower(email)) WHERE status = 'pending';
CREATE INDEX ON platform.account_requests (status, created_at DESC);

ALTER TABLE platform.account_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform.account_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY account_requests_staff ON platform.account_requests FOR ALL USING (platform.is_staff()) WITH CHECK (platform.is_staff());
GRANT SELECT, UPDATE ON platform.account_requests TO hekate_app;

CREATE FUNCTION platform.submit_account_request(p_name text, p_email text, p_phone text, p_company text, p_reason text)
RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path = platform, pg_temp AS $$
  INSERT INTO platform.account_requests (name, email, phone, company, reason)
  VALUES (p_name, lower(p_email), NULLIF(p_phone,''), NULLIF(p_company,''), p_reason)
  ON CONFLICT (lower(email)) WHERE status = 'pending' DO NOTHING
$$;
REVOKE ALL ON FUNCTION platform.submit_account_request FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.submit_account_request TO hekate_app;
