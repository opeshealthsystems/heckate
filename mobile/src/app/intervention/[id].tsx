import { useCallback, useEffect, useState } from "react";
import { View } from "react-native";
import { Stack, useFocusEffect, useLocalSearchParams } from "expo-router";
import { Banner, ErrorBanner } from "../../components/Banner";
import { Button } from "../../components/Button";
import { Loading } from "../../components/EmptyState";
import { useGate } from "../../components/Header";
import { Row, Section } from "../../components/List";
import { Screen } from "../../components/Screen";
import { SignatureSheet } from "../../components/SignaturePad";
import { visitName } from "../../components/Status";
import { Text } from "../../components/Text";
import { Field } from "../../components/TextField";
import { api, ApiError, replay } from "../../lib/api";
import { currentFix } from "../../lib/gps";
import { haptic } from "../../lib/haptics";
import { enqueue } from "../../lib/queue";
import { newKey } from "../../lib/queue-core";
import { useSession } from "../../lib/session";
import { sp } from "../../lib/theme";
import type { Intervention, StockItem, Warehouse } from "../../lib/types";

const stamp = (s: string | null) => (s ? new Date(s).toLocaleString("en-GB", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" }) : "–");

/** Scheduled, On site, Completed as three plain rows: a check for each step reached, the current one in semibold. */
function Progress({ v }: { v: Intervention }) {
  const at = v.status === "scheduled" ? 0 : v.status === "in_progress" ? 1 : 2;
  const steps: [string, string | null][] = [["Scheduled", v.scheduled_for], ["On site", v.checkin_at], ["Completed", v.checkout_at]];
  return (
    <Section header="Progress">
      {steps.map(([label, time], i) => (
        <Row key={label} icon={i <= at ? "checkCircle" : "circle"} iconTone={i <= at ? "tint" : "label3"} title={label} value={time ? stamp(time) : undefined} accessibilityLabel={`${label}${i <= at ? ", reached" : ", not yet"}`} />
      ))}
    </Section>
  );
}

export default function InterventionScreen() {
  const gate = useGate();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { refreshQueue } = useSession();
  const [v, setV] = useState<Intervention | null>(null);
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState(false);
  const [report, setReport] = useState("");
  const [signedBy, setSignedBy] = useState("");
  const [signature, setSignature] = useState<string | null>(null);
  const [padOpen, setPadOpen] = useState(false);
  const [items, setItems] = useState<StockItem[]>([]);
  const [wh, setWh] = useState<Warehouse | null>(null);
  const [qty, setQty] = useState<Record<string, number>>({});

  const load = useCallback(async () => {
    try {
      setV(await api.intervention(id));
    } catch (e) {
      setMsg((e as Error).message);
    }
  }, [id]);
  useFocusEffect(useCallback(() => void load(), [load]));
  useEffect(() => {
    api.items().then((r) => setItems(r.data.filter((i) => !i.serialised && i.on_hand > 0))).catch(() => {});
    api.warehouses().then((r) => setWh(r.data[0] ?? null)).catch(() => {});
  }, []);
  if (gate) return gate;

  /** Tries the call now; with no connection it stays on the phone with the real time of the action. */
  async function act(label: string, path: string, body: Record<string, unknown>, after: Partial<Intervention>) {
    setBusy(true);
    setMsg("");
    const action = { key: newKey("field"), method: "POST" as const, path, body, createdAt: Date.now(), label };
    try {
      const result = await replay(action);
      if (result === "retry") {
        await enqueue(action);
        await refreshQueue();
        setV((cur) => (cur ? { ...cur, ...after } : cur));
        haptic.warning();
        setMsg("Saved on this phone. It will be sent automatically when you are back online.");
      } else if (result === "drop") {
        haptic.error();
        setMsg("The server refused this update. Pull to refresh and check the visit's status.");
        await load();
      } else {
        haptic.success();
        await load();
      }
    } catch (e) {
      haptic.error();
      setMsg(e instanceof ApiError ? e.message : (e as Error).message);
    }
    setBusy(false);
  }

  async function checkIn() {
    setBusy(true);
    try {
      const fix = await currentFix();
      await act("Check in", `/v1/interventions/${id}/check-in`, { ...fix, at: new Date().toISOString() }, { status: "in_progress" });
    } catch (e) {
      haptic.error();
      setMsg((e as Error).message);
      setBusy(false);
    }
  }

  async function checkOut() {
    setBusy(true);
    try {
      const fix = await currentFix();
      const parts = wh ? Object.entries(qty).filter(([, n]) => n > 0).map(([item_id, n]) => ({ item_id, warehouse_id: wh.id, qty: n })) : [];
      await act("Check out", `/v1/interventions/${id}/check-out`, {
        ...fix, at: new Date().toISOString(), report: report.trim(),
        ...(signature ? { signed_by: signedBy.trim(), signature } : {}), parts,
      }, { status: "completed" });
    } catch (e) {
      haptic.error();
      setMsg((e as Error).message);
      setBusy(false);
    }
  }

  if (!v) {
    return (
      <Screen>
        <Stack.Screen options={{ title: "Site visit" }} />
        {msg ? <><ErrorBanner message={msg} /><Button title="Try again" onPress={() => void load()} /></> : <Loading />}
      </Screen>
    );
  }
  const canFinish = report.trim().length >= 10 && (!signature || signedBy.trim().length > 0);
  const saved = msg.startsWith("Saved");
  return (
    <Screen keyboard>
      <Stack.Screen options={{ title: "Site visit" }} />
      <View style={{ paddingHorizontal: sp.xs }}>
        <Text v="title" accessibilityRole="header">{v.summary || "Site visit"}</Text>
      </View>
      {!!msg && (saved ? <Banner icon="sync">{msg}</Banner> : <ErrorBanner message={msg} />)}

      <Progress v={v} />

      <Section header="Details">
        <Row title="Status" value={visitName(v.status)} />
        <Row title="Engineer" value={v.engineer} />
        <Row title="Scheduled" value={stamp(v.scheduled_for)} />
        {v.checkin_at && <Row title="Checked in" value={stamp(v.checkin_at)} />}
      </Section>

      {v.status === "scheduled" && (
        <View style={{ gap: sp.sm }}>
          <Button kind="primary" title={busy ? "Getting position…" : "Check in"} loading={busy} onPress={() => void checkIn()} />
          <Text v="footnote" tone="label2" style={{ paddingHorizontal: sp.lg }}>Your position and the time are recorded when you check in and out.</Text>
        </View>
      )}

      {v.status === "in_progress" && (
        <>
          <Section header="Work done" footer={`${report.trim().length} of 10 characters minimum`}>
            <Field multiline placeholder="What you found and what you did" value={report} onChangeText={setReport} />
          </Section>

          {items.length > 0 && wh && (
            <Section header={`Spare parts used · from ${wh.name}`}>
              {items.map((i) => (
                <Row
                  key={i.id}
                  title={i.name}
                  subtitle={`${i.sku} · ${i.on_hand} in stock`}
                  right={
                    <View style={{ flexDirection: "row", alignItems: "center" }}>
                      <Button title="−" accessibilityHint={`Remove one ${i.name}`} onPress={() => setQty({ ...qty, [i.id]: Math.max(0, (qty[i.id] ?? 0) - 1) })} style={{ minWidth: 44 }} />
                      <Text style={{ minWidth: 24, textAlign: "center" }}>{qty[i.id] ?? 0}</Text>
                      <Button title="+" accessibilityHint={`Add one ${i.name}`} onPress={() => setQty({ ...qty, [i.id]: Math.min(i.on_hand, (qty[i.id] ?? 0) + 1) })} style={{ minWidth: 44 }} />
                    </View>
                  }
                />
              ))}
            </Section>
          )}

          <Section header="Client acceptance" footer={signature && !signedBy.trim() ? "Enter the name of the person who signed." : undefined} footerTone="red">
            <Field label="Name" placeholder="Person signing" autoComplete="name" textContentType="name" returnKeyType="done" value={signedBy} onChangeText={setSignedBy} />
            <Row icon="signature" action={!signature} title="Signature" value={signature ? "Captured" : undefined} onPress={() => setPadOpen(true)} chevron={!!signature} />
          </Section>
          <SignatureSheet visible={padOpen} onClose={() => setPadOpen(false)} onDone={setSignature} />

          <View style={{ gap: sp.sm }}>
            <Button kind="primary" title="Check out" loading={busy} disabled={!canFinish} onPress={() => void checkOut()} />
            {!canFinish && <Text v="footnote" tone="label2" style={{ paddingHorizontal: sp.lg }}>Write at least 10 characters of report to check out.</Text>}
          </View>
        </>
      )}

      {v.status === "completed" && (
        <Section header="Completed">
          {v.checkout_at && <Row title="Completed" value={stamp(v.checkout_at)} />}
          {v.document_ref && <Row title="Acceptance document" right={<Text mono tone="label2" selectable>{v.document_ref}</Text>} />}
          {v.signed_by && <Row title="Signed by" value={v.signed_by} />}
          {!!v.report && <Row title={v.report} titleLines={0} />}
        </Section>
      )}
    </Screen>
  );
}
