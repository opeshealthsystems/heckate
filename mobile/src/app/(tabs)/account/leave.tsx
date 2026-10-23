import { useState } from "react";
import { FormPage } from "../../../components/Form";
import { Section } from "../../../components/List";
import { Field } from "../../../components/TextField";
import { useSelfService } from "../../../lib/self-service";

export default function RequestLeave() {
  const { out, submit } = useSelfService();
  const [f, setF] = useState({ kind: "annual", start: "", end: "", reason: "" });
  return (
    <FormPage
      title="Request leave"
      out={out}
      submitTitle="Request annual leave"
      disabled={!f.start || !f.end}
      onSubmit={() => void submit("Leave request", "/v1/leave", { kind: f.kind, start_date: f.start, end_date: f.end, reason: f.reason }, () => setF({ ...f, start: "", end: "", reason: "" }))}
    >
      <Section header="Dates" footer="Dates are written year-month-day.">
        <Field label="From" placeholder="YYYY-MM-DD" keyboardType="numbers-and-punctuation" autoCorrect={false} returnKeyType="next" value={f.start} onChangeText={(v) => setF({ ...f, start: v })} />
        <Field label="To" placeholder="YYYY-MM-DD" keyboardType="numbers-and-punctuation" autoCorrect={false} returnKeyType="next" value={f.end} onChangeText={(v) => setF({ ...f, end: v })} />
      </Section>
      <Section header="Reason (optional)">
        <Field placeholder="Reason" returnKeyType="done" value={f.reason} onChangeText={(v) => setF({ ...f, reason: v })} />
      </Section>
    </FormPage>
  );
}
