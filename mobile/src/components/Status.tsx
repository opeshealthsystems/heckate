import { View } from "react-native";
import { slaLine } from "../lib/sla";
import { sp } from "../lib/theme";
import type { Severity, Status, Ticket } from "../lib/types";
import { Dot } from "./List";
import { Text } from "./Text";

export const SEVERITY_NAME: Record<Severity, string> = { P1: "Critical", P2: "High", P3: "Medium", P4: "Low" };
export const STATUS_NAME: Record<Status, string> = { open: "Open", in_progress: "In progress", resolved: "Resolved", closed: "Closed" };
export const visitName = (s: string) => (s === "in_progress" ? "On site" : s === "scheduled" ? "Scheduled" : s === "completed" ? "Completed" : s === "cancelled" ? "Cancelled" : s);

/**
 * Severity as text: "Critical" (semibold for P1 and P2) with one small coloured dot for P1 (red) and P2 (orange).
 * P3 and P4 are plain secondary text. Never a filled badge.
 */
export function SeverityText({ severity, suffix }: { severity: Severity; suffix?: string }) {
  const high = severity === "P1" || severity === "P2";
  return (
    <View style={{ flexDirection: "row", alignItems: "center", gap: 6 }}>
      {high && <Dot tone={severity === "P1" ? "red" : "orange"} />}
      <Text v="subhead" tone={high ? "label" : "label2"} weight={high ? "semibold" : "regular"} numberOfLines={1} style={{ flexShrink: 1 }}>
        {`${severity} ${SEVERITY_NAME[severity]}${suffix ? ` · ${suffix}` : ""}`}
      </Text>
    </View>
  );
}

/** "3 h 59 m left" in secondaryLabel, orange when at risk, red "Overdue by 1 h 12 m" when breached. `now` drives the 30 s refresh. */
export function SlaText({ ticket, now, v = "subhead" }: { ticket: Pick<Ticket, "created_at" | "sla_due_at" | "resolved_at" | "sla_breached">; now: number; v?: "subhead" | "body" }) {
  const l = slaLine(ticket, now);
  if (!l) return null;
  return <Text v={v} tone={l.tone} weight={l.tone === "label2" ? "regular" : "semibold"} numberOfLines={1}>{l.text}</Text>;
}

export const rowGap = sp.xs;
