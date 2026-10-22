// Pure colour tokens (no React Native import, so they can be unit-tested for contrast).
// Brand values come from the HEKATE brand pack (navy, navy-deep, navy-raised, navy-line, gold-on-navy, on-navy-muted/faint).
// The pack defines no dark theme: the "Operations" palette below is derived from the brand's own navy family,
// see docs/MOBILE_DESIGN.md for each derived value and its measured contrast.

export interface Palette {
  mode: "dark" | "light";
  // layered surfaces, deepest first
  void: string; bg: string; surface: string; raised: string; line: string; lineSoft: string;
  // text
  fg: string; text2: string; muted: string;
  // accents
  gold: string; goldText: string; goldFill: string; onGold: string;
  // status and severity (text/edge colours; each passes 4.5:1 on bg, surface and raised)
  ok: string; warn: string; bad: string; info: string;
  p1: string; p2: string; p3: string; p4: string;
  // tinted fills behind status text
  okBg: string; warnBg: string; badBg: string; infoBg: string;
  // controls
  inputBorder: string; inputBg: string; scrim: string;
  // legacy names kept so older components keep working
  card: string; brand: string; brandFg: string; navyRaised: string; navyLine: string; onNavyMuted: string; onNavyFaint: string; goldOnNavy: string;
}

const dark: Palette = {
  mode: "dark",
  void: "#020F20", bg: "#041A36", surface: "#062245", raised: "#0E2F5A", line: "#1B3A66", lineSoft: "#12305A",
  fg: "#F2F6FB", text2: "#C9D6E8", muted: "#9FB3CC",
  gold: "#C9A75A", goldText: "#C9A75A", goldFill: "#B8913F", onGold: "#041A36",
  ok: "#5FD398", warn: "#F5A524", bad: "#FF7B72", info: "#7FB2F5",
  p1: "#FF7B72", p2: "#F5A524", p3: "#7FB2F5", p4: "#9FB3CC",
  okBg: "#0B3B35", warnBg: "#3A3010", badBg: "#4A1F2A", infoBg: "#0E2F5A",
  inputBorder: "#4F6D96", inputBg: "#041A36", scrim: "rgba(2,15,32,0.82)",
  card: "#062245", brand: "#062245", brandFg: "#ffffff", navyRaised: "#0E2F5A", navyLine: "#1B3A66", onNavyMuted: "#C9D6E8", onNavyFaint: "#9FB3CC", goldOnNavy: "#C9A75A",
};

// The pack's light tokens, unchanged (with the documented contrast deviations).
const light: Palette = {
  mode: "light",
  void: "#EAF0F7", bg: "#F4F7FB", surface: "#FFFFFF", raised: "#F4F7FB", line: "#DCE4EE", lineSoft: "#E8EEF5",
  fg: "#0B1B33", text2: "#3B4A60", muted: "#5B6675",
  gold: "#B8913F", goldText: "#8B6B2B", goldFill: "#B8913F", onGold: "#062245",
  ok: "#1E6B3E", warn: "#8A5300", bad: "#B3261E", info: "#0E4A8F",
  p1: "#B3261E", p2: "#8A5300", p3: "#0E4A8F", p4: "#5B6675",
  okBg: "#E6F3EA", warnBg: "#FFF3DC", badBg: "#FDF3F2", infoBg: "#E6EEF9",
  inputBorder: "#788EAC", inputBg: "#FFFFFF", scrim: "rgba(244,247,251,0.9)",
  card: "#FFFFFF", brand: "#062245", brandFg: "#ffffff", navyRaised: "#0E2F5A", navyLine: "#1B3A66", onNavyMuted: "#C9D6E8", onNavyFaint: "#9FB3CC", goldOnNavy: "#C9A75A",
};

export const palettes = { dark, light } as const;

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
