import { View } from "react-native";
import { router } from "expo-router";
import type { Ticket } from "../lib/types";
import { Row } from "./List";
import { SeverityText, SlaText, STATUS_NAME } from "./Status";

/** A ticket as a list row: subject, "P1 Critical · Open · Client", and the single SLA line. Three lines at most. */
export function TicketRow({ ticket: t, now, client }: { ticket: Ticket; now: number; client?: string }) {
  return (
    <Row
      title={t.subject}
      accessibilityLabel={`${t.severity} ${t.subject}`}
      subtitle={
        <View style={{ gap: 2 }}>
          <SeverityText severity={t.severity} suffix={`${STATUS_NAME[t.status]}${client ? ` · ${client}` : ""}`} />
          <SlaText ticket={t} now={now} />
        </View>
      }
      onPress={() => router.push(`/ticket/${t.id}`)}
    />
  );
}
