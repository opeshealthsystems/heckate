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

1. Restyle `web/` (ERP and portals) and `mobile/` with the same tokens and screen designs.
2. Match the platform's PDF layouts to the pack's forms and certificates, with the brand fonts and security elements.
3. Codes in the pack's registry (`forms/forms_registry.json`) must match the platform's references exactly.
