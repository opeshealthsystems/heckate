import { useState } from "react";
import { FormPage } from "../../../components/Form";
import { Section } from "../../../components/List";
import { DateField } from "../../../components/DateField";
import { Field } from "../../../components/TextField";
import { useSelfService } from "../../../lib/self-service";

const today = () => new Date().toISOString().slice(0, 10);

export default function RequestLeave() {
  const { out, submit } = useSelfService();
  const [f, setF] = useState({ kind: "annual", start: today(), end: today(), reason: "" });
  return (
    <FormPage
      title="Request leave"
      out={out}
      submitTitle="Request annual leave"
      disabled={!f.start || !f.end || f.end < f.start}
      onSubmit={() => void submit("Leave request", "/v1/leave", { kind: f.kind, start_date: f.start, end_date: f.end, reason: f.reason }, () => setF({ ...f, reason: "" }))}
    >
      <Section header="Dates">
        <DateField label="From" value={f.start} onChange={(v) => setF({ ...f, start: v, end: f.end < v ? v : f.end })} />
        <DateField label="To" value={f.end} onChange={(v) => setF({ ...f, end: v })} />
      </Section>
      <Section header="Reason (optional)">
        <Field placeholder="Reason" returnKeyType="done" value={f.reason} onChangeText={(v) => setF({ ...f, reason: v })} />
      </Section>
    </FormPage>
  );
}
