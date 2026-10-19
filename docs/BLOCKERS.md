# Blockers and decisions for the owner

Things I could not do or decide alone. Each says what is needed. Items marked **untested** were written but could not be run in the build sandbox.

## Needs a decision

| Decision | Why it matters | What I assumed meanwhile |
| --- | --- | --- |
| SLA response and resolution times for P1–P4 | They drive every ticket's due date, breach flag and the future service credits | Placeholders in `support.sla_policies`: P1 15 min / 4 h, P2 1 h / 8 h, P3 4 h / 24 h, P4 8 h / 72 h. Change the rows, no code change |
| Platform name | Appears in UI, app store listing, emails | "HEKATE" |
| Mobile payment providers first (MTN MoMo, Orange Money, card) | Need merchant accounts and API credentials before any integration | Payment tracking records are manual; no gateway code |
| App languages at launch | Website has EN and FR; the web app and mobile app are English only | English only in the app |
| Who approves code to production (two named people) | Required by the security section | Not enforced; set branch protection with two required reviewers on `main` |
| Host on own servers only, or add a cloud copy | Changes deployment and backup design | Docker/compose files work for either |

## Needs information only the company has

- **Company facts for the public site:** RCCM number, street address, phone numbers, emergency line, founders' names. Fill `site/content/company.ts`; empty fields are left out of structured data and the About page on purpose rather than guessed.
- **French copy** on the site and the service FAQs was written by me and needs a native review before launch.
- **Legal and contract text** (NDA, MSA/SLA, DPA, pentest authorisation, employment contract, screening consent) and the **Guarantee Policies (HS-GP)** are not in the repo; the document register only issues references for them.

## Needs credentials or accounts

- **Keycloak:** `deploy/keycloak/hekate-realm.json` defines roles, the `tenant_id` claim and the web and mobile clients. **Untested** (no container runtime in the sandbox). After import: create users, set each client user's `tenant_id` attribute to their company's tenant id, require TOTP for staff, and set the production redirect URIs. Set `HEKATE_JWKS_URL`, `HEKATE_ISSUER` on the API.
- **Push notifications:** devices register (`PUT /v1/devices`) but nothing sends yet. Needs an EAS project id (`extra.eas.projectId` in `mobile/app.json`), an FCM key for Android and an APNs key for iOS, and a consumer for the `ticket.*` outbox events.
- **NATS JetStream:** the API publishes outbox events to JetStream when `HEKATE_NATS_URL` is set (tested against an embedded server; the compose file sets it). Deploy and secure the broker in production (credentials, TLS, clustering) and add consumers.
- **App stores:** Apple Developer and Google Play accounts for builds and release.
- **Domains and TLS:** `hekatesystems.com`, `api.`, `app.`, `auth.` records and certificates.

## Needs infrastructure and people

- **Servers, WAF, VPN, Proxmox, off-site backups, HCDP monitoring:** the spec's perimeter and recovery requirements are infrastructure work, not code. Nothing here configures them.
- **Penetration tests (internal and external)** are required before go-live and cannot be done by me.
- **Accountant review** is the gate for payroll (phase 4) and two parallel quarters for accounting (phase 5). Code can exist, but the gates are human.
- **Dockerfiles and compose full stack: untested.** Run `docker compose -f deploy/docker-compose.yml --profile full up --build` once and fix anything environment-specific.
- **Mobile app on a physical device or emulator: untested.** It typechecks, passes expo-doctor and bundles for Android, but I could not run it.

## Known gaps in what was built

See the status table in the README for what exists per module. Everything in the spec not listed there as built is not built.
