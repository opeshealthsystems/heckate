# Brand and content pack

HEKATE supplied an approved Brand & Content Pack (four zip parts that merge into one tree of 192 files, about 105 MB). It is **not** committed: it is large, and most of it (forms, legal templates, certificate generator, stationery) is source material rather than code. Keep the original zips with the company records.

## What the pack contains

| Folder | Contents | Used so far |
| --- | --- | --- |
| `brand/` | Logos, seals, emblem, icons, tokens, guidelines | Logos (white, colour), seal watermark, icons and the tokens, in `site/public` and `site/app/globals.css` |
| `content/` | About, services, contact, guarantee policies, documents catalogue (EN) | The whole public site copy, in `site/content/en.ts` |
| `design/screens/` | Approved screen designs as HTML | Home, About, Contact followed for the public site; the sign-in, client, staff and documents screens are still to be matched in `web/` and `mobile/` |
| `forms/`, `legal-templates/`, `certificates/`, `stationery/` | 33 forms, contracts, certificate specimens and generator, letterhead, cards | Not yet. The platform renders its own PDFs; matching these exact layouts, fonts (Lora, Arial) and security print features (microtext, guilloche, PAdES) is still to do |
| `security/` | Public verification keys and signing tools | Not used. The tools expect a `pki/` folder and password that the pack deliberately omits |

## Rules from the pack that the code follows

- Navy + gold + white only on the website and app, no gradients, glows or shadows; status colours for badges only.
- Cinzel for headings, IBM Plex Sans for text, IBM Plex Mono for references; contrast at least 4.5:1; touch targets at least 44 px.
- Never invent company facts, prices, statistics or client names. Blank facts are omitted, not shown as placeholders.

## Still to do from the pack

1. Mobile is restyled (brand tokens, Cinzel and IBM Plex, navy headers, sign-in, icon, adaptive icon and splash from the emblem). It was bundled and typechecked but not run on a device; the remaining screens use the new theme but were not compared one for one with `design/screens/M*.dc.html`. (`web/` is done: navy header, brand fonts and the split-screen sign-in; the dashboard layouts in `design/screens/` are not copied one for one.)
2. PDFs now carry the brand frame on every page (colour logo, classification banner in the pack's colours, gold rule, navy headings, signed-QR block, 1.5 pt microtext footer rule). Documents already issued keep their stored layout (a reference is always one file); only new documents use it. Still to do: the exact field layout of each of the pack's forms, the Lora title font (the PDFs use the built-in Helvetica, so text stays searchable and Latin-1), certificate security print (guilloche, UV plate, hologram, Code128) and PAdES signing, which needs a certificate. The classification of payslips (Restricted), certificates and guarantee policies (Public) is mine: the pack's registry does not list them, so please confirm.
3. Codes in the pack's registry (`forms/forms_registry.json`) must match the platform's references exactly.

Mobile note: the app defaults to a dark theme derived from the navy family (the pack defines none); see `docs/MOBILE_DESIGN.md`.

## Deliberate deviations from the pack's tokens

The pack requires text contrast of at least 4.5:1. Three of its own values fall short, so the code uses the smallest darker value that passes (measured with WCAG relative luminance, see `docs/AUDIT.md`). Revert only if the pack's rule is relaxed.

| Token | Pack value | Used | Why |
| --- | --- | --- | --- |
| `gold-text` (small gold labels on white or the light band) | `#9A7630` (4.19:1 white, 3.90:1 band) | `#8B6B2B` (4.6:1 on both) | Section labels and numbers are small text |
| Form-field border | `#A9B7CA` (2.04:1) | `#788EAC` (3.1:1) | Users must be able to see where a field is (WCAG 1.4.11 asks 3:1) |
| PDF banner for CONFIDENTIAL | `#B26A00` (white text 4.24:1) | `#A96500` (4.6:1) | The banner text is small |
