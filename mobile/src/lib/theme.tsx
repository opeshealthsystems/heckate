import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { AccessibilityInfo, Appearance, DynamicColorIOS, Platform, PlatformColor, StyleSheet, useColorScheme, type ColorValue, type TextStyle } from "react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { palettes, type Palette } from "./palette";
import { androidScale, iosScale, type Variant } from "./type-scale";

// The ONE token module. No screen defines its own colour, font size or spacing.
//
//  iOS      neutral tokens are the platform's semantic colours (PlatformColor); tint and the three semantic colours are
//           DynamicColorIOS pairs from palette.ts, so everything follows the system (or the Account > Appearance override).
//  Android  Material-style neutrals as plain hex from palette.ts, picked by the resolved scheme (see MOBILE_DESIGN.md).
//  Web      the same hex: this is only the preview, equal to the iOS system values.

export type Preference = "system" | "light" | "dark";
export type Scheme = "light" | "dark";

/** What a colour token is: a string on Android and web, a native colour object on iOS. Use it wherever a colour is accepted. */
export type Colors = { [K in keyof Omit<Palette, "mode">]: ColorValue };

function iosColors(): Colors {
  const dyn = (k: keyof Omit<Palette, "mode">) => DynamicColorIOS({ light: palettes.light[k], dark: palettes.dark[k] });
  return {
    canvas: PlatformColor("systemBackground"),
    bg: PlatformColor("systemGroupedBackground"),
    surface: PlatformColor("secondarySystemGroupedBackground"),
    separator: PlatformColor("separator"),
    pressed: dyn("pressed"),
    label: PlatformColor("label"),
    label2: PlatformColor("secondaryLabel"),
    label3: PlatformColor("tertiaryLabel"),
    tint: dyn("tint"),
    tintFill: dyn("tintFill"),
    onTint: dyn("onTint"),
    red: dyn("red"),
    orange: dyn("orange"),
    green: dyn("green"),
  };
}

function plainColors(p: Palette): Colors {
  const { mode: _m, ...rest } = p;
  return rest;
}

/** 8-pt grid. Screens use 16 pt side margins (`sp.lg`). */
export const sp = { xs: 4, sm: 8, md: 12, lg: 16, xl: 24, xxl: 32 } as const;
export const radius = { field: 10, group: 12, button: 12 } as const;
/** A true hairline: 0.5 pt on iOS and Android, and on the web preview where hairlineWidth would be a full pixel. */
export const hairline = Platform.OS === "web" ? 0.5 : StyleSheet.hairlineWidth;
export const MIN_TARGET = 44;
export const ROW_MIN = 44;
export const BUTTON_HEIGHT = 50;

const PREF_KEY = "hekate.appearance";

interface Ctx { pref: Preference; setPref: (p: Preference) => void; scheme: Scheme; reduceMotion: boolean }
const ThemeCtx = createContext<Ctx>({ pref: "system", setPref: () => {}, scheme: "light", reduceMotion: false });

function applyOverride(p: Preference) {
  try {
    // null hands control back to the system. (Not available on every web runtime: the preview then follows the browser.)
    (Appearance as unknown as { setColorScheme?: (s: Scheme | null) => void }).setColorScheme?.(p === "system" ? null : p);
  } catch {
    /* ignore */
  }
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [pref, setPrefState] = useState<Preference>("system");
  const [reduceMotion, setReduce] = useState(false);
  const system = useColorScheme();
  useEffect(() => {
    AsyncStorage.getItem(PREF_KEY).then((v) => {
      if (v === "light" || v === "dark" || v === "system") { setPrefState(v); applyOverride(v); }
    }).catch(() => {});
    AccessibilityInfo.isReduceMotionEnabled().then(setReduce).catch(() => {});
    const sub = AccessibilityInfo.addEventListener("reduceMotionChanged", setReduce);
    return () => sub.remove();
  }, []);
  const setPref = useCallback((p: Preference) => {
    setPrefState(p);
    applyOverride(p);
    AsyncStorage.setItem(PREF_KEY, p).catch(() => {});
  }, []);
  const scheme: Scheme = pref === "system" ? (system === "dark" ? "dark" : "light") : pref;
  const value = useMemo(() => ({ pref, setPref, scheme, reduceMotion }), [pref, setPref, scheme, reduceMotion]);
  return <ThemeCtx.Provider value={value}>{children}</ThemeCtx.Provider>;
}

export const useReduceMotion = () => useContext(ThemeCtx).reduceMotion;

const scale = Platform.OS === "android" ? androidScale : iosScale;

/** Text style for a platform text-style name. No fontFamily: San Francisco on iOS, Roboto on Android. */
export const typeStyle = (v: Variant): TextStyle => scale[v];

export function useTheme() {
  const { pref, setPref, scheme, reduceMotion } = useContext(ThemeCtx);
  const c = useMemo<Colors>(() => (Platform.OS === "ios" ? iosColors() : plainColors(palettes[scheme])), [scheme]);
  return { c, scheme, pref, setPref, reduceMotion };
}

/** Monospace is only for references, ids and codes, at body size. */
export const monoFamily = Platform.select({ ios: "Menlo", android: "monospace", default: "ui-monospace, Menlo, monospace" });
