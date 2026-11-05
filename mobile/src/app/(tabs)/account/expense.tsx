import { useCallback, useState } from "react";
import { FormPage } from "../../../components/Form";
import { HistorySection, STATUS_WORD, useHistory } from "../../../components/History";
import { Row, Section } from "../../../components/List";
import { AddPhotoRow, PhotoUploadRow } from "../../../components/Photos";
import { DateField } from "../../../components/DateField";
import { Field } from "../../../components/TextField";
import { api, uploadPhoto } from "../../../lib/api";
import type { Photo } from "../../../lib/photos";
import { useSelfService } from "../../../lib/self-service";
import type { ExpenseClaim } from "../../../lib/types";

const today = () => new Date().toISOString().slice(0, 10);
const xaf = (n: number) => `${new Intl.NumberFormat("fr-FR").format(n)} XAF`;
const day = (s: string) => new Date(s).toLocaleDateString("en-GB", { day: "2-digit", month: "short" });

export default function ExpenseClaimScreen() {
  const { out, lastClaim, submitClaim } = useSelfService();
  const [f, setF] = useState({ date: today(), category: "", description: "", amount: "" });
  const [receipt, setReceipt] = useState<Photo | null>(null);
  const [photoMsg, setPhotoMsg] = useState("");
  const history = useHistory<ExpenseClaim>(useCallback(() => api.myExpenses(), []));
  return (
    <FormPage
      title="Expense claim"
      out={out}
      submitTitle="Submit claim"
      disabled={!f.category.trim() || !f.description.trim() || !(Number(f.amount) > 0)}
      onSubmit={() => void submitClaim(
        { expense_date: f.date, category: f.category.trim(), description: f.description.trim(), amount: Math.round(Number(f.amount)) },
        () => { setF({ ...f, category: "", description: "", amount: "" }); void history.reload(); },
        // The receipt chosen before sending goes up as soon as the claim exists.
        async (id) => { if (receipt) { try { await uploadPhoto("expense", id, receipt); } catch { setPhotoMsg("The claim was sent but the receipt did not upload. Add it again below."); } setReceipt(null); } },
      )}
      after={
        <HistorySection header="Your claims" {...history} empty="No expense claims yet." render={(c) => (
          <Row key={c.id} title={`${c.category} · ${xaf(c.amount)}`} subtitle={`${day(c.expense_date)} · ${STATUS_WORD[c.status] ?? c.status}`} />
        )} />
      }
    >
      <Section header="Claim">
        <DateField label="Date" value={f.date} onChange={(v) => setF({ ...f, date: v })} />
        <Field label="Category" placeholder="Travel" returnKeyType="next" value={f.category} onChangeText={(v) => setF({ ...f, category: v })} />
        <Field label="Description" placeholder="What it was for" returnKeyType="next" value={f.description} onChangeText={(v) => setF({ ...f, description: v })} />
        <Field label="Amount (XAF)" placeholder="0" keyboardType="number-pad" returnKeyType="done" value={f.amount ? new Intl.NumberFormat("fr-FR").format(Number(f.amount)) : ""} onChangeText={(v) => setF({ ...f, amount: v.replace(/\D/g, "") })} />
      </Section>
      <Section header="Receipt" footer={photoMsg || "Take a photo of the receipt. It is sent with the claim."} footerTone={photoMsg ? "red" : "label2"}>
        {lastClaim && photoMsg ? <PhotoUploadRow entityType="expense" entityId={lastClaim} onUploaded={() => setPhotoMsg("")} /> : (
          <AddPhotoRow subtitle={receipt ? "1 photo attached" : undefined} onError={setPhotoMsg} onPhoto={(p) => { setReceipt(p); setPhotoMsg(""); }} />
        )}
      </Section>
    </FormPage>
  );
}
