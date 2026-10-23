import { useState } from "react";
import { FormPage } from "../../../components/Form";
import { Section } from "../../../components/List";
import { PhotoUploadRow } from "../../../components/Photos";
import { Field } from "../../../components/TextField";
import { useSelfService } from "../../../lib/self-service";

const today = () => new Date().toISOString().slice(0, 10);

export default function ExpenseClaim() {
  const { out, lastClaim, submitClaim } = useSelfService();
  const [f, setF] = useState({ date: today(), category: "", description: "", amount: "" });
  return (
    <FormPage
      title="Expense claim"
      out={out}
      submitTitle="Submit claim"
      disabled={!f.category || !f.description || !(Number(f.amount) > 0)}
      onSubmit={() => void submitClaim({ expense_date: f.date, category: f.category, description: f.description, amount: Math.round(Number(f.amount)) }, () => setF({ ...f, category: "", description: "", amount: "" }))}
    >
      <Section header="Claim">
        <Field label="Date" placeholder="YYYY-MM-DD" keyboardType="numbers-and-punctuation" autoCorrect={false} returnKeyType="next" value={f.date} onChangeText={(v) => setF({ ...f, date: v })} />
        <Field label="Category" placeholder="Travel" returnKeyType="next" value={f.category} onChangeText={(v) => setF({ ...f, category: v })} />
        <Field label="Description" placeholder="What it was for" returnKeyType="next" value={f.description} onChangeText={(v) => setF({ ...f, description: v })} />
        <Field label="Amount (XAF)" placeholder="0" keyboardType="number-pad" returnKeyType="done" value={f.amount} onChangeText={(v) => setF({ ...f, amount: v })} />
      </Section>
      {lastClaim && (
        <Section header="Receipt">
          <PhotoUploadRow entityType="expense" entityId={lastClaim} />
        </Section>
      )}
    </FormPage>
  );
}
