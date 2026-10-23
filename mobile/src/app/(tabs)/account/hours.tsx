import { useState } from "react";
import { FormPage } from "../../../components/Form";
import { Section } from "../../../components/List";
import { DateField } from "../../../components/DateField";
import { Field } from "../../../components/TextField";
import { useSelfService } from "../../../lib/self-service";

const today = () => new Date().toISOString().slice(0, 10);

export default function LogHours() {
  const { out, submit } = useSelfService();
  const [f, setF] = useState({ date: today(), hours: "8", note: "" });
  return (
    <FormPage
      title="Log hours"
      out={out}
      submitTitle="Save hours"
      disabled={!(Number(f.hours) > 0)}
      onSubmit={() => void submit("Hours", "/v1/timesheets", { work_date: f.date, hours: Number(f.hours), note: f.note }, () => setF({ ...f, note: "" }))}
    >
      <Section header="Hours">
        <DateField label="Date" value={f.date} onChange={(v) => setF({ ...f, date: v })} />
        <Field label="Hours" placeholder="8" keyboardType="decimal-pad" returnKeyType="next" value={f.hours} onChangeText={(v) => setF({ ...f, hours: v })} />
      </Section>
      <Section header="Note">
        <Field placeholder="What you worked on" returnKeyType="done" value={f.note} onChangeText={(v) => setF({ ...f, note: v })} />
      </Section>
    </FormPage>
  );
}
