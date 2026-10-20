import type { ReactNode } from "react";
import { Pressable, Text, View } from "react-native";
import { font, useTheme } from "../lib/theme";

/** KPI tile as in the approved dashboard design: small caps label, large navy value, one line of context. */
export function Kpi({ label, value, sub, tone }: { label: string; value: string | number; sub?: string; tone?: "ok" | "bad" }) {
  const { c } = useTheme();
  const color = tone ? c[tone] : c.brand;
  return (
    <View accessible style={{ flexBasis: "47%", flexGrow: 1, backgroundColor: c.card, borderColor: tone === "bad" ? c.bad : c.line, borderWidth: 1, borderTopColor: tone === "bad" ? c.bad : c.gold, borderTopWidth: 3, borderRadius: 6, padding: 12, gap: 2 }}>
      <Text style={{ color: c.muted, fontSize: 12, fontFamily: font.semibold, textTransform: "uppercase", letterSpacing: 0.6 }}>{label}</Text>
      <Text style={{ color, fontSize: 26, fontFamily: font.display }}>{value}</Text>
      {!!sub && <Text style={{ color: tone ? c[tone] : c.muted, fontSize: 13, fontFamily: font.body }}>{sub}</Text>}
    </View>
  );
}

export function Section({ title, children }: { title: string; children: ReactNode }) {
  const { c, s } = useTheme();
  return (
    <View style={{ gap: 8 }}>
      <Text style={{ color: c.brand, fontSize: 18, fontFamily: font.display }} accessibilityRole="header">{title}</Text>
      <View style={[s.card, { padding: 0, gap: 0 }]}>{children}</View>
    </View>
  );
}

export function RowItem({ title, sub, right, onPress, last }: { title: string; sub?: string; right?: ReactNode; onPress?: () => void; last?: boolean }) {
  const { c, s } = useTheme();
  const body = (
    <View style={{ flexDirection: "row", alignItems: "center", gap: 8, padding: 14, borderBottomColor: c.line, borderBottomWidth: last ? 0 : 1 }}>
      <View style={{ flex: 1, gap: 2 }}>
        <Text style={[s.text, s.semibold]}>{title}</Text>
        {!!sub && <Text style={s.muted}>{sub}</Text>}
      </View>
      {right}
    </View>
  );
  if (!onPress) return body;
  return <Pressable accessibilityRole="button" onPress={onPress}>{body}</Pressable>;
}

export function Empty({ text }: { text: string }) {
  const { s } = useTheme();
  return <View style={{ padding: 14 }}><Text style={s.muted}>{text}</Text></View>;
}
