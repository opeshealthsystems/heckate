import { useCallback, useState } from "react";
import { FormPage } from "../../../components/Form";
import { HistorySection, STATUS_WORD, useHistory } from "../../../components/History";
import { Row, Section } from "../../../components/List";
import { DateField } from "../../../components/DateField";
import { Segmented } from "../../../components/Segmented";
import { Field } from "../../../components/TextField";
import { api } from "../../../lib/api";
import { confirmDestructive } from "../../../lib/dialogs";
import { newKey } from "../../../lib/queue-core";
import { useSelfService } from "../../../lib/self-service";
import type { LeaveRequest } from "../../../lib/types";

const today = () => new Date().toISOString().slice(0, 10);
const KINDS = ["annual", "sick", "unpaid", "other"] as const;
type Kind = (typeof KINDS)[number];
const KIND_LABEL: Record<Kind, string> = { annual: "Annual", sick: "Sick", unpaid: "Unpaid", other: "Other" };
const day = (s: string) => new Date(s).toLocaleDateString("en-GB", { day: "2-digit", month: "short" });

export default function RequestLeave() {
  const { out, submit } = useSelfService();
  const [f, setF] = useState<{ kind: Kind; start: string; end: string; reason: string }>({ kind: "annual", start: today(), end: today(), reason: "" });
  const history = useHistory<LeaveRequest>(useCallback(() => api.myLeave(), []));
  const cancel = (l: LeaveRequest) =>
    confirmDestructive("Cancel this leave request?", `${KIND_LABEL[l.kind]} leave from ${day(l.start_date)}.`, "Cancel request", () =>
      void api.decide("leave", l.id, "cancel", newKey("leave")).then(history.reload).catch(() => history.reload()));
  return (
    <FormPage
      title="Request leave"
      out={out}
      submitTitle={`Request ${KIND_LABEL[f.kind].toLowerCase()} leave`}
      disabled={!f.start || !f.end || f.end < f.start || (f.kind === "other" && !f.reason.trim())}
      onSubmit={() => void submit("Leave request", "/v1/leave", { kind: f.kind, start_date: f.start, end_date: f.end, reason: f.reason }, () => { setF({ ...f, reason: "" }); void history.reload(); })}
      after={
        <HistorySection header="Your requests" {...history} empty="No leave requests yet." render={(l) => (
          <Row
            key={l.id}
            title={`${KIND_LABEL[l.kind] ?? l.kind} · ${l.days} day${l.days === 1 ? "" : "s"}`}
            subtitle={`${day(l.start_date)} to ${day(l.end_date)} · ${STATUS_WORD[l.status] ?? l.status}`}
            value={l.status === "pending" ? "Cancel" : undefined}
            onPress={l.status === "pending" ? () => cancel(l) : undefined}
          />
        )} />
      }
    >
      <Section header="Type">
        <Segmented options={KINDS} labels={KIND_LABEL} value={f.kind} onChange={(kind) => setF({ ...f, kind })} inset />
      </Section>
      <Section header="Dates">
        <DateField label="From" value={f.start} onChange={(v) => setF({ ...f, start: v, end: f.end < v ? v : f.end })} />
        <DateField label="To" value={f.end} onChange={(v) => setF({ ...f, end: v })} />
      </Section>
      <Section header={f.kind === "other" ? "Reason" : "Reason (optional)"} footer={f.kind === "other" ? "Say what the leave is for." : undefined}>
        <Field placeholder="Reason" returnKeyType="done" value={f.reason} onChangeText={(v) => setF({ ...f, reason: v })} />
      </Section>
    </FormPage>
  );
}
