import type { ReactNode } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { haptic } from "../lib/haptics";
import { font, radius, sp, useTheme } from "../lib/theme";
import { Icon, type IconName } from "./Icon";
import { Card } from "./ui";

/** Metric tile: overline label, large mono value, one line of context. A bad tone adds a red rail. */
export function Kpi({ label, value, sub, tone, icon }: { label: string; value: string | number; sub?: string; tone?: "ok" | "bad"; icon?: IconName }) {
  const { c, s } = useTheme();
  const color = tone ? c[tone] : c.fg;
  return (
    <View accessible style={{ flexBasis: "47%", flexGrow: 1, backgroundColor: c.surface, borderColor: tone === "bad" ? c.bad : c.line, borderWidth: StyleSheet.hairlineWidth * 2, borderRadius: radius.md, padding: sp.md, paddingLeft: sp.lg, gap: 4, overflow: "hidden" }}>
      <View style={{ position: "absolute", left: 0, top: 0, bottom: 0, width: 3, backgroundColor: tone === "bad" ? c.bad : tone === "ok" ? c.ok : c.gold }} />
      <View style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}>
        <Text style={s.overline}>{label}</Text>
        {icon && <Icon name={icon} size={14} color={c.muted} />}
      </View>
      <Text style={{ color, fontSize: 26, lineHeight: 32, fontFamily: font.monoBold }} numberOfLines={1} adjustsFontSizeToFit>{value}</Text>
      {!!sub && <Text style={{ color: tone ? c[tone] : c.muted, fontSize: 12.5, fontFamily: font.body }}>{sub}</Text>}
    </View>
  );
}

/** Section with a small-caps title and hairline, then its rows in one card. */
export function Section({ title, children, count }: { title: string; children: ReactNode; count?: number }) {
  const { c, s } = useTheme();
  return (
    <View style={{ gap: sp.sm }}>
      <View style={{ flexDirection: "row", alignItems: "center", gap: sp.sm }}>
        <Text style={[s.overline, { color: c.goldText }]} accessibilityRole="header">{title}</Text>
        <View style={{ flex: 1, height: StyleSheet.hairlineWidth * 2, backgroundColor: c.line }} />
        {count !== undefined && <Text style={[s.mono, { color: c.muted }]}>{String(count).padStart(2, "0")}</Text>}
      </View>
      <Card style={{ padding: 0, gap: 0 }}>{children}</Card>
    </View>
  );
}

export function RowItem({ title, sub, right, onPress, last, rail, mono }: { title: string; sub?: ReactNode; right?: ReactNode; onPress?: () => void; last?: boolean; rail?: string; mono?: string }) {
  const { c, s } = useTheme();
  const body = (pressed: boolean) => (
    <View style={{ flexDirection: "row", alignItems: "center", gap: sp.sm, padding: sp.md, paddingLeft: sp.lg + (rail ? 3 : 0), minHeight: 56, backgroundColor: pressed ? c.raised : "transparent", borderBottomColor: c.lineSoft, borderBottomWidth: last ? 0 : StyleSheet.hairlineWidth * 2 }}>
      {rail && <View style={{ position: "absolute", left: 0, top: 0, bottom: 0, width: 3, backgroundColor: rail }} />}
      <View style={{ flex: 1, gap: 3 }}>
        {!!mono && <Text style={[s.mono, { color: c.goldText, fontSize: 11 }]}>{mono}</Text>}
        <Text style={[s.text, s.semibold, { fontSize: 15, lineHeight: 20 }]}>{title}</Text>
        {typeof sub === "string" ? (!!sub && <Text style={[s.muted, { fontSize: 13 }]}>{sub}</Text>) : sub}
      </View>
      {right}
      {onPress && <Icon name="chevron" size={16} color={c.muted} />}
    </View>
  );
  if (!onPress) return body(false);
  return <Pressable accessibilityRole="button" onPress={() => { haptic.tap(); onPress(); }}>{({ pressed }) => body(pressed)}</Pressable>;
}

export function Empty({ text }: { text: string }) {
  const { s } = useTheme();
  return <View style={{ padding: sp.lg }}><Text style={s.muted}>{text}</Text></View>;
}
