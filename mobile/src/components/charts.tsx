import { useEffect, useRef } from "react";
import { Animated, Easing, StyleSheet, Text, View } from "react-native";
import Svg, { Circle, Defs, Path, Pattern, Rect } from "react-native-svg";
import { arcDash } from "../lib/sla";
import { font, radius, sp, useReduceMotion, useTheme } from "../lib/theme";
import type { Severity } from "../lib/types";
import { sevColor, SEV_NAME, type Tone, toneColor } from "./ui";

const AnimatedCircle = Animated.createAnimatedComponent(Circle);

/** A ring whose arc eases to `fraction` (0..1). The centre is free for text. */
export function Ring({ fraction, color, size = 120, thickness, children }: { fraction: number; color: string; size?: number; thickness?: number; children?: React.ReactNode }) {
  const { c } = useTheme();
  const reduce = useReduceMotion();
  const stroke = thickness ?? Math.max(6, size * 0.085);
  const r = (size - stroke) / 2;
  const { circumference } = arcDash(fraction, r);
  const v = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    const to = Math.min(1, Math.max(0, fraction));
    if (reduce) { v.setValue(to); return; }
    Animated.timing(v, { toValue: to, duration: 900, easing: Easing.out(Easing.cubic), useNativeDriver: false }).start();
  }, [fraction, reduce, v]);
  const offset = v.interpolate({ inputRange: [0, 1], outputRange: [circumference, 0] });
  const mid = size / 2;
  return (
    <View style={{ width: size, height: size, alignItems: "center", justifyContent: "center" }}>
      <Svg width={size} height={size} style={StyleSheet.absoluteFill}>
        {/* faint concentric guides, like a radar scope */}
        <Circle cx={mid} cy={mid} r={r - stroke - 6} stroke={c.line} strokeWidth={0.8} fill="none" opacity={0.6} />
        <Circle cx={mid} cy={mid} r={r - stroke - 18} stroke={c.line} strokeWidth={0.8} fill="none" opacity={0.35} strokeDasharray={[2, 4]} />
        <Circle cx={mid} cy={mid} r={r} stroke={c.line} strokeWidth={stroke} fill="none" opacity={0.7} />
        <AnimatedCircle cx={mid} cy={mid} r={r} stroke={color} strokeWidth={stroke} fill="none" strokeLinecap="round" strokeDasharray={[circumference, circumference]} strokeDashoffset={offset as unknown as number} rotation={-90} originX={mid} originY={mid} />
      </Svg>
      {children}
    </View>
  );
}

/** Stacked bar of open tickets by severity, with a legend that repeats the numbers (colour is never the only cue). */
export function SeverityBar({ counts }: { counts: Record<Severity, number> }) {
  const { c, s } = useTheme();
  const total = (Object.values(counts) as number[]).reduce((a, b) => a + b, 0);
  const order: Severity[] = ["P1", "P2", "P3", "P4"];
  return (
    <View style={{ gap: 8 }} accessible accessibilityLabel={`Open by severity: ${order.map((k) => `${counts[k]} ${k}`).join(", ")}`}>
      <View style={{ flexDirection: "row", height: 8, borderRadius: 4, overflow: "hidden", backgroundColor: c.line, gap: 2 }}>
        {total > 0 && order.filter((k) => counts[k] > 0).map((k) => <View key={k} style={{ flex: counts[k], backgroundColor: sevColor(c, k) }} />)}
      </View>
      <View style={{ flexDirection: "row", justifyContent: "space-between" }}>
        {order.map((k) => (
          <View key={k} style={{ flexDirection: "row", alignItems: "center", gap: 5 }}>
            <View style={{ width: 8, height: 8, borderRadius: 2, backgroundColor: sevColor(c, k) }} />
            <Text style={[s.mono, { color: c.text2, fontSize: 11.5 }]}>{k} {counts[k]}</Text>
            <Text style={[s.muted, { fontSize: 11 }]}>{SEV_NAME[k]}</Text>
          </View>
        ))}
      </View>
    </View>
  );
}

export interface Stat { label: string; value: string | number; tone?: Tone }

/** Posture header of Home: scope-style ring with the headline score, three stats, and the severity bar. */
export function PostureCard({ title, score, scoreLabel, caption, stats, tone, counts, fraction }: { title: string; score: string; scoreLabel: string; caption: string; stats: Stat[]; tone: Tone; counts?: Record<Severity, number>; fraction: number }) {
  const { c, s } = useTheme();
  const col = toneColor(c, tone);
  return (
    <View style={{ backgroundColor: c.surface, borderColor: c.line, borderWidth: StyleSheet.hairlineWidth * 2, borderRadius: radius.lg, overflow: "hidden" }}>
      <Svg style={StyleSheet.absoluteFill} width="100%" height="100%" pointerEvents="none">
        <Defs>
          <Pattern id="dots" patternUnits="userSpaceOnUse" width={14} height={14}>
            <Circle cx={1} cy={1} r={0.8} fill={c.line} opacity={0.7} />
          </Pattern>
        </Defs>
        <Rect width="100%" height="100%" fill="url(#dots)" opacity={0.55} />
        <Path d="M0 0H18M0 0V18" stroke={c.gold} strokeWidth={2} />
      </Svg>
      <View style={{ padding: sp.lg, gap: sp.md }}>
        <View style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}>
          <Text style={[s.overline, { color: c.goldText }]}>{title}</Text>
          <View style={{ flexDirection: "row", alignItems: "center", gap: 5 }}>
            <View style={{ width: 7, height: 7, borderRadius: 4, backgroundColor: col }} />
            <Text style={[s.overline, { color: col }]}>{tone === "ok" ? "Nominal" : tone === "warn" ? "Attention" : tone === "bad" ? "At risk" : "Idle"}</Text>
          </View>
        </View>
        <View style={{ flexDirection: "row", alignItems: "center", gap: sp.lg }}>
          <Ring fraction={fraction} color={col} size={132}>
            <Text style={{ color: c.fg, fontSize: score.length > 3 ? 24 : 32, lineHeight: 38, fontFamily: font.monoBold }} accessibilityLabel={`${scoreLabel} ${score}`}>{score}</Text>
            <Text style={[s.overline, { fontSize: 9.5 }]}>{scoreLabel}</Text>
          </Ring>
          <View style={{ flex: 1, gap: sp.sm }}>
            {stats.map((st) => (
              <View key={st.label} style={{ flexDirection: "row", alignItems: "baseline", justifyContent: "space-between", borderBottomColor: c.lineSoft, borderBottomWidth: StyleSheet.hairlineWidth * 2, paddingBottom: 5 }}>
                <Text style={[s.muted, { fontSize: 13, flexShrink: 1 }]} numberOfLines={1}>{st.label}</Text>
                <Text numberOfLines={1} adjustsFontSizeToFit style={{ color: st.tone ? toneColor(c, st.tone) : c.fg, fontSize: typeof st.value === "string" ? 15 : 19, fontFamily: font.monoBold, flexShrink: 0 }}>{st.value}</Text>
              </View>
            ))}
          </View>
        </View>
        {counts && <SeverityBar counts={counts} />}
        <Text style={[s.muted, { fontSize: 12 }]}>{caption}</Text>
      </View>
    </View>
  );
}
