// The platform type scale (pure numbers, so it can be tested). System font only: no fontFamily anywhere.
// iOS: Apple HIG text styles at the default Dynamic Type size. Android: Material 3 where it differs (body 16, label 14).
// Only regular, semibold and bold weights are used.

export type Variant = "largeTitle" | "title" | "headline" | "body" | "callout" | "subhead" | "footnote" | "caption";
export interface Style { fontSize: number; lineHeight: number; fontWeight: "400" | "600" | "700" }

export const iosScale: Record<Variant, Style> = {
  largeTitle: { fontSize: 34, lineHeight: 41, fontWeight: "700" },
  title: { fontSize: 22, lineHeight: 28, fontWeight: "700" },
  headline: { fontSize: 17, lineHeight: 22, fontWeight: "600" },
  body: { fontSize: 17, lineHeight: 22, fontWeight: "400" },
  callout: { fontSize: 16, lineHeight: 21, fontWeight: "400" },
  subhead: { fontSize: 15, lineHeight: 20, fontWeight: "400" },
  footnote: { fontSize: 13, lineHeight: 18, fontWeight: "400" },
  caption: { fontSize: 12, lineHeight: 16, fontWeight: "400" },
};

export const androidScale: Record<Variant, Style> = {
  largeTitle: { fontSize: 32, lineHeight: 40, fontWeight: "700" },
  title: { fontSize: 22, lineHeight: 28, fontWeight: "700" },
  headline: { fontSize: 16, lineHeight: 24, fontWeight: "600" },
  body: { fontSize: 16, lineHeight: 24, fontWeight: "400" },
  callout: { fontSize: 16, lineHeight: 24, fontWeight: "400" },
  subhead: { fontSize: 14, lineHeight: 20, fontWeight: "400" },
  footnote: { fontSize: 13, lineHeight: 18, fontWeight: "400" },
  caption: { fontSize: 12, lineHeight: 16, fontWeight: "400" },
};

/** Dense rows and labels stop growing here; everything else follows the system setting up to the same cap. */
export const MAX_FONT_SCALE = 1.4;
