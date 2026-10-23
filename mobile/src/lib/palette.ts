// Pure colour tokens (no React Native import, so they can be unit-tested for contrast).
//
// These hex values are (a) the fallback used on the web preview and on Android, and (b) the values the contrast tests check.
// On iOS, theme.tsx maps the neutral tokens to the platform's own semantic colours (PlatformColor) so the real app follows the system.
// Light neutrals equal iOS systemGroupedBackground / secondarySystemGroupedBackground / label; the light semantic colours
// are Apple's "increased contrast" variants (systemRed #D70015 etc.) so text in them reaches 4.5:1.
// Every derivation is written down in docs/MOBILE_DESIGN.md.

export interface Palette {
  mode: "light" | "dark";
  /** screen background (iOS systemGroupedBackground) */
  bg: string;
  /** plain page background (iOS systemBackground): sign-in, lock, sheets */
  canvas: string;
  /** grouped list container, sheets, fields (iOS secondarySystemGroupedBackground) */
  surface: string;
  /** hairline between rows (iOS separator, opaque) */
  separator: string;
  /** row highlight while pressed (iOS cell selection) */
  pressed: string;
  /** primary text (label) */
  label: string;
  /** secondary text (secondaryLabel) */
  label2: string;
  /** placeholders, chevrons, disabled (tertiaryLabel; UI-component contrast, not body text) */
  label3: string;
  /** the one brand accent for text, links, icons, selected tab */
  tint: string;
  /** the same brand blue as a fill (primary button, switch track); `onTint` is the label on it */
  tintFill: string;
  onTint: string;
  /** semantic colours, rare: breached or error or destructive / at risk / confirmed success */
  red: string;
  orange: string;
  green: string;
}

const light: Palette = {
  mode: "light",
  canvas: "#FFFFFF", bg: "#F2F2F7", surface: "#FFFFFF", separator: "#C6C6C8", pressed: "#D1D1D6",
  label: "#000000", label2: "#6C6C70", label3: "#858589",
  tint: "#062245", tintFill: "#062245", onTint: "#FFFFFF",
  red: "#D70015", orange: "#C93400", green: "#1E7B34",
};

const dark: Palette = {
  mode: "dark",
  canvas: "#000000", bg: "#000000", surface: "#1C1C1E", separator: "#38383A", pressed: "#2C2C2E",
  label: "#FFFFFF", label2: "#98989D", label3: "#7C7C80",
  // HEKATE navy #062245 is hue 213 deg, saturation 84%. Dark tint: same hue, saturation 90%, lightness 64% = #519AF6.
  // Fill: same hue, lightness 44% = #0B65D5 (white label 5.5:1, 3.8:1 against black).
  tint: "#519AF6", tintFill: "#0B65D5", onTint: "#FFFFFF",
  red: "#FF453A", orange: "#FF9F0A", green: "#30D158",
};

export const palettes = { light, dark } as const;

function lum(hex: string): number {
  const n = parseInt(hex.slice(1), 16);
  const [r, g, b] = [(n >> 16) & 255, (n >> 8) & 255, n & 255].map((v) => {
    const x = v / 255;
    return x <= 0.03928 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/** WCAG 2.x contrast ratio between two #RRGGBB colours. */
export function contrast(a: string, b: string): number {
  const x = lum(a), y = lum(b);
  return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
}
