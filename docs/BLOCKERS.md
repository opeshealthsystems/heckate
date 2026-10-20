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

## Documents and forms: decisions for the company

* **Reference format.** The pack gives two schemes (`documents.md`: six-digit serial and `HS-CRT-A`; the register's ID Scheme sheet: four-digit serial and `HS-CERT`). The platform follows `documents.md`. Decide before the first real document is issued; see `FORMS_GLOSSARY.md`.
* **Exact form layouts.** Partly done. Quotation, invoice, purchase order, asset issue (HS-FRM-38), site-visit acceptance (HS-FRM-27), proposal, statement of work, order confirmation, change request (HS-FRM-23) and incident report (HS-FRM-14) now print in the pack's structure: title block, bilingual cells, section order, boxes, signature blocks. Seventeen further narrative forms print in the pack's title block with the entered sections as bars only. Credit note, receipt, penetration test report, payslip, RFQ, letter, certificate, guarantee and contract still use the plain brand frame. It is a programmatic re-drawing (core PDF fonts, no pack watermark or footer art), not the pack's artwork, and not a pixel copy; the signed originals remain available for preview. Open for the company: (1) confirm the verbatim pack clauses copied into the output (50 % / 50 % payment, 30 days, BEAC rate plus 5 points, the proposal's About and Why HEKATE text); (2) the web editor does not yet pre-fill the pack's section headings, so entered text lands in a pack box only when the heading matches (see `FORMS_GLOSSARY.md`); (3) the pack footer's "Digitally signed" line is deliberately not printed until PAdES signing exists; (4) the pack's official layouts for the remaining forms need field-level specs from the company to reproduce.
* **Signed QR codes.** The platform can print and verify the pack's signed QR addresses (`/v/<reference>.<version>.<date>.<class>.<signature>`, Ed25519, checked against `HEKATE_QR_Public.pem`). Printing them needs the private key from the secret store: set `HEKATE_QR_SIGNING_KEY_FILE` to an unencrypted PKCS#8 PEM (the pack's key is password-protected; export an unencrypted copy into the secret store, never the repository). Until then QR codes carry plain `/verify?ref=` addresses, which work but prove nothing offline. A key that does not match the published public key stops the API starting.
* **Access requests.** Someone who verified a document can ask to see it, with a reason; a manager or the CEO approves a time-limited, download-limited link or refuses. The platform has no email channel yet, so the manager copies the link and sends it to the requester.
* **Check character.** Certificates now use ISO/IEC 7064 MOD 37,36 over the whole reference, as the pack's generators do (HS-ASM-2026-000148 gets P, as on the approved design). The pack's assessment generator uses the prefix `HKS-ASM` where the templates use `HS-ASM`; the platform uses `HS-`.
* **Signing.** Issued PDFs carry a QR code and a recorded SHA-256 but are not PAdES-signed until a signing certificate exists (see `SECURITY.md`).

## Contracts, client forms, projects, labels, employee files, bank accounts: decisions for the company

* **Contract wording.** The platform stores the clauses you type and prints a *contract schedule* (terms, response times, clauses) from them. It is a printout of the register, not the signed agreement: have counsel draft the wording, and keep the signed original in the document register.
* **Contract response times.** There are no default numbers: a contract promises only what you enter. Tickets from a client with no active contract, or no time set for a severity, use the standard targets, which are still the placeholders in `support.sla_policies` (15 minutes / 4 hours for P1 and so on). Decide the real service levels.
* **Renewal rules.** Warnings go out 90, 60 and 30 days before the end to management and sales; a contract with auto-renew is extended by its renewal term when it passes its end date, otherwise it is marked expired. Confirm that this matches your standard terms (notice period, price changes on renewal).
* **Client form questions.** The four online forms (site survey, company details, incident report, delivery acceptance) are generic starting points the sender can edit. They are not the pack's numbered forms; the pack's exact layouts remain a decision above.
* **Employee data.** National ID, CNPS number, bank details and emergency contacts are held in a separate table that only HR, the CEO and the person can read. Confirm with counsel what Cameroonian data-protection rules require before real employee data goes in (consent, retention, who may export).
* **Bank accounts.** The register holds only what you type. Enter account numbers, IBAN and signatories exactly as the bank confirmed them in writing.
* **Project cost.** Costs are entered by hand (labour, materials, travel, subcontract). Timesheet hours are not yet priced automatically, so "actual" is only as complete as the entries.
* **QR stickers.** Sheets are A4, 3 x 8 (63.5 x 33.9 mm), a common 24-up label size. Check your label stock; the size is a constant in `api/internal/docs/labels.go`. A sticker's QR opens a page that needs sign-in, so a photographed sticker reveals nothing.
