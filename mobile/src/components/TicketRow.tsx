import { router } from "expo-router";
import type { Ticket } from "../lib/types";
import { Row } from "./List";
import { SeverityText, SlaText } from "./Status";

/** A ticket as a list row: subject, "P1 Critical · Client", and the SLA as the trailing value (before the chevron). */
export function TicketRow({ ticket: t, now, client }: { ticket: Ticket; now: number; client?: string }) {
  return (
    <Row
      title={t.subject}
      titleLines={2}
      accessibilityLabel={`${t.severity} ${t.subject}`}
      subtitle={<SeverityText severity={t.severity} suffix={client} />}
      right={<SlaText ticket={t} now={now} short />}
      chevron
      onPress={() => router.push(`/ticket/${t.id}`)}
    />
  );
}
