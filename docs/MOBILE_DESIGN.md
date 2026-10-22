# Mobile design: the "Operations" theme

The brand pack (see `BRAND_PACK.md`) defines navy, gold and white, and says nothing about a dark theme. The mobile app now defaults to a dark "Operations" theme derived only from the brand's own navy family; the pack's light tokens remain available under Account > Appearance. Tokens live in `mobile/src/lib/palette.ts`, and `palette.test.ts` checks every text and severity colour at 4.5:1 or more on every surface.

## What is brand, what is derived

| Token | Value | Source |
| --- | --- | --- |
| surface | `#062245` | brand navy |
| bg | `#041A36` | brand navy-deep |
| raised | `#0E2F5A` | brand navy-raised |
| line | `#1B3A66` | brand navy-line |
| void (status strip, tab bar, lock) | `#020F20` | derived: one step darker than navy-deep, so the layers read as depth without a gradient |
| fg / text2 / muted | `#F2F6FB` / `#C9D6E8` / `#9FB3CC` | near-white derived; the other two are the pack's on-navy-muted and on-navy-faint |
| gold text and accents | `#C9A75A` | brand gold-on-navy |
| gold button fill | `#B8913F`, text `#041A36` | brand gold, navy text (measured above 4.5:1) |
| P1 / bad | `#FF7B72` | derived severity colour, red tuned for dark surfaces |
| P2 / warn | `#F5A524` | derived amber |
| P3 / info | `#7FB2F5` | derived light steel blue (no cyan) |
| P4 | `#9FB3CC` | brand on-navy-faint |
| ok | `#5FD398` | derived green |
| status fills | `#0B3B35`, `#3A3010`, `#4A1F2A` | derived dark tints behind status text |
| input border | `#4F6D96` | derived, at least 3:1 on bg (WCAG 1.4.11) |

Severity is never colour alone: every badge carries its P-number, every verdict carries a word and an icon.

Deviations from the pack's "no gradients, glows or shadows" rule: none. Depth comes from layered flat surfaces and hairline borders. The grid and seal watermark are low-opacity textures; the watermark is the seal (`seal-white.png`), and the colour emblem (the app icon) is used on the lock and sign-in screens.

## Typography

Cinzel for large titles, IBM Plex Sans for text, IBM Plex Mono (new dependency `@expo-google-fonts/ibm-plex-mono`) for ids, references, timestamps and numbers.

## Components

- Status strip on every tab: link state (Encrypted when the API URL is https, "No TLS" otherwise, Offline from NetInfo), API host, DEV flag, queued updates, last-updated time. All read from the device, nothing assumed.
- Custom tab bar with a gold indicator and a badge for work waiting to sync.
- Posture card on Home: ring, three stats, severity bar. The staff score is the share of open tickets not overdue, breached or within 4 h of their deadline; the client score is the SLA met rate this month. Both are computed from the data on screen.
- SLA ring (react-native-svg): arc is the share of the resolution window used, colour is ok, warn (75% used or under 4 h) or bad, the centre counts down live (every second on detail, every 30 s in lists). Maths in `lib/sla.ts`.
- Skeleton loaders, pull to refresh, empty and error states, haptics (`expo-haptics`) on presses, tab changes and results, enter animations with `Animated` (react-native-reanimated is not installed). All motion respects the OS reduce-motion setting.

## App lock

`expo-local-authentication`. The lock engages at cold start and after 60 s in the background (`lib/lock-core.ts`), and the app is hidden in the app switcher while inactive. The device prompt accepts biometrics with the device passcode as fallback. With no biometrics enrolled the passcode is used; with no device screen lock at all the lock cannot be enforced and Account says so. Turning the lock on or off needs a successful prompt. The session token stays in SecureStore; the lock only gates the UI.

## Sign-in

Reframed as "Secure access". The copy states only what the code does: OIDC with PKCE in the system browser, token in secure storage, app lock. No certification, "military-grade" or similar claims, no metrics. The development token field stays behind `EXPO_PUBLIC_DEV_LOGIN`.

## Not verified

Biometric prompts, haptics, safe-area insets on a notched device, the app-switcher cover and background timing were not run on a device or simulator; only the web export was inspected.
