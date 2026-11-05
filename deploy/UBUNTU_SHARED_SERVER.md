# Deploying HEKATE on a shared Ubuntu server (no Docker)

For the agent or engineer doing the deployment. **Status: written but never run.** Nothing here has been executed on a real server, so treat each step as a proposal, verify it against what you find, and report anything that differs. Read `deploy/production.md` first for the overview and the domain table.

The server is shared with **opesware.com**. The first rule is to leave everything that belongs to opesware.com exactly as it is.

## Rules for this server

1. **Look before you change.** Do the read-only survey in section 1 and show the owner the results and your plan before running anything that changes the server.
2. **Do not modify, restart, reload or remove anything belonging to opesware.com**: its sites, Nginx server blocks, databases, users, services, cron jobs, certificates, firewall rules or files. Adding a new Nginx `server` block and reloading Nginx is allowed only after `nginx -t` passes; if Nginx also serves opesware.com, reload (never restart) and check opesware.com still answers afterwards.
3. **No Docker.** Everything runs as systemd services under a dedicated user.
4. **Secrets never go in the repository, in chat, or in shell history.** Generate them on the server, store them in files readable only by the HEKATE service user, and tell the owner where they are.
5. **Ask the owner before** opening firewall ports, installing large packages (Java, Keycloak), creating DNS records, or anything you cannot undo.
6. **Staging first.** If possible bring the stack up on a staging name (for example `staging-app.hekatesystems.com`) and run section 10 before pointing the real names at it.

## Target layout

| Host | Serves | Internal port (loopback only) |
| --- | --- | --- |
| `hekatesystems.com`, `www.hekatesystems.com` | Public site (static files) | none (Nginx serves files) |
| `app.hekatesystems.com` | ERP, client portal, `/verify`, QR links | web `127.0.0.1:13000` |
| `api.hekatesystems.com` | Go API | API `127.0.0.1:18080`, metrics `127.0.0.1:19090` |
| `auth.hekatesystems.com` | Keycloak (login) | `127.0.0.1:18081` |

The ports are suggestions chosen to avoid common clashes. Check they are free (section 1) and change them consistently if not. Only Nginx listens on public ports.

Install location: `/opt/hekate` (code and build output), `/etc/hekate` (settings and secrets), `/var/lib/hekate` (data), service user `hekate`.

## 1. Survey the server (read-only)

Run and record the output:

```bash
lsb_release -a; uname -r
nproc; free -h; df -h
ss -ltnp                           # which ports are already used
systemctl list-units --type=service --state=running
nginx -v 2>&1; ls /etc/nginx/sites-enabled /etc/nginx/conf.d 2>/dev/null
psql --version 2>&1; systemctl status postgresql --no-pager 2>&1 | head -5
node -v 2>&1; go version 2>&1; java -version 2>&1
ufw status 2>&1; id hekate 2>&1
```

Decide from the results:

- **Memory.** The API, the ERP (Node), Keycloak (Java) and NATS together want roughly 2 GB free on top of what opesware.com uses. If the server is small, tell the owner before continuing; a hosted OIDC provider is the usual way to avoid running Keycloak here.
- **PostgreSQL.** Version 16 or newer is required. If an older Postgres already serves opesware.com, do **not** upgrade or replace it; install a second cluster on another port (`pg_createcluster`) or ask the owner.
- **Nginx.** Use the existing Nginx if present. Do not change its global settings.

## 2. DNS and certificates

The owner must create A (and AAAA if used) records for `hekatesystems.com`, `www`, `app`, `api` and `auth` pointing at the server. Confirm with `dig +short app.hekatesystems.com`. Issue certificates with certbot for the new names only, for example:

```bash
sudo certbot certonly --nginx -d hekatesystems.com -d www.hekatesystems.com
sudo certbot certonly --nginx -d app.hekatesystems.com
sudo certbot certonly --nginx -d api.hekatesystems.com
sudo certbot certonly --nginx -d auth.hekatesystems.com
```

Do not touch existing certificates.

## 3. User and directories

```bash
sudo useradd --system --create-home --home-dir /var/lib/hekate --shell /usr/sbin/nologin hekate
sudo mkdir -p /opt/hekate /etc/hekate /etc/hekate/secrets /var/www/hekate-site
sudo chown -R hekate:hekate /opt/hekate /var/lib/hekate
sudo chown root:hekate /etc/hekate /etc/hekate/secrets
sudo chmod 750 /etc/hekate /etc/hekate/secrets
```

## 4. Get the code

```bash
sudo -u hekate git clone https://github.com/opeshealthsystems/heckate.git /opt/hekate/src
cd /opt/hekate/src && sudo -u hekate git checkout main && git log --oneline -1
```

Record the commit you deploy. To update later: `git pull --ff-only` on `main`, rebuild, restart (section 11).

## 5. PostgreSQL

Create a dedicated, **non-superuser** owner and database. The API depends on PostgreSQL row-level security, and production must not run as a superuser (the test suite runs both ways; `docs/SECURITY.md` explains why). The owner needs `CREATEROLE` because the first migration creates the `hekate_app` role.

```bash
HEKATE_DB_PASSWORD=$(openssl rand -base64 32 | tr -d '/+=' | cut -c1-32)
sudo -u postgres psql <<SQL
CREATE ROLE hekate LOGIN PASSWORD '${HEKATE_DB_PASSWORD}' CREATEROLE NOSUPERUSER NOBYPASSRLS;
CREATE DATABASE hekate OWNER hekate;
SQL
```

Keep the password for `/etc/hekate/api.env` (section 7). The API refuses to start in production if the database URL contains `sslmode=disable`; it does not reject other modes. Ubuntu's PostgreSQL has TLS switched on by default with a self-signed certificate, so for a database on the same machine use `sslmode=require` (verify with `sudo -u postgres psql -c "SHOW ssl;"`). Use `sslmode=verify-full` with a proper certificate if the database is ever on another host. Start the API once with the real settings: it lists every unsafe setting it finds, so fix those rather than working around the checks.

## 6. NATS

The API uses NATS (JetStream) for its event stream. Install the `nats-server` release binary or package, run it under its own user listening on `127.0.0.1:4222` with JetStream enabled (`-js`) and a store directory under `/var/lib/nats`, and set `HEKATE_NATS_URL=nats://127.0.0.1:4222`. If NATS is unset the API runs without the event stream; confirm with the owner whether that is acceptable.

## 7. Build and run the API

Requires Go 1.26.x (`api/go.mod` pins toolchain `go1.26.10`; if the system Go is older, install the matching toolchain from go.dev into `/opt/go` and use it for this build only).

```bash
cd /opt/hekate/src/api
sudo -u hekate CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$(git rev-parse --short HEAD)" -o /opt/hekate/bin/api ./cmd/api
```

Settings go in `/etc/hekate/api.env` (mode 640, owner `root:hekate`). Start from `deploy/production.env.example` and set at least:

```
HEKATE_ENV=production
HEKATE_ADDR=127.0.0.1:18080
HEKATE_DATABASE_URL=postgres://hekate:<password>@127.0.0.1:5432/hekate?sslmode=<see section 5>
HEKATE_JWKS_URL=https://auth.hekatesystems.com/realms/hekate/protocol/openid-connect/certs
HEKATE_ISSUER=https://auth.hekatesystems.com/realms/hekate
HEKATE_AUDIENCE=hekate-api
HEKATE_CORS_ORIGINS=https://app.hekatesystems.com,https://hekatesystems.com,https://www.hekatesystems.com
HEKATE_PORTAL_URL=https://app.hekatesystems.com
HEKATE_TRUSTED_PROXIES=127.0.0.1/32
HEKATE_METRICS_ADDR=127.0.0.1:19090
HEKATE_NATS_URL=nats://127.0.0.1:4222
```

Optional and off until the owner supplies them: `HEKATE_QR_SIGNING_KEY_FILE` (signed QR codes; generate an Ed25519 PKCS#8 PEM into `/etc/hekate/secrets/`, back it up, and never change it casually because it signs every QR already issued), `HEKATE_PDF_SIGNING_CERT_FILE` and `HEKATE_PDF_SIGNING_KEY_FILE` (set both or neither), and the MTN and Orange Money settings (leave unset; payments stay disabled). Never set `HEKATE_DEV_JWT_SECRET` or `HEKATE_PAYMENTS_SANDBOX` in production.

`/etc/systemd/system/hekate-api.service`:

```ini
[Unit]
Description=HEKATE API
After=network.target postgresql.service

[Service]
User=hekate
Group=hekate
EnvironmentFile=/etc/hekate/api.env
ExecStart=/opt/hekate/bin/api
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/var/lib/hekate

[Install]
WantedBy=multi-user.target
```

The API applies its migrations on start; `/opt/hekate/bin/api migrate` runs them alone and is a good first check. Then `sudo systemctl daemon-reload && sudo systemctl enable --now hekate-api` and `curl -s http://127.0.0.1:18080/readyz`.

### First company record (one-time, manual)

There is no setup command. The HEKATE company must exist as a tenant row, and its id becomes the `tenant_id` of every staff login. As the database owner:

```bash
sudo -u postgres psql hekate -c "INSERT INTO platform.tenants (name, kind) VALUES ('HEKATE SYSTEMS SARL', 'hekate') RETURNING id;"
```

(If the insert is refused by row-level security, run it as the `hekate` owner with the session variables the migrations expect, or ask the owner; report what you did.) Keep the returned id for section 9. Do not invent any other company data: leave unknown company details blank.

## 8. Build and run the ERP web app and the public site

Node 22 is required. The `NEXT_PUBLIC_*` values are compiled in at build time, so they must be set when building:

```bash
cd /opt/hekate/src/web
sudo -u hekate npm ci
sudo -u hekate env NEXT_PUBLIC_API_URL=https://api.hekatesystems.com \
  NEXT_PUBLIC_OIDC_ISSUER=https://auth.hekatesystems.com/realms/hekate \
  NEXT_PUBLIC_OIDC_CLIENT_ID=hekate-web NEXT_PUBLIC_DEV_LOGIN=0 npm run build
sudo -u hekate mkdir -p /opt/hekate/web
sudo -u hekate cp -r .next/standalone/. /opt/hekate/web/
sudo -u hekate mkdir -p /opt/hekate/web/.next && sudo -u hekate cp -r .next/static /opt/hekate/web/.next/static
sudo -u hekate cp -r public /opt/hekate/web/public
```

`/etc/systemd/system/hekate-web.service`:

```ini
[Unit]
Description=HEKATE ERP web
After=network.target

[Service]
User=hekate
Group=hekate
WorkingDirectory=/opt/hekate/web
Environment=NODE_ENV=production PORT=13000 HOSTNAME=127.0.0.1
ExecStart=/usr/bin/node server.js
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

The public site is a static export:

```bash
cd /opt/hekate/src/site
sudo -u hekate npm ci
sudo -u hekate env NEXT_PUBLIC_SITE_URL=https://hekatesystems.com \
  NEXT_PUBLIC_API_URL=https://api.hekatesystems.com \
  NEXT_PUBLIC_PORTAL_URL=https://app.hekatesystems.com npm run build
sudo rsync -a --delete out/ /var/www/hekate-site/
```

## 9. Login (Keycloak or another OIDC provider)

The API trusts tokens from `HEKATE_ISSUER`, signed with the keys at `HEKATE_JWKS_URL`, whose audience includes `HEKATE_AUDIENCE`. Tokens must carry a `tenant_id` claim and a `roles` claim. `deploy/keycloak/hekate-realm.json` defines the realm, roles, the claim mappers and two public clients, and **needs these changes before production use**:

- **Redirect addresses.** `hekate-web` currently allows only `http://localhost:3000/...`. Add `https://app.hekatesystems.com/auth/callback` (redirect), `https://app.hekatesystems.com` (web origin) and `https://app.hekatesystems.com/login` (post-logout) and remove the localhost entries. `hekate-mobile` uses the redirect `hekate://auth`.
- **Audience.** The realm file has no audience mapper. Add one so tokens carry `hekate-api`, or set `HEKATE_AUDIENCE` to what the tokens actually contain. The API rejects tokens with the wrong audience.
- **No users are defined.** Create the first administrator accounts by hand. Each staff user needs the user attribute `tenant_id` set to the HEKATE tenant id from section 7 and the right realm roles (`ceo`, `manager`, `finance`, and so on). Never create a user without a `tenant_id`.
- Keep `sslRequired` at `external` or stricter, set strong admin credentials, and turn on required MFA for staff.

Run Keycloak 26 on Java 21 behind Nginx at `auth.hekatesystems.com` (`KC_HOSTNAME=https://auth.hekatesystems.com`, `KC_PROXY_HEADERS=xforwarded`, `KC_HTTP_ENABLED=true`, listening on `127.0.0.1:18081`, its own database and role in PostgreSQL). Import the edited realm with `--import-realm`. If this is too heavy for the server, use another OIDC provider that can emit the same claims and set the issuer and JWKS addresses accordingly.

## 10. Nginx

Add four new files in `/etc/nginx/sites-available/` (linked into `sites-enabled/`), without changing existing ones. Shared proxy headers for the three proxied hosts:

```nginx
# api.hekatesystems.com
server {
  listen 443 ssl; http2 on;
  server_name api.hekatesystems.com;
  ssl_certificate     /etc/letsencrypt/live/api.hekatesystems.com/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/api.hekatesystems.com/privkey.pem;
  client_max_body_size 25m;
  location / {
    proxy_pass http://127.0.0.1:18080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $remote_addr;
    proxy_set_header X-Forwarded-Proto $scheme;
  }
}
server { listen 80; server_name api.hekatesystems.com; return 301 https://$host$request_uri; }
```

Use the same pattern for `app.hekatesystems.com` (to `127.0.0.1:13000`) and `auth.hekatesystems.com` (to `127.0.0.1:18081`, with `proxy_buffer_size 128k; proxy_buffers 4 256k;`). Do not expose the metrics port. Use `X-Forwarded-For $remote_addr` (not `$proxy_add_x_forwarded_for`) so the API's trusted-proxy handling sees the real client address.

The public site is plain files:

```nginx
server {
  listen 443 ssl; http2 on;
  server_name hekatesystems.com www.hekatesystems.com;
  ssl_certificate     /etc/letsencrypt/live/hekatesystems.com/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/hekatesystems.com/privkey.pem;
  root /var/www/hekate-site;
  index index.html;
  add_header X-Content-Type-Options nosniff always;
  add_header X-Frame-Options DENY always;
  add_header Referrer-Policy strict-origin-when-cross-origin always;
  location / { try_files $uri $uri/ $uri.html =404; }
  location /_next/static/ { expires 1y; add_header Cache-Control "public, immutable"; }
  error_page 404 /404.html;
}
server { listen 80; server_name hekatesystems.com www.hekatesystems.com; return 301 https://hekatesystems.com$request_uri; }
```

Then `sudo nginx -t` and, only if it passes, `sudo systemctl reload nginx`, and confirm opesware.com still responds.

## 11. Verify

1. `curl -s https://api.hekatesystems.com/readyz` succeeds.
2. `https://hekatesystems.com` loads in English and French; its "Client portal" and "Verify a document" links go to `app.hekatesystems.com`.
3. `https://app.hekatesystems.com/login` redirects to `auth.hekatesystems.com`, a staff user signs in and returns to the app, and `/verify` loads without signing in.
4. Create a test client record, then check that a client-role user cannot see another client's data.
5. Run `/opt/hekate/bin/api verify-data` (exit 0 means the ledger and audit hash chains are intact).
6. `sudo journalctl -u hekate-api -u hekate-web -n 100` shows no errors.
7. opesware.com still works.

Update procedure: `git pull --ff-only`, rebuild what changed (sections 7 and 8), `sudo systemctl restart hekate-api hekate-web`, rerun the checks above. The API applies new migrations on start; take a database dump first.

## 12. Backups and rollback

- Before the first start and before every update: `sudo -u postgres pg_dump -Fc hekate > /var/backups/hekate-$(date +%F).dump`. `deploy/backup/` has `backup.sh`, `restore.sh` and `drill.sh` (nightly dump, restore and restore drill) to adapt. Copy backups off the server; backups only on the same machine are not a backup plan.
- Rollback: keep the previous binary (`cp /opt/hekate/bin/api /opt/hekate/bin/api.prev`) and the previous web build; `systemctl stop`, swap back, `start`. Database migrations are forward-only, so a rollback after a migration means restoring the pre-update dump.

## 13. What is not done (tell the owner)

No penetration test; no off-site backup automation; no monitoring or alerting (`deploy/monitoring/alerts.yml` has example rules); Keycloak realm needs the edits in section 9; MTN and Orange Money payments are disabled; the PDF signing certificate and the QR signing key have not been provided; the open items in `docs/BLOCKERS.md` still need the owner's answers. Report these honestly instead of describing the deployment as finished.
