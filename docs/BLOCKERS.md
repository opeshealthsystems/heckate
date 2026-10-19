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
- **Push notifications:** the API sends them through the Expo push service when `HEKATE_PUSH=1` (tested against a fake service: right tenant only, never the author, P1 to support/manager/engineer devices, dead tokens removed). To reach real phones you still need an EAS project id (`extra.eas.projectId` in `mobile/app.json`), an FCM key for Android and an APNs key for iOS uploaded to EAS, and optionally `HEKATE_EXPO_ACCESS_TOKEN`. There is no on-call roster yet, so a P1 pages every support, manager, CEO and engineer device.
- **NATS JetStream:** the API publishes outbox events to JetStream when `HEKATE_NATS_URL` is set (tested against an embedded server; the compose file sets it). Deploy and secure the broker in production (credentials, TLS, clustering) and add consumers.
- **Mobile money payments:** set `HEKATE_MOMO_BASE_URL`, `HEKATE_MOMO_SUBSCRIPTION_KEY`, `HEKATE_MOMO_API_USER`, `HEKATE_MOMO_API_KEY`, `HEKATE_MOMO_ENVIRONMENT` (and `HEKATE_MOMO_CURRENCY`, default XAF) once MTN issues a Collection account. The client follows MTN's public "request to pay" flow and is tested only against a fake server, so run a sandbox payment end to end before launch. Orange Money (Cameroon) WebPay has a client that stays off until `HEKATE_ORANGE_BASE_URL`, `HEKATE_ORANGE_CLIENT_ID`, `HEKATE_ORANGE_CLIENT_SECRET` and `HEKATE_ORANGE_MERCHANT_KEY` are set (also `HEKATE_ORANGE_NOTIF_URL`, `_RETURN_URL`, `_CANCEL_URL`, `_COUNTRY` default cm). It follows the published WebPay flow and is tested only against a local stand-in; it is not exercised against the real service, and the client must be sent to the provider's payment page, which the web UI does not do yet. Callbacks need `HEKATE_PAYMENTS_WEBHOOK_SECRET`; providers that cannot sign with our HMAC header need a small adapter, and the page already settles by polling the provider every few seconds. Never set `HEKATE_PAYMENTS_SANDBOX=1` in production. A payment whose amount differs from the request, or that lands on an already-paid invoice, goes to status `review` for finance.
- **App stores:** Apple Developer and Google Play accounts for builds and release.
- **Domains and TLS:** `hekatesystems.com`, `api.`, `app.`, `auth.` records and certificates.

## Needs a decision or asset for documents

- **PAdES signing** needs a signing certificate (and a choice of CA). PDFs are hash-registered and QR-verifiable but not cryptographically signed.
- **Letterhead and company facts** for PDFs: set `HEKATE_LETTERHEAD` (lines separated by `|`).
- **Fonts:** PDFs use built-in Latin-1 fonts; characters outside cp1252 need an embedded Unicode font.
- **Placeholders needing sign-off:** SLA times, approval thresholds, leave rules, TVA/CAC rates and the starter chart of accounts.
- **Payroll rates must be validated by the accountant.** The seeded CNPS, CFC, FNE and income-tax figures (Payroll > Rules) come from public Cameroon rules as I understand them and are not verified. The system refuses to prepare any run until someone other than the editor validates the rules, and a changed rule needs validating again. Also missing: the communal tax table, overtime and bonuses, leave deductions, and the DIPE/CNPS declaration files. Ledger accounts 422, 431, 447 and 664 are a proposed mapping.
- **Bank reconciliation** takes a CSV (date, description, amount, reference); real bank file formats (MT940, CAMT.053) and bank APIs are not wired. Matching is exact-amount within five days only.
- **Not built:** DSF statements, paid-plan billing, SDKs other than TypeScript (the Python/Java/C# ones the spec lists can be generated from the same contract), Metabase.
- **Attachments** live in Postgres, not object storage; move to MinIO/S3 if volume grows.

## Needs input for the public website

The site now uses the approved brand pack (see `docs/BRAND_PACK.md`). Still missing, and therefore left out of the pages rather than shown as `[PLACEHOLDER]` or invented:

- **Phone, P.O. box, RCCM and NIU.** Fill them in `site/content/company.ts`; each appears on the site and in the structured data once set.
- **French copy review.** The French text is my translation of the approved English and has not been read by a native speaker. The guarantee policies are legal text: the French page shows the English original with a notice until an approved French version exists.
- **Privacy and Terms pages.** The design footer links to them, but the pack has no text for them, so there are no links.
- **Mobile app store links** (Google Play, App Store) once the app is published.
- **Founders' photos.**
- **Fonts and signing keys.** Fonts (Cinzel, IBM Plex) are self-hosted from the OFL `@fontsource` packages. The pack deliberately contains only public verification keys; the signing keys and password for its PDF tools must come from a secret store, never the repository.

## Needs infrastructure and people

- **Servers, WAF, VPN, Proxmox, off-site backups, HCDP monitoring:** the spec's perimeter and recovery requirements are infrastructure work, not code. Nothing here configures them.
- **Penetration tests (internal and external)** are required before go-live and cannot be done by me.
- **Accountant review** is the gate for payroll (phase 4) and two parallel quarters for accounting (phase 5). Code can exist, but the gates are human.
- **Dockerfiles and compose full stack: untested.** Run `docker compose -f deploy/docker-compose.yml --profile full up --build` once and fix anything environment-specific.
- **Mobile app on a physical device or emulator: untested.** It typechecks, passes expo-doctor and bundles for Android, but I could not run it.

## Known gaps in what was built

See the status table in the README for what exists per module. Everything in the spec not listed there as built is not built.
