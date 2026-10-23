import { View } from "react-native";
import { slaLine } from "../lib/sla";
import type { Severity, Status, Ticket } from "../lib/types";
import { Dot } from "./List";
import { Text } from "./Text";

export const SEVERITY_NAME: Record<Severity, string> = { P1: "Critical", P2: "High", P3: "Medium", P4: "Low" };
export const STATUS_NAME: Record<Status, string> = { open: "Open", in_progress: "In progress", resolved: "Resolved", closed: "Closed" };
export const visitName = (s: string) => (s === "in_progress" ? "On site" : s === "scheduled" ? "Scheduled" : s === "completed" ? "Completed" : s === "cancelled" ? "Cancelled" : s);

/**
 * Severity as text. P1 and P2 are set in primary text, P3 and P4 in secondary text; P1 alone gets one small
 * red dot (orange is reserved for "at risk"). Never a filled badge. `suffix` (status, client) follows in secondary text.
 */
export function SeverityText({ severity, suffix }: { severity: Severity; suffix?: string }) {
  const high = severity === "P1" || severity === "P2";
  return (
    <View style={{ flexDirection: "row", alignItems: "center", gap: 6 }}>
      {severity === "P1" && <Dot tone="red" />}
      <Text v="subhead" tone="label2" numberOfLines={1} style={{ flexShrink: 1 }}>
        <Text v="subhead" tone={high ? "label" : "label2"}>{`${severity} ${SEVERITY_NAME[severity]}`}</Text>
        {suffix ? ` · ${suffix}` : ""}
      </Text>
    </View>
  );
}

/** "3 h 59 m left" in secondaryLabel, orange when at risk, red "Overdue by 1 h 12 m" when breached. `now` drives the 30 s refresh. */
export function SlaText({ ticket, now, v = "subhead", short }: { ticket: Pick<Ticket, "created_at" | "sla_due_at" | "resolved_at" | "sla_breached">; now: number; v?: "subhead" | "body"; short?: boolean }) {
  const l = slaLine(ticket, now, short);
  if (!l) return null;
  return <Text v={v} tone={l.tone} weight={l.tone === "red" ? "semibold" : "regular"} numberOfLines={1} style={{ textAlign: "right" }}>{l.text}</Text>;
}
