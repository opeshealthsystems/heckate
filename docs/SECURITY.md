# Security

This is the document to hand to a bank's, insurer's or pharmaceutical company's security reviewers or to an independent audit firm. It says what is enforced, **where to verify each claim**, and, equally important, what is **not** done yet.

**No system is "hacker proof", and this one is not claimed to be.** The goal is a system whose controls are real, narrow, tested and verifiable by someone who does not trust us, with every known weakness written down. Anything an auditor finds that is not on the gap register below is a failure of this document, and we want to hear about it.

## 1. How the platform is protected

| Threat | Control | Where to verify |
| --- | --- | --- |
| One client reading another client's data | PostgreSQL **row-level security** on every business table. The API connects as an owner but runs each request as a role without bypass rights; the tenant, role and user are set inside the request's transaction | Migrations `0001` onward; `internal/server/*_test.go` (cross-tenant cases in every module); the whole suite is also run as an ordinary non-superuser database owner |
| A code bug bypassing authorization | The rules that matter are also **database triggers**: issued invoices and the ledger are immutable, approvals need a second person, payroll cannot be self-approved. Raw SQL cannot get around them | Tests named `...InTheDatabase`, `payroll_test.go`, `bank_test.go` |
| Tampering with financial history | Ledger is append-only, **hash-chained**, balanced entries enforced at commit; `GET /v1/accounting/verify` recomputes the chain | `accounting_test.go` |
| Tampering with the audit trail | Audit rows cannot be updated or deleted; a background sealer links them into a hash chain; `GET /v1/audit/verify` recomputes it and names the first bad row | `audit_test.go` (rewrites a row with the guard switched off and shows it is detected) |
| Fraud by one insider | **Dual control**: invoices and purchase orders above a threshold, payroll prepare / approve / pay by three different people, supplier bank details verified by call-back, expense approver differs from payer | Same tests |
| Forged documents | Every issued document has a reference with a check character, a QR code and a stored SHA-256; anyone can check a file against the register in their browser without uploading it | `pdf_test.go`, `e2e/documents.mjs` |
| Stolen or replayed requests | Idempotency keys on every POST (a retry never double-applies); optimistic locking (`If-Match`) on edits | `server_test.go` |
| Token forgery | Tokens verified against the identity provider's keys (RS256); issuer and audience pinned; expiry enforced; the development HS256 path **cannot start in production** | `platform/auth.go`, `config.go` `Problems()`, `hardening_test.go` |
| Brute force and abuse | Per-caller rate limits with standard headers; API keys have their own per-plan limits and monthly quota. The caller's address is taken from `X-Forwarded-For` **only** from configured trusted proxies, and only the right-most untrusted hop | `hardening_test.go` |
| Weak configuration reaching production | The server **refuses to start** in production mode with: a dev token secret, the payment sandbox, a database connection without TLS, a wildcard CORS origin, or without the identity provider's keys, issuer and audience | `Config.Problems()` |
| Browser attacks (XSS, clickjacking, MIME sniffing) | API: `nosniff`, `no-store`, HSTS, `frame-ancestors 'none'`, locked-down CSP. Web app: CSP restricting scripts and connections to itself, the API and the identity provider; `X-Frame-Options: DENY`; no token in `localStorage` (session storage only) | `platform/app.go` `SecureHeaders`, `web/next.config.ts` |
| Malicious uploads | Type decided from the file's bytes (JPEG, PNG, WebP, PDF only), 10 MB limit, safe file names, `nosniff` and a sandboxing CSP on download | `files_test.go` |
| Slow-client and oversized-request attacks | Read / write / idle timeouts, header size cap, 1 MB body cap on JSON | `cmd/api/main.go` |
| Vulnerable dependencies | `govulncheck` (fails on any vulnerability reachable from our code, including the Go standard library), `npm audit`, Trivy and secret scanning on every change **and nightly**, CodeQL, Dependabot | `.github/workflows/security.yml`, `.github/dependabot.yml` |
| Secrets in the repository | Gitleaks over the full history on every change; CI and test credentials are throwaway and allow-listed explicitly | `.gitleaks.toml` |
| Forged payment confirmations | A payment is booked only on an HMAC-signed provider callback or a status lookup at the provider; repeated deliveries are harmless; a different amount than requested goes to manual review | `payments_test.go`, `e2e/payments.mjs` |
| API key leakage | Keys are random, shown once, stored only as SHA-256, read-only, limited to named resources, cannot manage keys, revocable instantly | `devapi_test.go` |

## 2. Known gaps (the register)

Severity is our own judgement, assuming the platform handled real client data tomorrow.

| ID | Gap | Severity | Plan |
| --- | --- | --- | --- |
| S-01 | **No independent penetration test has been done.** | High | Commission before go-live; fix findings; keep the report in the evidence pack (needs people) |
| S-02 | **Identity provider integration is untested** against a real Keycloak (realm file is provided). Multi-factor authentication for staff, password policy, lockout and session length are configured there, not in this code | High | Stand up Keycloak, import the realm, test login, MFA, token expiry and rotation (needs infrastructure) |
| S-03 | Rate limiting is **in memory per instance**; with several replicas the effective limit multiplies | Medium | Enforce at the gateway / WAF; add a shared counter if needed |
| S-04 | No token revocation list: a stolen token works until it expires. Lifetime is set in Keycloak | Medium | Short access tokens (5 to 10 minutes) with refresh; document the Keycloak settings |
| S-05 | The web app's CSP still allows inline scripts (a Next.js requirement without nonces), and the session token lives in `sessionStorage`, readable by any script that runs in the page | Medium | Move to server-rendered pages with CSP nonces, or the backend-for-frontend pattern with an HttpOnly cookie |
| S-06 | Sensitive fields (salaries, bank details, phone numbers) are **not encrypted at the field level**; protection is the database's and the disk's | Medium | Disk and backup encryption in the deployment; field encryption with a managed key if the review requires it |
| S-07 | Uploaded files are checked by type but **not scanned for malware** | Medium | Add a scanning step (ClamAV) before a file becomes downloadable |
| S-08 | Metrics (`/metrics` on an internal port), probes and alert rules now exist, **but nothing is deployed**: no Prometheus loaded the rules, no log shipping, nobody is paged | High | Deploy monitoring; name an on-call rota (`RUNBOOK.md`) |
| S-09 | Backup, encrypted backup, restore and a restore drill exist and run in CI against a database with real ledger and audit rows. **Not done:** off-site copy, WAL archiving / replica (so RPO is the backup interval), a drill at production size, agreed RPO / RTO (`DR.md`) | High | Infrastructure work; owner to confirm objectives |
| S-10 | The Docker images and the production compose files are **untested**; images are not signed and there is no SBOM | Medium | Build in CI, scan images, generate an SBOM, sign with cosign |
| S-11 | Load has been measured once on a single shared 4-vCPU machine (`PERFORMANCE.md`: about 3,500 requests per second on ticket and profile reads, no errors). **Not done:** a production-like environment, stress and soak tests, and the heavy paths (PDF rendering, payroll, ledger posting, which serialises writers by design) | Medium | Load tests with budgets on production-like hardware; measure before claiming capacity |
| S-12 | Secrets come from environment variables; no managed secret store or rotation | Medium | Vault / cloud secret manager in the deployment |
| S-13 | PAdES signing is **built but off**: it activates when `HEKATE_PDF_SIGNING_CERT_FILE` and `HEKATE_PDF_SIGNING_KEY_FILE` are set (see "PDF signing" below). Until then, and for every document already stored, PDFs are hash-registered and QR-verifiable but not cryptographically signed. **Not done:** a certificate from a trusted CA, RFC 3161 timestamping and revocation data (LTV, so not PAdES-B-T/B-LT), visible signature appearance, Acrobat validation | Medium | Obtain a certificate (and ideally a timestamp authority); validate in Acrobat and an EU DSS validator before relying on it |
| S-14 | There is no responsible-disclosure channel (`security.txt`, a monitored mailbox) | Low | Needs a company contact address |
| S-15 | Orange Money (written, off) and (live) MTN MoMo are not exercised against the real services | Medium | Sandbox runs when the providers issue credentials |
| S-16 | The mobile app has not been tested on a real device, and certificate pinning is not in place | Medium | Device testing; evaluate pinning against the operational cost of key rotation |
| S-17 | Retention and deletion rules (data protection law, contracts) are not defined; the ledger and audit trail are append-only by design | Medium | Legal input on retention; design a deletion path for personal data that is not legally retained |
| S-18 | No formal threat model workshop or ASVS verification has been done by a second party; the mapping below is our own | Medium | Review with the auditor |

## 3. OWASP ASVS 4.0.3 self-assessment (level 2 target)

"Yes" means implemented and covered by a test or a verifiable configuration. "Partly" and "No" are listed honestly. This is **our own** assessment, not a certification.

| Chapter | Status | Notes |
| --- | --- | --- |
| V1 Architecture, design, threat modelling | Partly | Controls above; no formal threat model review (S-18) |
| V2 Authentication | Partly | Delegated to Keycloak; untested here (S-02) |
| V3 Session management | Partly | Stateless bearer tokens; no revocation (S-04); token storage (S-05) |
| V4 Access control | **Yes** | Row-level security, role allow-lists on every route, database triggers, cross-tenant tests |
| V5 Validation, sanitisation, encoding | Yes | Strict JSON decoding (unknown fields rejected), parameterised SQL everywhere, no HTML rendering of user input on the server, file type by content |
| V6 Cryptography | Partly | Standard library crypto only; no field-level encryption (S-06); document signing built but off until a certificate is supplied (S-13) |
| V7 Error handling and logging | Partly | RFC 9457 problems with a trace id and no internals; tamper-evident audit trail; no central log pipeline (S-08) |
| V8 Data protection | Partly | `no-store` on API responses; retention undefined (S-17) |
| V9 Communications | Partly | HSTS and TLS-only database enforced at start-up; TLS termination is a deployment matter (S-10) |
| V10 Malicious code | Partly | Dependency, secret and code scanning in CI; no malware scanning of uploads (S-07) |
| V11 Business logic | **Yes** | Dual control, idempotency, immutability, limits, enforced in the database |
| V12 Files and resources | Partly | See V10 |
| V13 API and web service | Yes | Contract-first OpenAPI, a test fails when code and contract differ, rate limits, content-type handling |
| V14 Configuration | Yes | Refuses unsafe production configuration; security headers; pinned dependencies |

## 4. Evidence an auditor can reproduce

```
# Everything, against a real PostgreSQL (see README for setup)
cd api && go test -race ./...

# The same suite as an ordinary, non-superuser database owner (what production should be)
# see .github/workflows/ci.yml "Tests as a non-superuser owner"

# Known-vulnerability scan of the code and the Go standard library
cd api && go install golang.org/x/vuln/cmd/govulncheck@v1.9.0 && govulncheck ./...

# Does the code match the published contract?
go test ./internal/server -run Contract

# Is the ledger intact?   GET /v1/accounting/verify
# Is the audit trail intact?   GET /v1/audit/verify
```

CI runs the first four on every change; the vulnerability scans also run nightly.

## 5. Reporting a vulnerability

A monitored security mailbox and a `security.txt` are still to be set up (S-14). Until then, report to the platform owner directly. Do not test against real client data.

## PDF signing (S-13)

**Supplying a certificate.** Put a PEM certificate file (leaf first, then any intermediates) and an unencrypted PEM private key (PKCS#8, PKCS#1 RSA or SEC1 EC; RSA or ECDSA P-256/P-384) in the secret store, mount them, and set `HEKATE_PDF_SIGNING_CERT_FILE` and `HEKATE_PDF_SIGNING_KEY_FILE` (see `deploy/production.env.example`). Never commit either file. With neither set, PDFs are byte-for-byte what they were before. With one set, `Config.Problems()` refuses to start in production. With both set, start-up fails if a file is unreadable, the key does not match the first certificate, or the certificate is not valid at start-up (an expired certificate is caught at the next restart, not while running).

**What it does.** The first time a document's PDF is rendered, `documents.ServePDF` appends a detached CMS signature (SHA-256; RSA PKCS#1 v1.5 or ECDSA; SubFilter `ETSI.CAdES.detached`; signed attributes content-type, message-digest and signing-certificate-v2) as an incremental update, then hashes, stores and serves those signed bytes. So the register's SHA-256, the public verification page, the verify-by-file check, access-request downloads and every later download all refer to the same signed file. The signing time (`/M`) is the register's issue time, not the clock; RSA output is reproducible for a fixed key and time (ECDSA signatures are randomised by the standard library, which is harmless because only one rendering is ever stored). If signing fails, the request fails; an unsigned original is never stored while signing is on. Code: `api/internal/docs/pades.go`.

**Consequences to know.** Documents whose PDF was stored before a certificate was supplied stay unsigned: one reference is one file, and re-signing would change the registered hash. Changing or renewing the certificate affects only documents rendered afterwards. The signature is invisible. The private key sits in API process memory; an HSM or KMS-backed signer is not implemented.

**What is validated, and what is not.** Tested in CI: a throwaway self-signed certificate is generated in the test; an independent parser (separate from the encoder) checks the ByteRange coverage, the message-digest against the recomputed SHA-256, the signature against the certificate, the signing-certificate-v2 hash, for RSA and ECDSA, compressed and uncompressed PDFs; a changed byte, a swapped signature and a corrupted appended section all fail; the xref chain is walked; signing time, determinism and key-loading rules are asserted; and an end-to-end test checks the register hash, verification page and repeat downloads against the signed bytes. Where `pdfsig` (poppler) is installed the test also requires it to report "Signature is Valid" (and not to accept a tampered file); it reported a valid SHA-256 `ETSI.CAdES.detached` signature for both key types, with "certificate issuer is unknown" because the test certificate is self-signed. **Not validated:** Adobe Acrobat/Reader (not available here), the EU DSS/ETSI conformance tool, any trusted-CA chain, certificate revocation, and long-term validation. The implementation is hand-written to the PAdES/CAdES profile rather than taken from a vetted library; have it validated against an independent PAdES validator before treating signed PDFs as legally signed documents.
