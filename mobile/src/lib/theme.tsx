import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { AccessibilityInfo, Platform, StyleSheet } from "react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { palettes, type Palette } from "./palette";

// HEKATE brand tokens live in palette.ts. The default is the derived dark "Operations" theme
// (brand navy family + gold); the pack's own light tokens stay available from Account > Appearance.
export type Theme = Palette;
export type Mode = "dark" | "light";

export const font = {
  display: "Cinzel_700Bold",
  body: "IBMPlexSans_400Regular",
  medium: "IBMPlexSans_500Medium",
  semibold: "IBMPlexSans_600SemiBold",
  mono: "IBMPlexMono_500Medium",
  monoBold: "IBMPlexMono_600SemiBold",
};

/** Spacing (4-pt grid) and radii shared by every screen. */
export const sp = { xs: 4, sm: 8, md: 12, lg: 16, xl: 24 } as const;
export const radius = { sm: 4, md: 8, lg: 12, pill: 999 } as const;

const MODE_KEY = "hekate.theme";

interface Ctx { mode: Mode; setMode: (m: Mode) => void; reduceMotion: boolean }
const ThemeCtx = createContext<Ctx>({ mode: "dark", setMode: () => {}, reduceMotion: false });

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [mode, setModeState] = useState<Mode>("dark");
  const [reduceMotion, setReduce] = useState(false);
  useEffect(() => {
    AsyncStorage.getItem(MODE_KEY).then((v) => { if (v === "light" || v === "dark") setModeState(v); }).catch(() => {});
    AccessibilityInfo.isReduceMotionEnabled().then(setReduce).catch(() => {});
    const sub = AccessibilityInfo.addEventListener("reduceMotionChanged", setReduce);
    return () => sub.remove();
  }, []);
  const setMode = useCallback((m: Mode) => {
    setModeState(m);
    AsyncStorage.setItem(MODE_KEY, m).catch(() => {});
  }, []);
  const value = useMemo(() => ({ mode, setMode, reduceMotion }), [mode, setMode, reduceMotion]);
  return <ThemeCtx.Provider value={value}>{children}</ThemeCtx.Provider>;
}

export const useReduceMotion = () => useContext(ThemeCtx).reduceMotion;

function build(c: Palette) {
  return StyleSheet.create({
    screen: { flex: 1, backgroundColor: c.bg },
    pad: { padding: sp.lg, gap: sp.md },
    card: { backgroundColor: c.surface, borderColor: c.line, borderWidth: StyleSheet.hairlineWidth * 2, borderRadius: radius.md, padding: sp.lg, gap: sp.xs },
    h1: { color: c.fg, fontSize: 26, lineHeight: 32, fontFamily: font.display },
    h2: { color: c.fg, fontSize: 18, lineHeight: 24, fontFamily: font.semibold },
    text: { color: c.fg, fontSize: 16, lineHeight: 22, fontFamily: font.body },
    muted: { color: c.muted, fontSize: 14, lineHeight: 20, fontFamily: font.body },
    overline: { color: c.muted, fontSize: 11, lineHeight: 14, fontFamily: font.semibold, textTransform: "uppercase", letterSpacing: 1.3 },
    mono: { color: c.text2, fontSize: 12, lineHeight: 16, fontFamily: font.mono, letterSpacing: 0.2 },
    label: { color: c.text2, fontSize: 12, fontFamily: font.semibold, marginTop: sp.sm, textTransform: "uppercase", letterSpacing: 1 },
    input: { minHeight: 48, borderWidth: 1, borderColor: c.inputBorder, borderRadius: radius.md, paddingHorizontal: sp.md, paddingVertical: 10, color: c.fg, backgroundColor: c.inputBg, fontSize: 16, fontFamily: font.body },
    semibold: { fontFamily: font.semibold },
    error: { color: c.bad, fontSize: 14, lineHeight: 20, fontFamily: font.body },
    row: { flexDirection: "row", gap: sp.sm, flexWrap: "wrap", alignItems: "center" },
    // legacy button styles (components/ui Button supersedes these)
    btn: { minHeight: 48, borderRadius: radius.md, alignItems: "center", justifyContent: "center", paddingHorizontal: sp.lg, borderWidth: 1, borderColor: c.inputBorder, backgroundColor: c.surface },
    btnPrimary: { backgroundColor: c.goldFill, borderColor: c.goldFill },
    btnText: { color: c.fg, fontSize: 16, fontFamily: font.semibold },
    btnPrimaryText: { color: c.onGold, fontSize: 16, fontFamily: font.semibold },
  });
}

export function useTheme() {
  const { mode, setMode, reduceMotion } = useContext(ThemeCtx);
  const c = palettes[mode];
  const s = useMemo(() => build(c), [c]);
  return { c, s, mode, setMode, reduceMotion, web: Platform.OS === "web" };
}
