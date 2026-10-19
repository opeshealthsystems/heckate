import { Pressable, Text, View } from "react-native";
import { slaRemaining } from "../lib/sla";
import { useTheme } from "../lib/theme";
import type { Severity, Ticket } from "../lib/types";

export function Button({ title, onPress, primary, disabled }: { title: string; onPress: () => void; primary?: boolean; disabled?: boolean }) {
  const { s } = useTheme();
  return (
    <Pressable accessibilityRole="button" disabled={disabled} onPress={onPress} style={[s.btn, primary && s.btnPrimary, disabled && { opacity: 0.5 }]}>
      <Text style={primary ? s.btnPrimaryText : s.btnText}>{title}</Text>
    </Pressable>
  );
}

export function Pill({ text, tone }: { text: string; tone?: "ok" | "bad" | "warn" }) {
  const { c } = useTheme();
  const color = tone ? c[tone] : c.muted;
  return (
    <View style={{ borderColor: color, borderWidth: 1, borderRadius: 999, paddingHorizontal: 8, paddingVertical: 2 }}>
      <Text style={{ color, fontSize: 12, fontWeight: "600" }}>{text}</Text>
    </View>
  );
}

const sevTone: Record<Severity, "bad" | "warn" | undefined> = { P1: "bad", P2: "warn", P3: undefined, P4: undefined };

export function TicketPills({ t }: { t: Ticket }) {
  const { s } = useTheme();
  const done = !!t.resolved_at;
  const sla = slaRemaining(t.sla_due_at);
  return (
    <View style={s.row}>
      <Pill text={t.severity} tone={sevTone[t.severity]} />
      <Pill text={t.status.replace("_", " ")} />
      {done ? <Pill text={t.sla_breached ? "SLA missed" : "SLA met"} tone={t.sla_breached ? "bad" : "ok"} /> : sla && <Pill text={sla.label} tone={sla.overdue ? "bad" : "ok"} />}
    </View>
  );
}
