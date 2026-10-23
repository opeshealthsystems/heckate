# Mobile design contract

The app looks and behaves like a platform app (Settings, Mail, Wallet, 1Password), calm and quiet, and happens to be for a security company. This replaces the earlier dark "Operations" theme, which the owner rejected. All tokens live in ONE module: `mobile/src/lib/theme.tsx` (hex values and the contrast maths in `palette.ts`, the type scale in `type-scale.ts`). No screen defines a colour, font size or font family.

## 1. Follow the platform

- Appearance: Account > Appearance is System / Light / Dark, default System (`Appearance.setColorScheme`, persisted). `app.json` has `userInterfaceStyle: automatic`.
- Navigation: native stack headers with collapsing large titles on iOS (`headerLargeTitle`), swipe-back, platform back buttons. Tabs are `expo-router/native-tabs` (UITabBarController / Material bottom navigation) with role-hidden triggers. The tab navigator is held until the session is known, so deep links to `/home` or `/documents` resolve (the `landingRoute` and `?next=` fix is unchanged).
- Native controls: Switch, RefreshControl, ActivityIndicator, `@react-native-segmented-control/segmented-control`, header search (`headerSearchBarOptions`) on Tickets and Documents, native modal for New ticket / Report an incident (`/report`, also the deep link), page-sheet Modal for the signature pad, ActionSheetIOS (Alert on Android) for Take / Choose photo, Alert for sign-out confirmation.
- Web preview only: `(tabs)/_layout.web.tsx` uses expo-router `Tabs` with default styling, and `Screen webTitle` draws the large title inline. Neither ships on a phone.

## 2. Colour: one accent

| Token | Light | Dark | iOS native source |
| --- | --- | --- | --- |
| canvas | #FFFFFF | #000000 | systemBackground |
| bg | #F2F2F7 | #000000 | systemGroupedBackground |
| surface | #FFFFFF | #1C1C1E | secondarySystemGroupedBackground |
| separator | #C6C6C8 | #38383A | separator |
| pressed | #D1D1D6 | #2C2C2E | DynamicColorIOS |
| label | #000000 | #FFFFFF | label |
| label2 | #6C6C70 | #98989D | DynamicColorIOS (explicit AA) |
| label3 | #858589 | #7C7C80 | DynamicColorIOS (explicit AA) |
| tint | #062245 | #519AF6 | DynamicColorIOS |
| tintFill (button, switch) | #062245 | #0B65D5 | DynamicColorIOS |
| onTint | #FFFFFF | #FFFFFF | |
| red | #D70015 | #FF453A | DynamicColorIOS (Apple's accessible red, systemRed) |
| orange | #C93400 | #FF9F0A | DynamicColorIOS |
| green | #1E7B34 | #30D158 | DynamicColorIOS |

Derivation of the dark tint: HEKATE navy #062245 is hue 213 degrees, saturation 84%. The dark tint keeps the hue, saturation 90%, lightness 64% = #519AF6 (7.3:1 on black, 5.9:1 on #1C1C1E). A button fill cannot be both readable text on dark and carry a white label, so the dark fill is the same hue at lightness 44% = #0B65D5 (white label 5.5:1; 3.8:1 against black). `palette.test.ts` checks text >= 4.5:1 and components >= 3:1 for every pair, in both modes, on canvas, bg and surface.

Rules: no gold anywhere in the UI chrome. Red only for breached, overdue, error, destructive. Orange only for at risk. Green only for the "Authentic" verdict. Everything else is neutral. Severity is text weight (P1 and P2 semibold) plus one small red dot for P1; never a filled badge. No gradients, glows, coloured shadows, blur, textures or watermark.

Android and web use the hex values (neutral Material-style surfaces); wallpaper-derived dynamic colours are not used because they would replace the brand accent. On iOS, label2 and label3 are also these explicit hex values (DynamicColorIOS), NOT Apple's translucent secondaryLabel/tertiaryLabel (about 3.4:1 on white): accessibility beats pixel-matching Apple's alpha. The tests enforce >= 4.5:1 for label2 and >= 3:1 for label3.

## 3. Typography

System font only (San Francisco, Roboto): no `fontFamily` except below. Scale (`type-scale.ts`): iOS Large title 34/700, Title 22/700, Headline 17/600, Body 17, Callout 16, Subheadline 15, Footnote 13, Caption 12. Android: body 16, title 22, headline 16/600, subhead 14, footnote 13. Weights: regular, semibold, bold. Sentence case; the only uppercase is the grouped-list section header (13 pt, secondaryLabel). Dynamic Type honoured (`allowFontScaling`, `maxFontSizeMultiplier` 1.4, large title 1.2). Monospace (Menlo / monospace) only for references, ids and codes: body size in detail rows, 13 pt secondary under the title in the Documents list.

**Deliberate deviation from the brand pack (needs the owner's sign-off):** the pack says Cinzel for display and IBM Plex for body. Native UI uses system fonts. Cinzel is loaded only for the HEKATE wordmark on the sign-in and lock screens. `@expo-google-fonts/ibm-plex-*` were removed.

## 4. Layout

8-pt grid (`sp`), 16 pt screen margins, column capped at 640 pt on tablets. Main structure: inset grouped lists (`Section` + `Row`): 12 pt radius, rows at least 44 pt, hairline separators inset to the text edge, title Body, subtitle Subheadline in secondaryLabel, trailing value or chevron, at most three lines. No cards in cards, no hero, no status strip. Connection, encryption, server, version, last sync and queued updates are ordinary rows under Account > Connection. A quiet inline banner appears only when offline or updates wait to send.

Row anatomy for tickets: line 1 title (up to 2 lines), line 2 "P1 Critical · Client" (red dot for P1 only), SLA as the trailing value before the chevron ("Overdue 1 h 15 m" red, "3 h 55 m" orange only when at risk, neutral otherwise). Documents: title, then reference in 13 pt monospace, date trailing; only exceptions (Revoked, red) show a status. Segmented controls are never inside a list: they sit full width above it (Tickets filter, Account > Appearance). Home shows at most 3 "Needs attention" rows plus "Show all"; the queue belongs to the Tickets tab, and the Overview count rows open Tickets with the matching scope (open, at risk, assigned to me, incidents). Dates use the system date picker (iOS compact, Android dialog), shown in the device locale, ISO to the API; amounts are grouped as typed.

## 5. Buttons and touch

One primary per screen at most (filled `tintFill`, white label, 50 pt, 12 pt radius, full width in forms). Secondary: plain tinted text. Destructive: red text confirmed with an Alert. Hit targets at least 44 pt. Pressed: system highlight / opacity on iOS, ripple on Android; no scaling. Haptics only on selection change, success, warning and error (existing `lib/haptics.ts`).

## 6. Icons

`<Icon name=... />` maps semantic names to SF Symbols (expo-symbols) on iOS and Material icons (`@expo/vector-icons`) elsewhere. 22 pt in lists, 24 in tabs. The only hand-drawn artwork is the seal.

## 7. Motion

Native transitions, native pull to refresh, ActivityIndicator. No shimmer, pulsing, looping or counting animation. `FadeIn` and `motion.tsx` were deleted.

## 8. Data

SLA is one line (`slaLine` in `lib/sla.ts`): "3 h 59 m left" in secondaryLabel, orange when at risk (75% used or under 4 h), red "Overdue by 1 h 12 m" when breached; lists refresh every 30 s. Home: Needs attention (`needsAttention` in `home-core.ts`), then count rows, then queue. Client Overview: Awaiting your action, counts, balance due, latest documents. Role rules are exactly those of `home-core.ts`.

## 9. Flow

Tabs: Home (My day / Overview), Tickets, Field (engineers), Documents, Account. New ticket is the "+" in the Tickets nav bar (modal) and an empty-state action; `report` is no longer a tab but `/report` still works. Document tap opens `/document/[ref]` with the verdict.

## 10. Sign-in and lock

Flat: seal (96 pt), HEKATE wordmark, one sentence, one primary button. Dev token only behind `DEV_LOGIN`, as a normal grouped field. Lock: seal, "HEKATE is locked", Unlock, plain Sign out.

## Removed

`TabBar`, `charts`, `dash`, `ui`, `PhotoButtons`, `AttachmentList`, `motion`; arc, countdown and posture maths and tests.

## Deliberately not done

iOS 26 native bar-button items; Android dynamic colour.

## Unverified without a phone

The native date pickers (`@react-native-community/datetimepicker`) are untested on a device; the web preview uses the browser date input.

Native tab bar, large-title collapse, PlatformColor / DynamicColorIOS rendering, `headerTintColor` with native colours, Dynamic Type (the web check used 1.3x CSS zoom as a proxy), haptics, biometric prompt, safe areas, Android ripple and Material segmented fallback, header search bar, action sheets.
