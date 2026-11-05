#!/usr/bin/env python3
"""
Creates the one-tap demo logins for the mobile app on a running HEKATE server.

  * Keycloak users (one per role), already verified, each with a random password.
  * A "Demo Client SARL" organisation with sample tickets and site visits, so every screen has something to show.
  * Prints the HEKATE_DEMO_LOGINS line to add to the API's environment. Restart the API and the buttons appear.

Run it on the server (Python 3 standard library and psql only; safe to run again):

  KC_URL=https://auth.hekatesystems.com KC_ADMIN=admin KC_ADMIN_PASSWORD='...' \
  HEKATE_DATABASE_URL='postgres://...' python3 deploy/demo/create_demo_logins.py

WARNING: staff demo accounts can see every organisation's data, real clients included. Anyone who opens the app
can use them. Turn them off by emptying HEKATE_DEMO_LOGINS (and disable the users in Keycloak) when you are done.
"""
import json, os, secrets, string, subprocess, sys, urllib.parse, urllib.request

KC = os.environ.get("KC_URL", "").rstrip("/")
REALM = os.environ.get("KC_REALM", "hekate")
DB = os.environ.get("HEKATE_DATABASE_URL", "")
DOMAIN = os.environ.get("DEMO_EMAIL_DOMAIN", "demo.hekatesystems.com")
if not (KC and os.environ.get("KC_ADMIN") and os.environ.get("KC_ADMIN_PASSWORD") and DB):
    sys.exit("Set KC_URL, KC_ADMIN, KC_ADMIN_PASSWORD and HEKATE_DATABASE_URL first.")

# label, role, email, staff?
ACCOUNTS = [
    ("Admin", "ceo", "admin", True),
    ("Manager", "manager", "manager", True),
    ("Engineer", "engineer", "engineer", True),
    ("Support", "support", "support", True),
    ("Finance", "finance", "finance", True),
    ("Client admin", "client_admin", "client.admin", False),
    ("Client user", "client_user", "client.user", False),
]


def psql(sql):
    # The tables enforce row-level security even for their owner; act as staff, as the API does for staff calls.
    out = subprocess.run(["psql", "-d", DB, "-v", "ON_ERROR_STOP=1", "-qtAX", "-c", "SET app.role = 'staff'; " + sql], capture_output=True, text=True)
    if out.returncode:
        sys.exit("psql failed: " + out.stderr.strip())
    return out.stdout.strip()


def q(v):
    return "'" + str(v).replace("'", "''") + "'"


def http(method, url, token=None, body=None, form=None):
    data, headers = None, {}
    if form is not None:
        data = urllib.parse.urlencode(form).encode()
        headers["Content-Type"] = "application/x-www-form-urlencoded"
    elif body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(url, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            raw = r.read()
            return r.status, (json.loads(raw) if raw else None), r.headers
    except urllib.error.HTTPError as e:
        return e.code, None, e.headers


def password():
    a = string.ascii_letters + string.digits
    return "Demo-" + "".join(secrets.choice(a) for _ in range(10)) + "!7"


# 1. Organisations: HEKATE's own tenant for staff, and a demo client company.
staff_tenant = psql("SELECT id FROM platform.tenants WHERE kind='hekate' ORDER BY created_at LIMIT 1")
if not staff_tenant:
    sys.exit("No tenant of kind 'hekate' found: create HEKATE's own organisation first.")
client_tenant = psql("SELECT id FROM platform.tenants WHERE name='Demo Client SARL' LIMIT 1") or psql(
    "INSERT INTO platform.tenants (name, kind) VALUES ('Demo Client SARL','client') RETURNING id")
psql(f"""INSERT INTO clients.accounts (tenant_id, legal_name, email, phone, kyc_status)
         VALUES ({q(client_tenant)}, 'Demo Client SARL', 'contact@{DOMAIN}', '+237 600 000 000', 'verified')
         ON CONFLICT (tenant_id) DO NOTHING""")

# 2. Keycloak users.
st, tok, _ = http("POST", f"{KC}/realms/master/protocol/openid-connect/token",
                  form={"grant_type": "password", "client_id": "admin-cli", "username": os.environ["KC_ADMIN"], "password": os.environ["KC_ADMIN_PASSWORD"]})
if st != 200:
    sys.exit(f"Keycloak admin sign-in failed ({st}).")
token = tok["access_token"]
admin = f"{KC}/admin/realms/{REALM}"
logins, ids = [], {}
for label, role, local, staff in ACCOUNTS:
    email, pw = f"{local}@{DOMAIN}", password()
    user = {"username": email, "email": email, "emailVerified": True, "enabled": True,
            "firstName": "Demo", "lastName": label, "requiredActions": [],
            "attributes": {"tenant_id": [staff_tenant if staff else client_tenant]}}
    st, found, _ = http("GET", f"{admin}/users?exact=true&email={urllib.parse.quote(email)}", token)
    if found:
        uid = found[0]["id"]
        http("PUT", f"{admin}/users/{uid}", token, user)
    else:
        st, _, hdr = http("POST", f"{admin}/users", token, user)
        if st != 201:
            sys.exit(f"Could not create {email} ({st}).")
        uid = hdr["Location"].rsplit("/", 1)[1]
    st, _, _ = http("PUT", f"{admin}/users/{uid}/reset-password", token, {"type": "password", "value": pw, "temporary": False})
    if st != 204:
        sys.exit(f"Could not set the password for {email} ({st}).")
    st, rep, _ = http("GET", f"{admin}/roles/{role}", token)
    if st != 200:
        sys.exit(f"Realm role {role} is missing.")
    http("POST", f"{admin}/users/{uid}/role-mappings/realm", token, [rep])
    ids[role] = uid
    logins.append(f"{label}|{email}|{pw}")
    print(f"  {label:<13} {email}")

# 3. Sample work, once: tickets for the demo client and site visits for the demo engineer.
if psql(f"SELECT count(*) FROM support.tickets WHERE tenant_id={q(client_tenant)}") == "0":
    reporter = ids["client_admin"]
    for subj, sev, status, hours in [
        ("Internet down at head office", "P1", "in_progress", 2),
        ("Printer on 2nd floor not reachable", "P3", "open", 20),
        ("New laptop setup for accountant", "P4", "open", 50),
        ("VPN drops every hour", "P2", "resolved", 30),
    ]:
        psql(f"""INSERT INTO support.tickets (tenant_id, subject, description, severity, channel, status, reporter, assignee, respond_due_at, sla_due_at, resolved_at, created_at)
                 VALUES ({q(client_tenant)}, {q(subj)}, 'Demo ticket.', {q(sev)}, 'app', {q(status)}, {q(reporter)},
                         {q(ids['engineer']) if status != 'open' else 'NULL'}, now() + interval '1 hour', now() + interval '{hours} hours',
                         {"now()" if status == 'resolved' else 'NULL'}, now() - interval '3 hours')""")
if psql(f"SELECT count(*) FROM projects.interventions WHERE tenant_id={q(client_tenant)}") == "0":
    for summary, when, addr in [
        ("Replace core switch", "1 day", "Rue Joss, Bonanjo, Douala"),
        ("Router health check", "3 days", "Avenue Kennedy, Yaoundé"),
    ]:
        psql(f"""INSERT INTO projects.interventions (tenant_id, engineer, scheduled_for, summary, site_address, site_contact, created_by)
                 VALUES ({q(client_tenant)}, {q(ids['engineer'])}, now() + interval '{when}', {q(summary)}, {q(addr)}, 'Demo contact, +237 600 000 000', 'demo-setup')""")

print("\nAdd this line to the API environment, then restart the API:\n")
print("HEKATE_DEMO_LOGINS='" + ";".join(logins) + "'")
print("\nTo switch demos off later: empty HEKATE_DEMO_LOGINS, restart, and disable the demo users in Keycloak.")
