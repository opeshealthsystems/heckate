# Forms glossary and register

Every form and template in the approved HEKATE pack, with its official name, the other names it goes by, how the platform uses it, and the check value of the approved original. Generated from the pack's `forms_registry.json` and `HEKATE_Document_Register.xlsx`; the same data drives the Documents library and Glossary screens (`web/lib/forms-library.ts`, `web/lib/glossary.ts`).

## How the platform treats these forms

* **The approved originals are shown untouched.** `web/public/forms/<code>.pdf` are the pack's signed PDFs byte for byte; each file's SHA-256 equals the value in the register (checked when this list was generated). The Documents screen previews them blank, so people see the approved layout before filling anything in.
* **Filling in** produces a draft text document in HEKATE's brand frame (title, client, sections, signature line). A draft can be previewed as it will print, marked DRAFT PREVIEW - NOT ISSUED - NOT VALID, with no reference and no QR code. Issuing gives it its reference, freezes it and files it in the register.
* **Printed layout follows the pack where it can be done programmatically.** The platform draws, with fixed measurements and no clock, the pack's title block (English and French title, `FORM / FORMULAIRE code - VERSION - date`), bilingual field cells, numbered or lettered section bars, tables with English-over-French headers, check boxes, the bilingual clause panels and the "Name / Nom, Date, Signature & stamp / Signature et cachet" signature blocks. It is a faithful re-drawing, not the pack's artwork: fonts are the PDF core fonts, the pack's watermark and footer line are not copied, and the footer keeps the platform's own reference and verification line (the pack's "Digitally signed" wording is not printed, because issued PDFs are not PAdES-signed). Status per form is in the next section.
* **How entered text is placed.** A narrative has free titled sections. Those whose heading equals a pack section heading (English or French, number prefix ignored) go into that pack box, in pack order; the rest print afterwards under "Additional sections / Sections complémentaires", never dropped. Pack boxes nobody filled print empty. Blank stays blank: unknown fields (address, NIU, telephone, bank details, staff number ...) are left empty, check boxes are ticked only when the entered text names the option, and fee tables carry no figures unless entered.
* **Pack wording is copied verbatim** where the pack prints a fixed clause (quotation, invoice and purchase-order terms, the asset declaration, the proposal "About" and "Why HEKATE" text, the order-confirmation and acceptance clauses). These are the company's approved texts, but they state commercial terms (50 % / 50 % payment, 30 days, BEAC rate plus 5 points): confirm they still apply.
* Classification is printed in the banner of every page and follows the pack's register.

## Printed layout status

Layout-faithful (header cells, section order, bilingual labels, box structure, signature blocks as in the pack; text of the entered fields only):

| Form | Notes |
| --- | --- |
| HS-QT Quotation | Document block, A client, B quoted services (7 rows minimum), totals, amount in words (French, computed), C terms, client acceptance boxes. Unit, discount, client ref., address, contact, NIU, telephone are blank (not held). The TVA 17.5 % / CAC labels print the rate only when the amount matches it. |
| HS-INV Invoice | Same blocks plus C payment (bank block blank) and issuer signature. Still shows nothing that changes after issue. |
| HS-PO Purchase order | Supplier block, items, net total only (tax lines and words blank), delivery and terms cells blank, three signature blocks. |
| HS-FRM-38 Asset issue and return | Employee block, asset table, security set-up check boxes (unticked), declaration, three signature blocks. Asset tag, staff no., department, condition, return date are blank (not recorded). |
| HS-FRM-27 Site visit acceptance | Title, PV no., project, client, acceptance decision boxes, reservations table, clause, client and HEKATE signature blocks (client signature image kept). Deviation: section 1 is headed "Work carried out / Travaux réalisés" with engineer, arrival, completion and report, because a visit records work, not a deliverables list; decision boxes stay unticked because only a signature is recorded. |
| HS-PR Proposal, HS-SOW Statement of work, HS-OC Order confirmation | All sections in pack order with the pack's blank tables (phases, team, schedule, fees, deliverables, milestones ...). Text entered under a matching heading goes in the box; a table filled in as text replaces the blank grid. |
| HS-FRM-23 Change request | Priority and decision boxes (ticked from entered text), requested change, impact assessment, two signature blocks. |
| HS-FRM-14 Incident report | Organisation block, type, severity, personal data and notified check boxes, description and actions boxes, reporter signature, "For HEKATE use only" block (blank). |

Frame only (pack title block and header cells, then each entered section as a numbered bar; the pack's own boxes for that form are not reproduced): HS-FRM-06, 07, 08, 09, 10, 11, 12, 13, 15, 24, 26, 31, 32, 33, 37, 39, 41.

Not changed (still the platform's plain brand-framed layout): HS-CN credit note, HS-RCT receipt, HS-RPT penetration test report, payslips, RFQ, letters, certificates (`HS-CRT-A`, the pack has no certificate PDF to follow), guarantee policies, contracts, HS-FRM-30 (no printable form), HS-FRM-35 and HS-FRM-36 (not printed by the platform), and the generated reports.

## Forms and templates

| Code | Official name | Also known as | Class | Folder | Module | In the platform |
| --- | --- | --- | --- | --- | --- | --- |
| HS-FRM-06 | Transmittal and Fax Cover Sheet | Transmittal Fax Cover | Confidential | Stationery | Documents | Filled in and printed by the platform |
| HS-FRM-07 | Client Information and Onboarding Form | Client Onboarding | Confidential | Client Intake | CRM | Filled in and printed by the platform |
| HS-FRM-08 | Service Request and Requirements Questionnaire | Service Request | Confidential | Client Intake | CRM / Support | Filled in and printed by the platform |
| HS-FRM-09 | Penetration Test Scoping Form | Pentest Scoping | Restricted | Client Intake | Projects | Filled in and printed by the platform |
| HS-FRM-10 | Security Assessment Pre-Engagement Questionnaire | Security Pre-Assessment | Confidential | Client Intake | Projects | Filled in and printed by the platform |
| HS-FRM-11 | hecID e-KYC Integration Request | hecID Integration | Confidential | Client Intake | Identity | Filled in and printed by the platform |
| HS-FRM-12 | Background Screening Request | – | Restricted | Client Intake | Identity | Filled in and printed by the platform |
| HS-FRM-13 | Client KYC and Due-Diligence Form | Client KYC Due Diligence | Restricted | Client Intake | CRM / Compliance | Filled in and printed by the platform |
| HS-FRM-14 | Incident Report and Emergency Response Request | Incident Report | Restricted | Client Intake | Support & Incidents | Filled in and printed by the platform |
| HS-FRM-15 | Customer Feedback and Satisfaction Form | Customer Feedback | Internal | Client Intake | Support | Filled in and printed by the platform |
| HS-CN | Credit Note | – | Confidential | Commercial | Finance | Filled in and printed by the platform |
| HS-FRM-23 | Change Request Form | Change Request | Confidential | Commercial | Projects | Filled in and printed by the platform |
| HS-FRM-24 | Rate Card | – | Confidential | Commercial | Sales | Filled in and printed by the platform |
| HS-INV | Invoice | – | Confidential | Commercial | Finance | Filled in and printed by the platform |
| HS-OC | Order Confirmation | – | Confidential | Commercial | Sales | Filled in and printed by the platform |
| HS-PR | Commercial and Technical Proposal | Proposal Template | Confidential | Commercial | Sales | Filled in and printed by the platform |
| HS-QT | Quotation | – | Confidential | Commercial | Sales | Filled in and printed by the platform |
| HS-RCT | Receipt | – | Confidential | Commercial | Finance | Filled in and printed by the platform |
| HS-SOW | Statement of Work | – | Confidential | Commercial | Sales / Projects | Filled in and printed by the platform |
| HS-FRM-26 | Project Kick-off Record | Project Kickoff | Confidential | Project Delivery | Projects | Filled in and printed by the platform |
| HS-FRM-27 | Delivery Note and Acceptance Certificate | Delivery and Acceptance PV | Confidential | Project Delivery | Projects | Filled in and printed by the platform |
| HS-FRM-30 | Timesheet and Service Log | Timesheet | Internal | Project Delivery | Projects / HR | Data recorded in its own module; no printable form yet |
| HS-FRM-31 | Project Closure and Sign-off | Project Closure | Confidential | Project Delivery | Projects | Filled in and printed by the platform |
| HS-RPT | Penetration Test Report | Penetration Test Report Template | Restricted | Project Delivery | Projects | Filled in and printed by the platform |
| HS-FRM-32 | Job Application Form | Job Application | Confidential | HR | HR | Filled in and printed by the platform |
| HS-FRM-33 | Offer of Employment | Offer Letter | Confidential | HR | HR | Filled in and printed by the platform |
| HS-FRM-35 | Leave Request | – | Internal | HR | HR | Data recorded in its own module; no printable form yet |
| HS-FRM-36 | Expense Claim | – | Internal | HR | HR / Finance | Data recorded in its own module; no printable form yet |
| HS-FRM-37 | Visitor and Laboratory Access Log | Visitor and Lab Access Log | Internal | HR | Facilities | Filled in and printed by the platform |
| HS-FRM-38 | Asset Issue and Return Form | Asset Issue and Return | Internal | HR | Inventory & Assets | Filled in and printed by the platform |
| HS-FRM-39 | Supplier Registration Form | Supplier Registration | Confidential | Suppliers | Procurement | Filled in and printed by the platform |
| HS-FRM-41 | Supplier Evaluation Form | Supplier Evaluation | Internal | Suppliers | Procurement | Filled in and printed by the platform |
| HS-PO | Purchase Order | – | Confidential | Suppliers | Procurement | Filled in and printed by the platform |

Approved version of every entry above: **1.0**. SHA-256 of each approved original:

| Code | SHA-256 |
| --- | --- |
| HS-FRM-06 | `88ce6d305c71eaabb642dd81a8792e0e1114afbc9df9d49d0e2763ccfd7225e6` |
| HS-FRM-07 | `3aa5863a09fd64a2072c42ac0cb016700a4dfa7b0f5a5c56350694b1706accf4` |
| HS-FRM-08 | `ca0cd4c6995ec67291c9594a2594a2db387178e09e36c1ace850012179e0ccd6` |
| HS-FRM-09 | `beae289a312c1dc909567a665f72bcca767a55002af749d938029117b4d23c7c` |
| HS-FRM-10 | `9152f978251e3b3ee292f8ccb8abbf0247e89310e8b10baf914c75f154b91f22` |
| HS-FRM-11 | `d8d0ea3bd89a7263e76e116e848513a29aa8f179ef956f29dcda2f9cf4f9ccf4` |
| HS-FRM-12 | `010e5744baf02dfe68f59568a68a87066f7317f677b426ebfac992044f3302f2` |
| HS-FRM-13 | `c8b4684bee2390716f87bdc999585492983cba689b678a83bc3a00dd6a14c01d` |
| HS-FRM-14 | `c32174a5e57e030217eca02f1c5eb7d7800befa5989922eb4e95d59683b685ae` |
| HS-FRM-15 | `7189066666945bb44a7f10da691c8b412e7e607bfc9205da373afb9b9244b5ce` |
| HS-CN | `37459fd42770e00f7b01de2b70ef8823d5983a98a64c828d438717e2e31cacc6` |
| HS-FRM-23 | `bc8154b6abed652d949f24a2d70f9891e7cac62bac0b0c66e6dc37607bf09a8a` |
| HS-FRM-24 | `912fc6514e21029dd6791676dfd5483e5c2efd86138294e3a524afd08442319d` |
| HS-INV | `7c7f284744143fd59c3e65b736507595f3281f2786b301e44964dc41cec20e01` |
| HS-OC | `263613a4d400a865ca93eefe70d51698c935dd7cf4d93ff96f667908a3233af8` |
| HS-PR | `bc1e449f2e6478ba0b96cc9e566d3a3f88ce7417ac94b8f72debef4e88168921` |
| HS-QT | `fd3057381e2eda233012ea8ac4b2817caa29174faaa0e121cba4bd504f17f671` |
| HS-RCT | `3f588f9511403ebbe95b9ac1bb469a315e4ec397f36acf071465f8b3946c7adc` |
| HS-SOW | `305789101f98543ded6bd78fa9479b2393b07879f26ab02fea48d8f915deca28` |
| HS-FRM-26 | `b76e2b42fca4083aa63c71e44e125d6ef97c6fa1f54abf6c8a9dd81e8333fc38` |
| HS-FRM-27 | `3decc06e98eaf266ab945cfa3ae8ee585b78d1945beec54c4fd4dee52809875a` |
| HS-FRM-30 | `1d017257bcee65fa074b66ee14f16b44b822e727a39ea1df2801a8160a713e1d` |
| HS-FRM-31 | `14831739e53610ac49313f0f68e70bfe5e1e780102b76bc7aa9dcb1f8d2d1e88` |
| HS-RPT | `a24aaf7fade04bf6cfebbc085a21693bee774f5e5f26128176db0f205c3bc10a` |
| HS-FRM-32 | `f618acff0edc23615cc887268c37b5f36c50abe86813a99636222780d82eb320` |
| HS-FRM-33 | `c8983e6f86978baf6272f4dfd3952f84a2fcbb2c214d769f66ed2200ee8c99b5` |
| HS-FRM-35 | `988303f7657ebe6acb9875318e1e57d199a7c9e9678fc7f2d9f79e73b86cc792` |
| HS-FRM-36 | `f7651701293e6239181fd5fe6ae6176db515392336b5482829cccbb8a84ab58e` |
| HS-FRM-37 | `07e4d2fc8ebe7f2a50c34dea98805293f27cfad7a97b309fc1224904bb09042e` |
| HS-FRM-38 | `112c30d8a561ca7360fb8766acca2fb676de0bf25238d3546b44af31f2130d65` |
| HS-FRM-39 | `26a1a22386aa2e6a765c2b0a73aaaf9ce697394c19ee0a7ef19e21e14b39ce51` |
| HS-FRM-41 | `6bd9bdd9eb3ad4f0e98998b46318500206241ca8cd0654ffa957e0c83b8f122f` |
| HS-PO | `a53a3cb96469e68e89049c89558da8cce2645bf39085284cfb3d53ca2ce1ac5a` |

## Version history and revisions

* Every save of a draft text document keeps a numbered version: who saved it, when, why (a short change note), and a SHA-256 fingerprint of the content. Saving without a change records nothing. The history is append-only: the database refuses to change or delete a version.
* Issuing records a final version (`Issued as <reference>`) and freezes the document.
* A change after issue is a **revision**: a new draft copied from the issued document (`Revise`), numbered one higher, whose history begins `Revision of <reference>`. The issued document is never touched. When the revision is issued, the old reference stays valid and verifiable, and `/verify` shows that it has been replaced and by which reference.
* Every create, edit, issue and revise is also in the tamper-evident audit log. Screens: Documents, My documents, History and Revise. API: `GET /v1/narratives/{id}/versions`, `POST /v1/narratives/{id}/revise`.
* Not yet covered: documents produced by other modules (invoices, quotes, payslips and so on) are frozen at issue and corrected with credit notes or new documents, as accounting requires; they do not have draft histories. The pack's *Revision Log* sheet (document, version, date, author, change, approver, hash) is not imported: the platform keeps its own history, and approver sign-off on a revision is not modelled.

## Verification page

`/verify` (and `/v/<code>` for a scanned QR code) is public. It shows the HEKATE seal and one of: **Authentic**, **Genuine but replaced** (naming the newer version), **Withdrawn**, or **Not valid** (unknown reference, or a code whose signature is not HEKATE's or does not match the record). For a genuine document it shows the official document type, title, reference, version, issue date, classification, issuer, how often it has been verified, whether the QR signature was checked, and the original file's fingerprint (a file you hold can be compared in your own browser; it is never uploaded). It never shows the client or the content. From the page, someone can **request access** with a reason; managers decide under Documents, Access requests. Staff see how often each document was verified in its History panel.

## Reference numbers: a difference in the pack that needs a decision

The pack describes document references in two ways:

* `content/documents.md`: `HS-<CODE>-<YYYY>-<NNNNNN>` (six-digit serial), with a check character on certificates (`HS-CRT-A`, `HS-ASM`, `HS-CRT-C`).
* The ID Scheme sheet of `HEKATE_Document_Register.xlsx`: `HS-<CODE>-<year>-<nnnn>` (four-digit serial) and `HS-CERT-<year>-<nnnn>` for certificates; corporate documents are `HS-DOC-<nn>`.

The platform follows `documents.md` (six digits, `HS-CRT-A`). Which one is right is the company's call; changing it is a small, contained change in `documents.Issue` and the reference pattern, but it must be decided before the first real document is issued, because issued references never change.

## Terms

| Term | Meaning |
| --- | --- |
| HS | HEKATE SYSTEMS: the prefix of every HEKATE document code. |
| FRM | Form. HS-FRM-23 is form number 23 of the HEKATE form set. |
| Reference | The unique number the platform gives a document when it is issued, for example HS-INV-2026-000012. It is what a QR code and the /verify page check. |
| Draft | A document being written. It has no reference yet, can still be changed, and is not valid. |
| Issued | Numbered, frozen and recorded in the document register. An issued document can no longer be edited. |
| Classification | How widely a document may be shared: Public, Internal, Confidential or Restricted. It is printed in the banner at the top of every page. |
| Verify | Anyone can check a document's reference or file at /verify; a changed file fails the check. |
| Register | The log of every issued document, with its version, owner and SHA-256 check value. |
| SHA-256 | A fingerprint of a file. If one character of the file changes, the fingerprint changes. |
| PAdES | A standard for electronically signing PDF files so that tampering can be detected. |
| PV | Procès-verbal: the signed record of delivery and acceptance (HS-FRM-27). |
| RFQ | Request for quotation: HEKATE asks suppliers to price a list of items. |
| PO | Purchase order: HEKATE's order to a supplier. |
| SOW | Statement of work: what will be delivered, by when, and for how much. |
| MSA / SLA | Master services agreement and service levels: the contract terms and the response and restoration times that go with it. |
| P1 to P4 | Ticket priorities. P1 is critical (service down), P4 is a question or minor issue. |
| KYC | Know your customer: identity and due-diligence checks on a client. |
| hecID | HEKATE's identity and e-KYC service. |
| TVA | Taxe sur la valeur ajoutée: value-added tax, shown on quotations and invoices. |
| CAC | Centimes additionnels communaux: the municipal surcharge added to TVA. |
| IRPP | Impôt sur le revenu des personnes physiques: personal income tax withheld on payslips. |
| RCCM | Registre du commerce et du crédit mobilier: the company registration number. |
| NIU | Numéro d'identifiant unique: the taxpayer identification number. |
| XAF | Central African CFA franc, the currency of all amounts. |
| OHADA | The business-law treaty that governs HEKATE's contracts together with the law of Cameroon. |
