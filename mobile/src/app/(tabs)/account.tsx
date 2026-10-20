import { useCallback, useState } from "react";
import { ScrollView, Text, TextInput, View } from "react-native";
import { useFocusEffect } from "expo-router";
import { Button } from "../../components/ui";
import { PhotoButtons } from "../../components/PhotoButtons";
import { api, ApiError, replay } from "../../lib/api";
import { classify } from "../../lib/drafts-core";
import { enqueue } from "../../lib/queue";
import { newKey } from "../../lib/queue-core";
import { useSession } from "../../lib/session";
import { useTheme } from "../../lib/theme";
import type { EmployeeRecord } from "../../lib/types";

const today = () => new Date().toISOString().slice(0, 10);

export default function Account() {
  const { s, c } = useTheme();
  const { me, signOut, refreshQueue, pendingActions } = useSession();
  const staff = me?.scope === "staff";
  const [rec, setRec] = useState<EmployeeRecord | null>(null);
  const [recMsg, setRecMsg] = useState("");
  const [msg, setMsg] = useState("");
  const [lastClaim, setLastClaim] = useState<string | null>(null);
  const [hours, setHours] = useState({ date: today(), hours: "8", note: "" });
  const [leave, setLeave] = useState({ kind: "annual", start: "", end: "", reason: "" });
  const [claim, setClaim] = useState({ date: today(), category: "", description: "", amount: "" });

  useFocusEffect(useCallback(() => {
    if (!staff) return;
    api.myRecord().then((r) => { setRec(r); setRecMsg(""); }).catch((e) => setRecMsg((e as Error).message));
  }, [staff]));

  /** Sends now or keeps it on the phone until there is a connection. */
  async function submit(label: string, path: string, body: Record<string, unknown>, done: () => void) {
    setMsg("");
    const action = { key: newKey("self"), method: "POST" as const, path, body, createdAt: Date.now(), label };
    try {
      const r = await replay(action);
      if (r === "retry") {
        await enqueue(action);
        await refreshQueue();
        setMsg(`${label} saved on this phone and will be sent when you are online.`);
        done();
      } else if (r === "drop") {
        setMsg(`${label} was refused by the server. Check the dates and amounts.`);
      } else {
        setMsg(`${label} sent.`);
        done();
        api.myRecord().then(setRec).catch(() => {});
      }
    } catch (e) {
      setMsg(e instanceof ApiError ? e.message : (e as Error).message);
    }
  }

  /** Expense claims keep the new claim's id when sent online, so a receipt photo can be attached to it. */
  async function submitClaim() {
    setMsg("");
    const body = { expense_date: claim.date, category: claim.category, description: claim.description, amount: Math.round(Number(claim.amount)) };
    const key = newKey("self");
    try {
      const c = await api.claimExpense(body, key);
      setLastClaim(c.id);
      setMsg("Expense claim sent. You can attach a photo of the receipt below.");
      setClaim({ ...claim, category: "", description: "", amount: "" });
    } catch (e) {
      if (classify(e instanceof ApiError ? e.status : null) === "retry") {
        await enqueue({ key, method: "POST", path: "/v1/expenses", body, createdAt: Date.now(), label: "Expense claim" });
        await refreshQueue();
        setLastClaim(null);
        setMsg("Expense claim saved on this phone and will be sent when you are online. Add the receipt photo from here once it is sent.");
        setClaim({ ...claim, category: "", description: "", amount: "" });
      } else {
        setMsg(e instanceof ApiError ? e.message : (e as Error).message);
      }
    }
  }

  return (
    <ScrollView style={s.screen} contentContainerStyle={s.pad} keyboardShouldPersistTaps="handled">
      <Text style={s.h1}>Account</Text>
      <View style={s.card}>
        <Text style={s.text}>{me?.email || me?.sub}</Text>
        <Text style={s.muted}>{me?.roles.join(", ")}</Text>
        {pendingActions > 0 && <Text style={s.muted}>{pendingActions} update(s) waiting to be sent.</Text>}
      </View>
      {!!msg && <Text style={/sent|saved/i.test(msg) ? s.muted : s.error}>{msg}</Text>}

      {staff && (
        <>
          <View style={s.card}>
            {rec ? (
              <Text style={s.text}>Annual leave: {rec.annual_leave_balance} of {rec.annual_leave_entitlement} days left</Text>
            ) : (
              <Text style={s.muted}>{recMsg || "Loading your leave balance…"}</Text>
            )}
          </View>

          <Text style={s.label}>Log hours</Text>
          <TextInput style={s.input} value={hours.date} onChangeText={(v) => setHours({ ...hours, date: v })} placeholder="YYYY-MM-DD" placeholderTextColor={c.muted} />
          <TextInput style={s.input} value={hours.hours} onChangeText={(v) => setHours({ ...hours, hours: v })} keyboardType="decimal-pad" placeholder="Hours" placeholderTextColor={c.muted} />
          <TextInput style={s.input} value={hours.note} onChangeText={(v) => setHours({ ...hours, note: v })} placeholder="What you worked on" placeholderTextColor={c.muted} />
          <Button title="Save hours" disabled={!(Number(hours.hours) > 0)} onPress={() => void submit("Hours", "/v1/timesheets", { work_date: hours.date, hours: Number(hours.hours), note: hours.note }, () => setHours({ ...hours, note: "" }))} />

          <Text style={s.label}>Request leave</Text>
          <TextInput style={s.input} value={leave.start} onChangeText={(v) => setLeave({ ...leave, start: v })} placeholder="From (YYYY-MM-DD)" placeholderTextColor={c.muted} />
          <TextInput style={s.input} value={leave.end} onChangeText={(v) => setLeave({ ...leave, end: v })} placeholder="To (YYYY-MM-DD)" placeholderTextColor={c.muted} />
          <TextInput style={s.input} value={leave.reason} onChangeText={(v) => setLeave({ ...leave, reason: v })} placeholder="Reason (optional)" placeholderTextColor={c.muted} />
          <Button title="Request annual leave" disabled={!leave.start || !leave.end} onPress={() => void submit("Leave request", "/v1/leave", { kind: leave.kind, start_date: leave.start, end_date: leave.end, reason: leave.reason }, () => setLeave({ ...leave, start: "", end: "", reason: "" }))} />

          <Text style={s.label}>Expense claim</Text>
          <TextInput style={s.input} value={claim.category} onChangeText={(v) => setClaim({ ...claim, category: v })} placeholder="Category (e.g. travel)" placeholderTextColor={c.muted} />
          <TextInput style={s.input} value={claim.description} onChangeText={(v) => setClaim({ ...claim, description: v })} placeholder="Description" placeholderTextColor={c.muted} />
          <TextInput style={s.input} value={claim.amount} onChangeText={(v) => setClaim({ ...claim, amount: v })} keyboardType="number-pad" placeholder="Amount (XAF)" placeholderTextColor={c.muted} />
          <Button title="Submit claim" disabled={!claim.category || !claim.description || !(Number(claim.amount) > 0)} onPress={() => void submitClaim()} />
          {lastClaim && <PhotoButtons entityType="expense" entityId={lastClaim} />}
        </>
      )}
      <Button title="Sign out" onPress={() => void signOut()} />
    </ScrollView>
  );
}
