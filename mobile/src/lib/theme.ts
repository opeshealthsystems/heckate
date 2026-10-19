import { StyleSheet, useColorScheme } from "react-native";

const light = { bg: "#f6f7f9", fg: "#14181f", muted: "#5b6573", card: "#ffffff", line: "#dde1e7", brand: "#0b3d91", brandFg: "#ffffff", ok: "#0f7b3f", bad: "#b3261e", warn: "#9a5b00" };
const dark = { bg: "#0e1116", fg: "#e8ebf0", muted: "#98a2b3", card: "#171b22", line: "#2a303a", brand: "#6ea0ff", brandFg: "#0e1116", ok: "#4cc38a", bad: "#ff8a80", warn: "#f0b24a" };
export type Theme = typeof light;

export function useTheme() {
  const c = useColorScheme() === "dark" ? dark : light;
  const s = StyleSheet.create({
    screen: { flex: 1, backgroundColor: c.bg },
    pad: { padding: 16, gap: 12 },
    card: { backgroundColor: c.card, borderColor: c.line, borderWidth: 1, borderRadius: 10, padding: 14, gap: 4 },
    h1: { color: c.fg, fontSize: 24, fontWeight: "700" },
    text: { color: c.fg, fontSize: 16 },
    muted: { color: c.muted, fontSize: 14 },
    label: { color: c.fg, fontSize: 15, fontWeight: "600", marginTop: 8 },
    input: { minHeight: 48, borderWidth: 1, borderColor: c.line, borderRadius: 8, paddingHorizontal: 12, paddingVertical: 10, color: c.fg, backgroundColor: c.card, fontSize: 16 },
    btn: { minHeight: 48, borderRadius: 8, alignItems: "center", justifyContent: "center", paddingHorizontal: 16, borderWidth: 1, borderColor: c.line, backgroundColor: c.card },
    btnPrimary: { backgroundColor: c.brand, borderColor: c.brand },
    btnText: { color: c.fg, fontSize: 16, fontWeight: "600" },
    btnPrimaryText: { color: c.brandFg, fontSize: 16, fontWeight: "600" },
    error: { color: c.bad, fontSize: 14 },
    row: { flexDirection: "row", gap: 8, flexWrap: "wrap", alignItems: "center" },
  });
  return { c, s };
}
