import { StyleSheet } from "react-native";

// Brand tokens (HEKATE brand pack): navy + gold + white; status colours for pills only.
// The brand defines no dark theme, so the app is always light (app.json: userInterfaceStyle "light").
const c = {
  bg: "#F4F7FB", fg: "#0B1B33", muted: "#5B6675", card: "#ffffff", line: "#DCE4EE", inputBorder: "#A9B7CA",
  brand: "#062245", brandFg: "#ffffff", navyRaised: "#0E2F5A", navyLine: "#1B3A66", onNavyMuted: "#C9D6E8",
  gold: "#B8913F", goldText: "#9A7630", goldOnNavy: "#C9A75A",
  ok: "#1E6B3E", bad: "#B3261E", warn: "#8A5300",
};
export type Theme = typeof c;

export const font = {
  display: "Cinzel_700Bold",
  body: "IBMPlexSans_400Regular",
  medium: "IBMPlexSans_500Medium",
  semibold: "IBMPlexSans_600SemiBold",
};

export function useTheme() {
  const s = StyleSheet.create({
    screen: { flex: 1, backgroundColor: c.bg },
    pad: { padding: 16, gap: 12 },
    card: { backgroundColor: c.card, borderColor: c.line, borderWidth: 1, borderRadius: 10, padding: 14, gap: 4 },
    h1: { color: c.brand, fontSize: 24, fontFamily: font.display },
    text: { color: c.fg, fontSize: 16, fontFamily: font.body },
    muted: { color: c.muted, fontSize: 14, fontFamily: font.body },
    label: { color: c.fg, fontSize: 15, fontFamily: font.semibold, marginTop: 8 },
    input: { minHeight: 48, borderWidth: 1, borderColor: c.inputBorder, borderRadius: 6, paddingHorizontal: 12, paddingVertical: 10, color: c.fg, backgroundColor: c.card, fontSize: 16, fontFamily: font.body },
    btn: { minHeight: 48, borderRadius: 6, alignItems: "center", justifyContent: "center", paddingHorizontal: 16, borderWidth: 1, borderColor: c.inputBorder, backgroundColor: c.card },
    btnPrimary: { backgroundColor: c.brand, borderColor: c.brand },
    btnText: { color: c.brand, fontSize: 16, fontFamily: font.semibold },
    btnPrimaryText: { color: c.brandFg, fontSize: 16, fontFamily: font.semibold },
    error: { color: c.bad, fontSize: 14, fontFamily: font.body },
    row: { flexDirection: "row", gap: 8, flexWrap: "wrap", alignItems: "center" },
  });
  return { c, s };
}
