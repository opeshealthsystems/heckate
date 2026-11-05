import { useCallback, useState } from "react";
import { FormPage } from "../../../components/Form";
import { HistorySection, STATUS_WORD, useHistory } from "../../../components/History";
import { Row, Section } from "../../../components/List";
import { DateField } from "../../../components/DateField";
import { Field } from "../../../components/TextField";
import { api } from "../../../lib/api";
import { haptic } from "../../../lib/haptics";
import { newKey } from "../../../lib/queue-core";
import { useSelfService } from "../../../lib/self-service";
import type { Timesheet } from "../../../lib/types";

const today = () => new Date().toISOString().slice(0, 10);
const day = (s: string) => new Date(s).toLocaleDateString("en-GB", { weekday: "short", day: "2-digit", month: "short" });

export default function LogHours() {
  const { out, submit } = useSelfService();
  const [f, setF] = useState({ date: today(), hours: "8", note: "" });
  const [sending, setSending] = useState("");
  const history = useHistory<Timesheet>(useCallback(() => api.timesheets(), []));
  const h = Number(f.hours.replace(",", "."));
  const valid = h > 0 && h <= 24 && f.date <= today();
  // A logged day stays a draft until it is sent for approval; drafts can be sent from the list below.
  const send = async (t: Timesheet) => {
    setSending(t.id);
    try { await api.submitTimesheet(t.id, newKey("ts")); haptic.success(); } catch { haptic.error(); }
    await history.reload();
    setSending("");
  };
  return (
    <FormPage
      title="Log hours"
      out={out}
      submitTitle="Save hours"
      disabled={!valid}
      onSubmit={() => void submit("Hours", "/v1/timesheets", { work_date: f.date, hours: h, note: f.note.trim() }, () => { setF({ ...f, note: "" }); void history.reload(); })}
      after={
        <HistorySection header="Your hours" {...history} empty="No hours logged yet." render={(t) => (
          <Row
            key={t.id}
            title={`${day(t.work_date)} · ${t.hours} h`}
            subtitle={`${STATUS_WORD[t.status] ?? t.status}${t.note ? ` · ${t.note}` : ""}`}
            value={t.status === "draft" ? (sending === t.id ? "Sending…" : "Send for approval") : undefined}
            onPress={t.status === "draft" && !sending ? () => void send(t) : undefined}
          />
        )} />
      }
    >
      <Section header="Hours" footer={!valid && f.hours ? (h > 24 ? "A day has at most 24 hours." : f.date > today() ? "You cannot log hours for a future day." : "Enter the hours worked.") : undefined} footerTone="red">
        <DateField label="Date" value={f.date} onChange={(v) => setF({ ...f, date: v })} />
        <Field label="Hours" placeholder="8" keyboardType="decimal-pad" returnKeyType="next" maxLength={5} value={f.hours} onChangeText={(v) => setF({ ...f, hours: v.replace(/[^\d.,]/g, "") })} />
      </Section>
      <Section header="Note">
        <Field placeholder="What you worked on" returnKeyType="done" value={f.note} onChangeText={(v) => setF({ ...f, note: v })} />
      </Section>
    </FormPage>
  );
}
