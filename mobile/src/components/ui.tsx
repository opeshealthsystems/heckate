import { useEffect, useRef, useState, type ReactNode } from "react";
import { ActivityIndicator, Animated, Easing, Image, Pressable, StyleSheet, Text, TextInput, View, type PressableProps, type TextInputProps, type ViewStyle } from "react-native";
import { useNetInfo } from "@react-native-community/netinfo";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import Svg, { Circle, Defs, Path, Pattern, Rect } from "react-native-svg";
import { arcDash, countdown, slaRemaining, slaRing, type SlaState } from "../lib/sla";
import { API_URL } from "../lib/api";
import { DEV_LOGIN } from "../lib/auth";
import { haptic } from "../lib/haptics";
import { useNow } from "../lib/motion";
import { useSession } from "../lib/session";
import { font, radius, sp, useReduceMotion, useTheme } from "../lib/theme";
import type { Palette } from "../lib/palette";
import type { Intervention, Severity, Ticket } from "../lib/types";
import { Icon, type IconName } from "./Icon";

const AnimatedCircle = Animated.createAnimatedComponent(Circle);

export type Tone = "ok" | "bad" | "warn" | "info" | "gold" | "neutral";

export const toneColor = (c: Palette, tone: Tone | undefined): string =>
  tone === "ok" ? c.ok : tone === "bad" ? c.bad : tone === "warn" ? c.warn : tone === "info" ? c.info : tone === "gold" ? c.goldText : c.muted;
const toneBg = (c: Palette, tone: Tone | undefined): string =>
  tone === "ok" ? c.okBg : tone === "bad" ? c.badBg : tone === "warn" ? c.warnBg : tone === "info" ? c.infoBg : tone === "gold" ? c.raised : c.raised;

export const sevColor = (c: Palette, s: Severity): string => c[("p" + s.slice(1)) as "p1" | "p2" | "p3" | "p4"];
export const stateColor = (c: Palette, s: SlaState): string => (s === "ok" || s === "met" ? c.ok : s === "warn" ? c.warn : s === "bad" || s === "missed" ? c.bad : c.muted);

/* ───────────────────────── backdrop: hairline grid + seal watermark ───────────────────────── */

export function Backdrop() {
  const { c } = useTheme();
  return (
    <View pointerEvents="none" style={StyleSheet.absoluteFill} accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
      <Svg width="100%" height="100%" style={StyleSheet.absoluteFill}>
        <Defs>
          <Pattern id="grid" patternUnits="userSpaceOnUse" width={32} height={32}>
            <Path d="M32 0H0V32" stroke={c.line} strokeWidth={0.6} fill="none" opacity={c.mode === "dark" ? 0.38 : 0.55} />
          </Pattern>
        </Defs>
        <Rect width="100%" height="100%" fill="url(#grid)" />
      </Svg>
      {c.mode === "dark" && (
        <Image source={require("../../assets/seal-white.png")} style={{ position: "absolute", right: -70, bottom: 90, width: 300, height: 300, opacity: 0.045 }} resizeMode="contain" />
      )}
    </View>
  );
}

/** Common frame of a screen: backdrop, safe-area top inset, and the live status strip. */
export function Screen({ children, updated, noStrip }: { children: ReactNode; updated?: Date | null; noStrip?: boolean }) {
  const { c } = useTheme();
  const insets = useSafeAreaInsets();
  return (
    <View style={{ flex: 1, backgroundColor: c.bg, paddingTop: insets.top }}>
      <Backdrop />
      {!noStrip && <StatusStrip updated={updated} />}
      {children}
    </View>
  );
}

/* ───────────────────────── status strip ───────────────────────── */

const hostOf = (u: string) => {
  try { return new URL(u).host; } catch { return u; }
};
const stamp = (d: Date) => d.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit", second: "2-digit" });

/** Environment, link security, connection, sync state and last-updated time. Everything here is read from the device, nothing is assumed. */
export function StatusStrip({ updated }: { updated?: Date | null }) {
  const { c, s } = useTheme();
  const net = useNetInfo();
  const { pendingDrafts, pendingActions } = useSession();
  const queued = pendingDrafts + pendingActions;
  const online = net.isConnected !== false;
  const tls = API_URL.startsWith("https://");
  const link = !online ? c.bad : tls ? c.ok : c.warn;
  const cell = { flexDirection: "row", alignItems: "center", gap: 5 } as const;
  const txt = [s.mono, { fontSize: 10.5, letterSpacing: 0.6, textTransform: "uppercase" as const }];
  return (
    <View style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: sp.md, minHeight: 30, paddingHorizontal: sp.lg, backgroundColor: c.void, borderBottomColor: c.line, borderBottomWidth: StyleSheet.hairlineWidth * 2 }} accessibilityRole="summary" accessibilityLabel={`${online ? "Online" : "Offline"}, ${queued ? `${queued} updates queued` : "synced"}`}>
      <View style={[cell, { flexShrink: 1 }]}>
        <View style={{ width: 7, height: 7, borderRadius: 4, backgroundColor: link }} />
        <Text style={[txt, { color: link }]}>{!online ? "Offline" : tls ? "Encrypted" : "No TLS"}</Text>
        <Text style={[txt, { color: c.muted, flexShrink: 1 }]} numberOfLines={1}>{hostOf(API_URL)}</Text>
        {DEV_LOGIN && <Text style={[txt, { color: c.warn }]}>DEV</Text>}
      </View>
      <View style={cell}>
        <Icon name="sync" size={11} color={queued ? c.warn : c.muted} stroke={2} />
        <Text style={[txt, { color: queued ? c.warn : c.muted }]}>{queued ? `${queued} queued` : "Synced"}</Text>
        {updated && <Text style={[txt, { color: c.muted }]}> · {stamp(updated)}</Text>}
      </View>
    </View>
  );
}

/* ───────────────────────── typography blocks ───────────────────────── */

export function ScreenHeader({ overline, title, right }: { overline?: string; title: string; right?: ReactNode }) {
  const { c, s } = useTheme();
  return (
    <View style={{ flexDirection: "row", alignItems: "flex-end", justifyContent: "space-between", gap: sp.md, paddingTop: sp.sm, paddingBottom: sp.xs }}>
      <View style={{ flex: 1, gap: 4 }}>
        {!!overline && <Text style={[s.overline, { color: c.goldText }]}>{overline}</Text>}
        <Text style={[s.h1, { fontSize: 30, lineHeight: 36 }]} accessibilityRole="header">{title}</Text>
      </View>
      {right}
    </View>
  );
}

/** Stack-screen header: back button, mono reference, title. */
export function DetailHeader({ onBack, overline, title, right }: { onBack: () => void; overline?: string; title: string; right?: ReactNode }) {
  const { c, s } = useTheme();
  return (
    <View style={{ gap: sp.sm, paddingBottom: sp.xs }}>
      <View style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}>
        <Pressable accessibilityRole="button" accessibilityLabel="Back" onPress={() => { haptic.tap(); onBack(); }} hitSlop={8} style={({ pressed }) => ({ minHeight: 44, minWidth: 44, flexDirection: "row", alignItems: "center", gap: 2, opacity: pressed ? 0.6 : 1 })}>
          <Icon name="back" size={22} color={c.goldText} />
          <Text style={{ color: c.goldText, fontSize: 15, fontFamily: font.semibold }}>Back</Text>
        </Pressable>
        {right}
      </View>
      {!!overline && <Text style={[s.mono, { color: c.goldText, fontSize: 12 }]} selectable>{overline}</Text>}
      <Text style={[s.h1, { fontSize: 24, lineHeight: 30 }]} accessibilityRole="header">{title}</Text>
    </View>
  );
}

/* ───────────────────────── controls ───────────────────────── */

type Variant = "primary" | "secondary" | "danger" | "ghost";
export function Button({ title, onPress, primary, variant, disabled, loading, icon, small, style, accessibilityLabel }: { accessibilityLabel?: string; title: string; onPress: () => void; primary?: boolean; variant?: Variant; disabled?: boolean; loading?: boolean; icon?: IconName; small?: boolean; style?: ViewStyle }) {
  const { c } = useTheme();
  const v: Variant = variant ?? (primary ? "primary" : "secondary");
  const bg = v === "primary" ? c.goldFill : v === "danger" ? c.badBg : v === "ghost" ? "transparent" : c.raised;
  const border = v === "primary" ? c.goldFill : v === "danger" ? c.bad : v === "ghost" ? "transparent" : c.inputBorder;
  const fg = v === "primary" ? c.onGold : v === "danger" ? c.bad : v === "ghost" ? c.goldText : c.fg;
  const off = disabled || loading;
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel}
      accessibilityState={{ disabled: !!off, busy: !!loading }}
      disabled={off}
      onPress={() => { (v === "primary" ? haptic.press : haptic.tap)(); onPress(); }}
      style={({ pressed }) => [{ minHeight: small ? 44 : 50, borderRadius: radius.md, flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 8, paddingHorizontal: sp.lg, borderWidth: 1, backgroundColor: bg, borderColor: border, opacity: off ? 0.45 : pressed ? 0.82 : 1, transform: [{ scale: pressed ? 0.98 : 1 }] }, style]}
    >
      {loading ? <ActivityIndicator size="small" color={fg} /> : icon ? <Icon name={icon} size={18} color={fg} /> : null}
      <Text style={{ color: fg, fontSize: small ? 14 : 16, fontFamily: font.semibold, letterSpacing: 0.2 }}>{title}</Text>
    </Pressable>
  );
}

export function Input({ label, hint, mono, ...p }: TextInputProps & { label?: string; hint?: string; mono?: boolean }) {
  const { c, s } = useTheme();
  const [focus, setFocus] = useState(false);
  return (
    <View style={{ gap: 6 }}>
      {!!label && <Text style={s.label}>{label}</Text>}
      <TextInput
        accessibilityLabel={label}
        placeholderTextColor={c.muted}
        {...p}
        onFocus={(e) => { setFocus(true); p.onFocus?.(e); }}
        onBlur={(e) => { setFocus(false); p.onBlur?.(e); }}
        style={[s.input, mono && { fontFamily: font.mono, fontSize: 14 }, focus && { borderColor: c.gold, borderWidth: 1.5 }, p.multiline && { minHeight: 110, textAlignVertical: "top" }, p.style]}
      />
      {!!hint && <Text style={s.muted}>{hint}</Text>}
    </View>
  );
}

export function Segmented<T extends string>({ options, value, onChange, counts }: { options: readonly T[]; value: T; onChange: (v: T) => void; counts?: Partial<Record<T, number>> }) {
  const { c } = useTheme();
  return (
    <View accessibilityRole="tablist" style={{ flexDirection: "row", backgroundColor: c.void, borderRadius: radius.md, padding: 3, borderColor: c.line, borderWidth: StyleSheet.hairlineWidth * 2 }}>
      {options.map((o) => {
        const on = o === value;
        return (
          <Pressable key={o} accessibilityRole="tab" accessibilityState={{ selected: on }} onPress={() => { haptic.select(); onChange(o); }} style={{ flex: 1, minHeight: 44, borderRadius: radius.sm + 2, alignItems: "center", justifyContent: "center", flexDirection: "row", gap: 6, backgroundColor: on ? c.raised : "transparent", borderColor: on ? c.gold : "transparent", borderWidth: 1 }}>
            <Text style={{ color: on ? c.fg : c.muted, fontSize: 14, fontFamily: font.semibold }}>{o}</Text>
            {counts?.[o] !== undefined && <Text style={{ color: on ? c.goldText : c.muted, fontSize: 12, fontFamily: font.mono }}>{counts[o]}</Text>}
          </Pressable>
        );
      })}
    </View>
  );
}

/* ───────────────────────── badges ───────────────────────── */

export function Pill({ text, tone, icon }: { text: string; tone?: "ok" | "bad" | "warn" | "info" | "gold" | "neutral"; icon?: IconName }) {
  const { c } = useTheme();
  const color = toneColor(c, tone);
  return (
    <View style={{ flexDirection: "row", alignItems: "center", gap: 4, backgroundColor: toneBg(c, tone), borderColor: color, borderWidth: StyleSheet.hairlineWidth * 2, borderRadius: radius.pill, paddingHorizontal: 8, paddingVertical: 2, minHeight: 22 }}>
      {icon && <Icon name={icon} size={11} color={color} stroke={2.2} />}
      <Text style={{ color, fontSize: 11, fontFamily: font.semibold, letterSpacing: 0.6, textTransform: "uppercase" }}>{text}</Text>
    </View>
  );
}

/** Square severity badge: the colour is never the only cue, the P-number is always there. */
export function SevBadge({ sev, size = 26 }: { sev: Severity; size?: number }) {
  const { c } = useTheme();
  const col = sevColor(c, sev);
  return (
    <View accessible accessibilityLabel={`Severity ${sev}`} style={{ width: size, height: size, borderRadius: radius.sm, borderWidth: 1, borderColor: col, backgroundColor: sev === "P1" ? c.badBg : sev === "P2" ? c.warnBg : c.raised, alignItems: "center", justifyContent: "center" }}>
      <Text style={{ color: col, fontSize: size * 0.43, fontFamily: font.monoBold }}>{sev}</Text>
    </View>
  );
}

export const visitTone = (st: Intervention["status"]) => (st === "completed" ? "ok" : st === "in_progress" ? "info" : st === "cancelled" ? "neutral" : "gold") as "ok" | "info" | "neutral" | "gold";
export const visitColor = (c: Palette, st: Intervention["status"]) => (st === "completed" ? c.ok : st === "in_progress" ? c.info : st === "cancelled" ? c.muted : c.gold);
export const SEV_NAME: Record<Severity, string> = { P1: "Critical", P2: "High", P3: "Medium", P4: "Low" };

/** Compact live SLA indicator for lists: dot + remaining time (minute precision, refreshed every 15 s). */
export function SlaChip({ t }: { t: Ticket }) {
  const { c, s } = useTheme();
  const open = !t.resolved_at;
  const now = useNow(open ? 15_000 : 0);
  const r = slaRing(t, now);
  if (r.state === "none") return null;
  const col = stateColor(c, r.state);
  const text = open ? slaRemaining(t.sla_due_at, now)?.label ?? "" : r.state === "missed" ? "SLA missed" : "SLA met";
  return (
    <View style={{ flexDirection: "row", alignItems: "center", gap: 5 }}>
      <View style={{ width: 6, height: 6, borderRadius: 3, backgroundColor: col }} />
      <Text style={[s.mono, { color: col, fontSize: 11.5 }]}>{text}</Text>
    </View>
  );
}

export function TicketPills({ t }: { t: Ticket }) {
  const { s } = useTheme();
  return (
    <View style={[s.row, { gap: 8 }]}>
      <SevBadge sev={t.severity} />
      <Pill text={t.status.replace("_", " ")} tone={t.status === "closed" ? "neutral" : t.status === "resolved" ? "ok" : t.status === "in_progress" ? "info" : "gold"} />
      <SlaChip t={t} />
    </View>
  );
}

/* ───────────────────────── surfaces ───────────────────────── */

export function Card({ children, style, tone, rail }: { children: ReactNode; style?: ViewStyle; tone?: Tone; rail?: string }) {
  const { c } = useTheme();
  return (
    <View style={[{ backgroundColor: c.surface, borderColor: tone ? toneColor(c, tone) : c.line, borderWidth: StyleSheet.hairlineWidth * 2, borderRadius: radius.md, padding: sp.lg, gap: sp.xs, overflow: "hidden" }, style]}>
      {rail && <View style={{ position: "absolute", left: 0, top: 0, bottom: 0, width: 3, backgroundColor: rail }} />}
      {children}
    </View>
  );
}

/** Tappable row card with an optional severity rail, used by the ticket and visit lists. */
export function PressCard({ children, onPress, rail, label }: { children: ReactNode; onPress: () => void; rail?: string; label?: string }) {
  const { c } = useTheme();
  return (
    <Pressable accessibilityRole="button" accessibilityLabel={label} onPress={() => { haptic.tap(); onPress(); }} style={({ pressed }) => ({ backgroundColor: pressed ? c.raised : c.surface, borderColor: pressed ? c.gold : c.line, borderWidth: StyleSheet.hairlineWidth * 2, borderRadius: radius.md, padding: sp.lg, paddingLeft: sp.lg + (rail ? 4 : 0), gap: 8, overflow: "hidden", transform: [{ scale: pressed ? 0.992 : 1 }] })}>
      {rail && <View style={{ position: "absolute", left: 0, top: 0, bottom: 0, width: 3, backgroundColor: rail }} />}
      {children}
    </Pressable>
  );
}

export function KV({ k, v, mono }: { k: string; v: string; mono?: boolean }) {
  const { s, c } = useTheme();
  return (
    <View style={{ flexDirection: "row", justifyContent: "space-between", gap: sp.md, paddingVertical: 6, borderBottomColor: c.lineSoft, borderBottomWidth: StyleSheet.hairlineWidth * 2 }}>
      <Text style={s.overline}>{k}</Text>
      <Text style={[mono ? s.mono : s.muted, { flexShrink: 1, textAlign: "right", color: c.text2 }]} selectable>{v}</Text>
    </View>
  );
}

/* ───────────────────────── SLA ring ───────────────────────── */

/**
 * Ring of a ticket's resolution window: the arc is the share of time used, the colour is the state,
 * the centre shows the live countdown (seconds tick every second while the ticket is open).
 */
export function SlaRingView({ t, size = 148, now: nowProp }: { t: Pick<Ticket, "created_at" | "sla_due_at" | "resolved_at" | "sla_breached">; size?: number; now?: number }) {
  const { c, s } = useTheme();
  const open = !t.resolved_at;
  const tick = useNow(open && nowProp === undefined ? (size >= 120 ? 1000 : 30_000) : 0);
  const now = nowProp ?? tick;
  const r = slaRing(t, now);
  const col = stateColor(c, r.state);
  const stroke = Math.max(5, size * 0.075);
  const rad = (size - stroke) / 2;
  const { circumference } = arcDash(r.used, rad);
  const anim = useRef(new Animated.Value(0)).current;
  const reduce = useReduceMotion();
  useEffect(() => {
    if (reduce) { anim.setValue(r.used); return; }
    Animated.timing(anim, { toValue: r.used, duration: 900, easing: Easing.out(Easing.cubic), useNativeDriver: false }).start();
  }, [r.used, reduce, anim]);
  const offset = anim.interpolate({ inputRange: [0, 1], outputRange: [circumference, 0] });
  const label = r.state === "none" ? "No SLA" : r.state === "met" ? "SLA met" : r.state === "missed" ? "SLA missed" : r.remainingMs < 0 ? "Overdue" : "Remaining";
  const big = size >= 120;
  return (
    <View accessible accessibilityLabel={`SLA ${label}${r.state === "none" ? "" : ` ${countdown(r.remainingMs)}`}`} style={{ width: size, height: size, alignItems: "center", justifyContent: "center" }}>
      <Svg width={size} height={size} style={StyleSheet.absoluteFill}>
        <Circle cx={size / 2} cy={size / 2} r={rad} stroke={c.line} strokeWidth={stroke} fill="none" opacity={0.7} />
        {/* tick marks every quarter */}
        {[0, 1, 2, 3].map((i) => {
          const a = (i * Math.PI) / 2;
          const x1 = size / 2 + Math.sin(a) * (rad - stroke);
          const y1 = size / 2 - Math.cos(a) * (rad - stroke);
          const x2 = size / 2 + Math.sin(a) * (rad - stroke - 5);
          const y2 = size / 2 - Math.cos(a) * (rad - stroke - 5);
          return <Path key={i} d={`M${x1} ${y1}L${x2} ${y2}`} stroke={c.muted} strokeWidth={1} opacity={0.6} />;
        })}
        {r.state !== "none" && (
          <AnimatedCircle cx={size / 2} cy={size / 2} r={rad} stroke={col} strokeWidth={stroke} fill="none" strokeLinecap="round" strokeDasharray={[circumference, circumference]} strokeDashoffset={offset as unknown as number} rotation={-90} originX={size / 2} originY={size / 2} />
        )}
      </Svg>
      <View style={{ alignItems: "center", gap: 2 }}>
        {r.state === "none" ? <Icon name="clock" size={size * 0.24} color={c.muted} /> : (
          <Text style={[s.mono, { color: c.fg, fontSize: big ? size * 0.16 : size * 0.24, lineHeight: big ? size * 0.2 : size * 0.3, fontFamily: font.monoBold }]}>{big ? countdown(r.remainingMs) : `${Math.round(r.used * 100)}%`}</Text>
        )}
        {big && <Text style={[s.overline, { color: col, fontSize: 10 }]}>{label}</Text>}
      </View>
    </View>
  );
}

/* ───────────────────────── loading, empty, error ───────────────────────── */

export function Skeleton({ h = 16, w = "100%", r = 6, style }: { h?: number; w?: number | `${number}%`; r?: number; style?: ViewStyle }) {
  const { c } = useTheme();
  const reduce = useReduceMotion();
  const v = useRef(new Animated.Value(0.45)).current;
  useEffect(() => {
    if (reduce) return;
    const loop = Animated.loop(Animated.sequence([
      Animated.timing(v, { toValue: 0.9, duration: 800, useNativeDriver: true }),
      Animated.timing(v, { toValue: 0.45, duration: 800, useNativeDriver: true }),
    ]));
    loop.start();
    return () => loop.stop();
  }, [reduce, v]);
  return <Animated.View accessibilityElementsHidden importantForAccessibility="no-hide-descendants" style={[{ height: h, width: w, borderRadius: r, backgroundColor: c.raised, opacity: v }, style]} />;
}

export function SkeletonRows({ n = 4, h = 78 }: { n?: number; h?: number }) {
  return (
    <View style={{ gap: sp.sm }} accessibilityLabel="Loading" accessibilityRole="progressbar">
      {Array.from({ length: n }).map((_, i) => <Skeleton key={i} h={h} r={radius.md} />)}
    </View>
  );
}

export function EmptyState({ icon = "shieldCheck", title, text, action }: { icon?: IconName; title: string; text?: string; action?: ReactNode }) {
  const { c, s } = useTheme();
  return (
    <View style={{ alignItems: "center", gap: sp.sm, paddingVertical: sp.xl, paddingHorizontal: sp.lg, borderRadius: radius.lg, borderWidth: 1, borderStyle: "dashed", borderColor: c.line }}>
      <View style={{ width: 56, height: 56, borderRadius: 28, borderWidth: 1, borderColor: c.line, backgroundColor: c.raised, alignItems: "center", justifyContent: "center" }}>
        <Icon name={icon} size={26} color={c.goldText} />
      </View>
      <Text style={[s.h2, { textAlign: "center" }]}>{title}</Text>
      {!!text && <Text style={[s.muted, { textAlign: "center" }]}>{text}</Text>}
      {action}
    </View>
  );
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  const { c, s } = useTheme();
  return (
    <Card tone="bad" style={{ backgroundColor: c.badBg }}>
      <View style={{ flexDirection: "row", alignItems: "center", gap: 8 }}>
        <Icon name="alert" size={20} color={c.bad} />
        <Text style={[s.semibold, { color: c.bad, fontSize: 15 }]}>Could not load</Text>
      </View>
      <Text style={[s.mono, { color: c.text2 }]} selectable>{message}</Text>
      <Text style={s.muted}>Your data on the server is unchanged. Pull down to try again.</Text>
      {onRetry && <View style={{ marginTop: 6 }}><Button small title="Retry" icon="sync" onPress={onRetry} /></View>}
    </Card>
  );
}

/** A banner for queued work or other state the user should notice. */
export function Notice({ icon = "info", tone = "warn", children }: { icon?: IconName; tone?: Tone; children: ReactNode }) {
  const { c, s } = useTheme();
  const col = toneColor(c, tone);
  return (
    <View accessibilityRole="alert" style={{ flexDirection: "row", gap: 10, alignItems: "flex-start", backgroundColor: toneBg(c, tone), borderColor: col, borderWidth: StyleSheet.hairlineWidth * 2, borderRadius: radius.md, padding: sp.md }}>
      <Icon name={icon} size={18} color={col} />
      <Text style={[s.muted, { flex: 1, color: c.text2 }]}>{children}</Text>
    </View>
  );
}

export function Emblem({ size = 120 }: { size?: number }) {
  return <Image source={require("../../assets/splash-icon.png")} accessible accessibilityLabel="HEKATE seal" style={{ width: size, height: size }} resizeMode="contain" />;
}
