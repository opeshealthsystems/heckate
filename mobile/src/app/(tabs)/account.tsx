import { useCallback, useState, type ReactNode } from "react";
import { Pressable, ScrollView, StyleSheet, Switch, Text, View } from "react-native";
import { useFocusEffect } from "expo-router";
import { Icon, type IconName } from "../../components/Icon";
import { PhotoButtons } from "../../components/PhotoButtons";
import { Button, Card, Input, KV, Notice, Pill, Screen, ScreenHeader, Segmented } from "../../components/ui";
import { api, ApiError, replay } from "../../lib/api";
import { classify } from "../../lib/drafts-core";
import { haptic } from "../../lib/haptics";
import { useLock } from "../../lib/lock";
import { enqueue } from "../../lib/queue";
import { newKey } from "../../lib/queue-core";
import { useSession } from "../../lib/session";
import { font, radius, sp, useTheme } from "../../lib/theme";
import type { EmployeeRecord } from "../../lib/types";

const today = () => new Date().toISOString().slice(0, 10);
const roleLabel = (r: string) => (r === "ceo" ? "CEO" : r.charAt(0).toUpperCase() + r.slice(1).replace(/_/g, " "));

function Group({ title, children }: { title: string; children: ReactNode }) {
  const { c, s } = useTheme();
  return (
    <View style={{ gap: sp.sm }}>
      <View style={{ flexDirection: "row", alignItems: "center", gap: sp.sm }}>
        <Text style={[s.overline, { color: c.goldText }]} accessibilityRole="header">{title}</Text>
        <View style={{ flex: 1, height: StyleSheet.hairlineWidth * 2, backgroundColor: c.line }} />
      </View>
      {children}
    </View>
  );
}

function Accordion({ icon, title, open, onToggle, children }: { icon: IconName; title: string; open: boolean; onToggle: () => void; children: ReactNode }) {
  const { c } = useTheme();
  return (
    <View style={{ backgroundColor: c.surface, borderColor: open ? c.gold : c.line, borderWidth: StyleSheet.hairlineWidth * 2, borderRadius: radius.md, overflow: "hidden" }}>
      <Pressable accessibilityRole="button" accessibilityState={{ expanded: open }} onPress={() => { haptic.select(); onToggle(); }} style={{ minHeight: 56, flexDirection: "row", alignItems: "center", gap: sp.md, paddingHorizontal: sp.lg }}>
        <Icon name={icon} size={20} color={c.goldText} />
        <Text style={{ flex: 1, color: c.fg, fontSize: 16, fontFamily: font.semibold }}>{title}</Text>
        <View style={{ transform: [{ rotate: open ? "90deg" : "0deg" }] }}><Icon name="chevron" size={18} color={c.muted} /></View>
      </Pressable>
      {open && <View style={{ padding: sp.lg, paddingTop: sp.xs, gap: sp.sm, borderTopColor: c.lineSoft, borderTopWidth: 1 }}>{children}</View>}
    </View>
  );
}

export default function Account() {
  const { s, c, mode, setMode } = useTheme();
  const { me, signOut, refreshQueue, pendingActions } = useSession();
  const lock = useLock();
  const staff = me?.scope === "staff";
  const [rec, setRec] = useState<EmployeeRecord | null>(null);
  const [recMsg, setRecMsg] = useState("");
  const [msg, setMsg] = useState("");
  const [lockMsg, setLockMsg] = useState("");
  const [open, setOpen] = useState<"hours" | "leave" | "claim" | null>(null);
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
        haptic.warning();
        setMsg(`${label} saved on this phone and will be sent when you are online.`);
        done();
      } else if (r === "drop") {
        haptic.error();
        setMsg(`${label} was refused by the server. Check the dates and amounts.`);
      } else {
        haptic.success();
        setMsg(`${label} sent.`);
        done();
        api.myRecord().then(setRec).catch(() => {});
      }
    } catch (e) {
      haptic.error();
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
      haptic.success();
      setLastClaim(c.id);
      setMsg("Expense claim sent. You can attach a photo of the receipt below.");
      setClaim({ ...claim, category: "", description: "", amount: "" });
    } catch (e) {
      if (classify(e instanceof ApiError ? e.status : null) === "retry") {
        await enqueue({ key, method: "POST", path: "/v1/expenses", body, createdAt: Date.now(), label: "Expense claim" });
        await refreshQueue();
        haptic.warning();
        setLastClaim(null);
        setMsg("Expense claim saved on this phone and will be sent when you are online. Add the receipt photo from here once it is sent.");
        setClaim({ ...claim, category: "", description: "", amount: "" });
      } else {
        haptic.error();
        setMsg(e instanceof ApiError ? e.message : (e as Error).message);
      }
    }
  }

  const lockSub = lock.cap === "none"
    ? "This device has no screen lock, so the app lock cannot be enforced."
    : lock.cap === "biometric" ? "Fingerprint or face, with the device passcode as fallback." : "No biometrics enrolled: the device passcode is used.";
  return (
    <Screen>
      <ScrollView contentContainerStyle={{ padding: sp.lg, gap: sp.lg }} keyboardShouldPersistTaps="handled">
        <ScreenHeader overline="Identity and security" title="Account" />

        <Card style={{ gap: sp.md }}>
          <View style={{ flexDirection: "row", alignItems: "center", gap: sp.md }}>
            <View style={{ width: 48, height: 48, borderRadius: 24, borderWidth: 1, borderColor: c.gold, backgroundColor: c.raised, alignItems: "center", justifyContent: "center" }}>
              <Icon name="user" size={24} color={c.goldText} />
            </View>
            <View style={{ flex: 1, gap: 2 }}>
              <Text style={[s.text, s.semibold]} numberOfLines={1}>{me?.email || me?.sub}</Text>
              <Text style={[s.mono, { color: c.muted, fontSize: 11 }]} numberOfLines={1}>{me?.scope?.toUpperCase()} · {me?.sub}</Text>
            </View>
          </View>
          <View style={[s.row, { gap: 6 }]}>{me?.roles.map((r) => <Pill key={r} text={roleLabel(r)} tone="gold" />)}</View>
          {pendingActions > 0 && <Notice icon="sync">{pendingActions} update{pendingActions === 1 ? "" : "s"} waiting to be sent.</Notice>}
        </Card>

        <Group title="Security">
          <Card style={{ gap: sp.md }}>
            <View style={{ flexDirection: "row", alignItems: "center", gap: sp.md }}>
              <View style={{ width: 40, height: 40, borderRadius: 10, backgroundColor: c.raised, borderColor: c.line, borderWidth: 1, alignItems: "center", justifyContent: "center" }}>
                <Icon name={lock.enabled && lock.cap !== "none" ? "lock" : "unlock"} size={22} color={lock.enabled && lock.cap !== "none" ? c.ok : c.muted} />
              </View>
              <View style={{ flex: 1, gap: 2 }}>
                <Text style={[s.text, s.semibold, { fontSize: 16 }]}>App lock</Text>
                <Text style={[s.muted, { fontSize: 13 }]}>{lockSub}</Text>
              </View>
              <Switch
                accessibilityLabel="App lock"
                value={lock.enabled && lock.cap !== "none"}
                trackColor={{ true: c.goldFill, false: c.line }}
                thumbColor={c.fg}
                onValueChange={(on) => { haptic.select(); setLockMsg(""); void lock.setEnabled(on).then((m) => setLockMsg(m ?? "")); }}
              />
            </View>
            {!!lockMsg && <Notice tone="warn" icon="alert">{lockMsg}</Notice>}
            <View style={{ height: StyleSheet.hairlineWidth * 2, backgroundColor: c.line }} />
            <KV k="Locks" v="at launch and after 60 s away" />
            <KV k="Session token" v="device secure storage" />
            <KV k="Sign-in" v="OIDC in the system browser" />
          </Card>
        </Group>

        <Group title="Appearance">
          <Segmented options={["Operations (dark)", "Light"] as const} value={mode === "dark" ? "Operations (dark)" : "Light"} onChange={(v) => setMode(v === "Light" ? "light" : "dark")} />
        </Group>

        {!!msg && (/sent|saved/i.test(msg) ? <Notice icon="check" tone="ok">{msg}</Notice> : <Notice tone="bad" icon="alert">{msg}</Notice>)}

        {staff && (
          <Group title="My time and expenses">
            <Card style={{ flexDirection: "row", alignItems: "center", gap: sp.md }}>
              <Icon name="calendar" size={22} color={c.goldText} />
              {rec ? (
                <View style={{ flex: 1 }}>
                  <Text style={s.overline}>Annual leave</Text>
                  <Text style={{ color: c.fg, fontSize: 20, fontFamily: font.monoBold }}>{rec.annual_leave_balance}<Text style={{ color: c.muted, fontSize: 14 }}> of {rec.annual_leave_entitlement} days left</Text></Text>
                </View>
              ) : (
                <Text style={[s.muted, { flex: 1 }]}>{recMsg || "Loading your leave balance…"}</Text>
              )}
            </Card>

            <Accordion icon="clock" title="Log hours" open={open === "hours"} onToggle={() => setOpen(open === "hours" ? null : "hours")}>
              <Input label="Date" mono value={hours.date} onChangeText={(v) => setHours({ ...hours, date: v })} placeholder="YYYY-MM-DD" />
              <Input label="Hours" mono value={hours.hours} onChangeText={(v) => setHours({ ...hours, hours: v })} keyboardType="decimal-pad" placeholder="Hours" />
              <Input label="Note" value={hours.note} onChangeText={(v) => setHours({ ...hours, note: v })} placeholder="What you worked on" />
              <Button title="Save hours" disabled={!(Number(hours.hours) > 0)} onPress={() => void submit("Hours", "/v1/timesheets", { work_date: hours.date, hours: Number(hours.hours), note: hours.note }, () => setHours({ ...hours, note: "" }))} />
            </Accordion>

            <Accordion icon="calendar" title="Request leave" open={open === "leave"} onToggle={() => setOpen(open === "leave" ? null : "leave")}>
              <Input label="From" mono value={leave.start} onChangeText={(v) => setLeave({ ...leave, start: v })} placeholder="YYYY-MM-DD" />
              <Input label="To" mono value={leave.end} onChangeText={(v) => setLeave({ ...leave, end: v })} placeholder="YYYY-MM-DD" />
              <Input label="Reason (optional)" value={leave.reason} onChangeText={(v) => setLeave({ ...leave, reason: v })} />
              <Button title="Request annual leave" disabled={!leave.start || !leave.end} onPress={() => void submit("Leave request", "/v1/leave", { kind: leave.kind, start_date: leave.start, end_date: leave.end, reason: leave.reason }, () => setLeave({ ...leave, start: "", end: "", reason: "" }))} />
            </Accordion>

            <Accordion icon="documents" title="Expense claim" open={open === "claim"} onToggle={() => setOpen(open === "claim" ? null : "claim")}>
              <Input label="Category" value={claim.category} onChangeText={(v) => setClaim({ ...claim, category: v })} placeholder="e.g. travel" />
              <Input label="Description" value={claim.description} onChangeText={(v) => setClaim({ ...claim, description: v })} />
              <Input label="Amount (XAF)" mono value={claim.amount} onChangeText={(v) => setClaim({ ...claim, amount: v })} keyboardType="number-pad" />
              <Button title="Submit claim" disabled={!claim.category || !claim.description || !(Number(claim.amount) > 0)} onPress={() => void submitClaim()} />
              {lastClaim && <PhotoButtons entityType="expense" entityId={lastClaim} />}
            </Accordion>
          </Group>
        )}

        <Button variant="danger" icon="logout" title="Sign out" onPress={() => void signOut()} />
      </ScrollView>
    </Screen>
  );
}
