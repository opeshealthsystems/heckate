import { useCallback, useEffect, useState } from "react";
import { KeyboardAvoidingView, Platform, ScrollView, StyleSheet, Text, View } from "react-native";
import { router, useFocusEffect, useLocalSearchParams } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { Icon } from "../../components/Icon";
import { SignaturePad } from "../../components/SignaturePad";
import { Button, Card, DetailHeader, ErrorState, Input, KV, Notice, Pill, Screen, Skeleton, visitColor, visitTone } from "../../components/ui";
import { api, ApiError, replay } from "../../lib/api";
import { currentFix } from "../../lib/gps";
import { haptic } from "../../lib/haptics";
import { enqueue } from "../../lib/queue";
import { newKey } from "../../lib/queue-core";
import { useSession } from "../../lib/session";
import { font, radius, sp, useTheme } from "../../lib/theme";
import type { Intervention, StockItem, Warehouse } from "../../lib/types";

const stamp = (s: string | null) => (s ? new Date(s).toLocaleString("en-GB", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit", second: "2-digit" }) : "–");

/** Three-step track: scheduled, on site, completed. The line fills as the visit advances. */
function Track({ v }: { v: Intervention }) {
  const { c, s } = useTheme();
  const at = v.status === "scheduled" ? 0 : v.status === "in_progress" ? 1 : 2;
  const steps: [string, string | null][] = [["Scheduled", v.scheduled_for], ["On site", v.checkin_at], ["Completed", v.checkout_at]];
  return (
    <View style={{ flexDirection: "row", gap: 0 }} accessible accessibilityLabel={`Step ${at + 1} of 3: ${steps[at][0]}`}>
      {steps.map(([label, time], i) => {
        const done = i <= at;
        const col = i === at ? visitColor(c, v.status) : done ? c.ok : c.line;
        return (
          <View key={label} style={{ flex: 1, gap: 6 }}>
            <View style={{ flexDirection: "row", alignItems: "center" }}>
              <View style={{ width: 22, height: 22, borderRadius: 11, borderWidth: 2, borderColor: col, backgroundColor: done && i < at ? col : c.surface, alignItems: "center", justifyContent: "center" }}>
                {done && i < at ? <Icon name="check" size={13} color={c.onGold} stroke={3} /> : <View style={{ width: 6, height: 6, borderRadius: 3, backgroundColor: done ? col : "transparent" }} />}
              </View>
              {i < 2 && <View style={{ flex: 1, height: 2, backgroundColor: i < at ? c.ok : c.line }} />}
            </View>
            <Text style={[s.overline, { color: done ? c.fg : c.muted, fontSize: 10 }]}>{label}</Text>
            <Text style={[s.mono, { fontSize: 10.5, color: c.muted }]}>{time ? new Date(time).toLocaleString("en-GB", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" }) : "–"}</Text>
          </View>
        );
      })}
    </View>
  );
}

export default function InterventionScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { s, c } = useTheme();
  const { refreshQueue } = useSession();
  const insets = useSafeAreaInsets();
  const [v, setV] = useState<Intervention | null>(null);
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState(false);
  const [report, setReport] = useState("");
  const [signedBy, setSignedBy] = useState("");
  const [signature, setSignature] = useState<string | null>(null);
  const [items, setItems] = useState<StockItem[]>([]);
  const [wh, setWh] = useState<Warehouse | null>(null);
  const [qty, setQty] = useState<Record<string, number>>({});
  const [updated, setUpdated] = useState<Date | null>(null);

  const load = useCallback(async () => {
    try {
      setV(await api.intervention(id));
      setUpdated(new Date());
    } catch (e) {
      setMsg((e as Error).message);
    }
  }, [id]);
  useFocusEffect(useCallback(() => void load(), [load]));
  useEffect(() => {
    api.items().then((r) => setItems(r.data.filter((i) => !i.serialised && i.on_hand > 0))).catch(() => {});
    api.warehouses().then((r) => setWh(r.data[0] ?? null)).catch(() => {});
  }, []);

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

  const back = () => (router.canGoBack() ? router.back() : router.replace("/(tabs)/field"));
  if (!v) {
    return (
      <Screen>
        <View style={{ padding: sp.lg, gap: sp.md }}>
          <DetailHeader onBack={back} title="Site visit" />
          {msg ? <ErrorState message={msg} onRetry={load} /> : <><Skeleton h={90} r={12} /><Skeleton h={140} r={12} /></>}
        </View>
      </Screen>
    );
  }
  const canFinish = report.trim().length >= 10 && (!signature || signedBy.trim().length > 0);
  return (
    <Screen updated={updated}>
      <KeyboardAvoidingView style={{ flex: 1 }} behavior={Platform.OS === "ios" ? "padding" : undefined}>
        <ScrollView contentContainerStyle={{ padding: sp.lg, paddingBottom: insets.bottom + sp.xl, gap: sp.lg }} keyboardShouldPersistTaps="handled">
          <DetailHeader onBack={back} overline={`VISIT ${v.id.slice(0, 8).toUpperCase()}`} title={v.summary || "Site visit"} right={<Pill text={v.status.replace("_", " ")} tone={visitTone(v.status)} />} />
          <Card style={{ gap: sp.md }}>
            <Track v={v} />
            <View style={{ height: StyleSheet.hairlineWidth * 2, backgroundColor: c.line }} />
            <KV k="Scheduled" v={stamp(v.scheduled_for)} mono />
            <KV k="Engineer" v={v.engineer} mono />
            {v.checkin_at && <KV k="Checked in" v={stamp(v.checkin_at)} mono />}
          </Card>

          {!!msg && (msg.startsWith("Saved") ? <Notice icon="sync">{msg}</Notice> : <Notice tone="bad" icon="alert">{msg}</Notice>)}

          {v.status === "scheduled" && (
            <View style={{ gap: sp.sm }}>
              <Button primary icon="field" title={busy ? "Getting position…" : "Check in on site"} loading={busy} onPress={() => void checkIn()} />
              <View style={{ flexDirection: "row", gap: 6, alignItems: "center" }}>
                <Icon name="info" size={14} color={c.muted} />
                <Text style={[s.muted, { fontSize: 13, flex: 1 }]}>Your position and the time are recorded when you check in and out.</Text>
              </View>
            </View>
          )}

          {v.status === "in_progress" && (
            <>
              <Input label="Work done" multiline value={report} onChangeText={setReport} placeholder="What you found and what you did (at least 10 characters)" hint={`${report.trim().length}/10 characters minimum`} />
              {items.length > 0 && wh && (
                <View style={{ gap: sp.sm }}>
                  <Text style={s.label}>Spare parts used · from {wh.name}</Text>
                  <Card style={{ padding: 0, gap: 0 }}>
                    {items.map((i, k) => (
                      <View key={i.id} style={{ flexDirection: "row", alignItems: "center", gap: sp.sm, padding: sp.md, borderBottomColor: c.lineSoft, borderBottomWidth: k === items.length - 1 ? 0 : 1 }}>
                        <View style={{ flex: 1, gap: 2 }}>
                          <Text style={[s.mono, { color: c.goldText }]}>{i.sku}</Text>
                          <Text style={[s.text, { fontSize: 15 }]}>{i.name}</Text>
                          <Text style={[s.muted, { fontSize: 12 }]}>{i.on_hand} in stock</Text>
                        </View>
                        <Button small title="−" accessibilityLabel={`Remove one ${i.name}`} onPress={() => setQty({ ...qty, [i.id]: Math.max(0, (qty[i.id] ?? 0) - 1) })} style={{ minWidth: 44, paddingHorizontal: 0 }} />
                        <Text style={{ color: c.fg, fontSize: 18, fontFamily: font.monoBold, minWidth: 24, textAlign: "center" }}>{qty[i.id] ?? 0}</Text>
                        <Button small title="+" onPress={() => setQty({ ...qty, [i.id]: Math.min(i.on_hand, (qty[i.id] ?? 0) + 1) })} style={{ minWidth: 44, paddingHorizontal: 0 }} />
                      </View>
                    ))}
                  </Card>
                </View>
              )}
              <View style={{ gap: sp.sm }}>
                <Input label="Client acceptance" value={signedBy} onChangeText={setSignedBy} placeholder="Name of the person signing" />
                <SignaturePad onDone={setSignature} />
              </View>
              <Button primary icon="shieldCheck" title={busy ? "Completing…" : "Complete the visit"} loading={busy} disabled={!canFinish} onPress={() => void checkOut()} />
              {!canFinish && <Text style={[s.muted, { fontSize: 13 }]}>Write at least 10 characters of report{signature && !signedBy.trim() ? " and the signer's name" : ""} to complete.</Text>}
            </>
          )}

          {v.status === "completed" && (
            <Card tone="ok" style={{ gap: sp.sm }}>
              <View style={{ flexDirection: "row", alignItems: "center", gap: 8 }}>
                <Icon name="shieldCheck" size={20} color={c.ok} />
                <Text style={[s.semibold, { color: c.ok, fontSize: 16 }]}>Visit completed</Text>
              </View>
              {v.checkout_at && <KV k="Completed" v={stamp(v.checkout_at)} mono />}
              {v.document_ref && <KV k="Acceptance document" v={v.document_ref} mono />}
              {v.signed_by && <KV k="Signed by" v={v.signed_by} />}
              {!!v.report && <Text style={[s.muted, { color: c.text2, borderRadius: radius.sm }]}>{v.report}</Text>}
            </Card>
          )}
        </ScrollView>
      </KeyboardAvoidingView>
    </Screen>
  );
}
