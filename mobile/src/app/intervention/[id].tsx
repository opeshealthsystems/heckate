import { useCallback, useEffect, useState } from "react";
import { ScrollView, Text, TextInput, View } from "react-native";
import { useFocusEffect, useLocalSearchParams } from "expo-router";
import { Button, Pill } from "../../components/ui";
import { SignaturePad } from "../../components/SignaturePad";
import { api, ApiError, replay } from "../../lib/api";
import { currentFix } from "../../lib/gps";
import { enqueue } from "../../lib/queue";
import { newKey } from "../../lib/queue-core";
import { useSession } from "../../lib/session";
import { useTheme } from "../../lib/theme";
import type { Intervention, StockItem, Warehouse } from "../../lib/types";

export default function InterventionScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { s, c } = useTheme();
  const { refreshQueue } = useSession();
  const [v, setV] = useState<Intervention | null>(null);
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState(false);
  const [report, setReport] = useState("");
  const [signedBy, setSignedBy] = useState("");
  const [signature, setSignature] = useState<string | null>(null);
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
        setMsg("Saved on this phone. It will be sent automatically when you are back online.");
      } else if (result === "drop") {
        setMsg("The server refused this update. Pull to refresh and check the visit's status.");
        await load();
      } else {
        await load();
      }
    } catch (e) {
      setMsg(e instanceof ApiError ? e.message : (e as Error).message);
    }
    setBusy(false);
  }

  async function checkIn() {
    try {
      const fix = await currentFix();
      await act("Check in", `/v1/interventions/${id}/check-in`, { ...fix, at: new Date().toISOString() }, { status: "in_progress" });
    } catch (e) {
      setMsg((e as Error).message);
    }
  }

  async function checkOut() {
    try {
      const fix = await currentFix();
      const parts = wh ? Object.entries(qty).filter(([, n]) => n > 0).map(([item_id, n]) => ({ item_id, warehouse_id: wh.id, qty: n })) : [];
      await act("Check out", `/v1/interventions/${id}/check-out`, {
        ...fix, at: new Date().toISOString(), report: report.trim(),
        ...(signature ? { signed_by: signedBy.trim(), signature } : {}), parts,
      }, { status: "completed" });
    } catch (e) {
      setMsg((e as Error).message);
    }
  }

  if (!v) return <View style={s.pad}>{msg ? <Text style={s.error}>{msg}</Text> : <Text style={s.muted}>Loading…</Text>}</View>;
  const canFinish = report.trim().length >= 10 && (!signature || signedBy.trim().length > 0);
  return (
    <ScrollView style={s.screen} contentContainerStyle={s.pad} keyboardShouldPersistTaps="handled">
      <Text style={s.h1}>{v.summary || "Site visit"}</Text>
      <View style={s.row}><Pill text={v.status.replace("_", " ")} tone={v.status === "completed" ? "ok" : undefined} /></View>
      {!!msg && <Text style={msg.startsWith("Saved") ? s.muted : s.error}>{msg}</Text>}

      {v.status === "scheduled" && <Button primary title={busy ? "Getting position…" : "Check in on site"} disabled={busy} onPress={() => void checkIn()} />}

      {v.status === "in_progress" && (
        <>
          <Text style={s.label}>Work done</Text>
          <TextInput style={[s.input, { minHeight: 110, textAlignVertical: "top" }]} multiline value={report} onChangeText={setReport} placeholder="What you found and what you did (at least 10 characters)" placeholderTextColor="#888" />
          {items.length > 0 && wh && (
            <>
              <Text style={s.label}>Spare parts used (from {wh.name})</Text>
              {items.map((i) => (
                <View key={i.id} style={[s.row, { justifyContent: "space-between" }]}>
                  <Text style={[s.text, { flex: 1 }]}>{i.sku} · {i.name} ({i.on_hand} in stock)</Text>
                  <View style={s.row}>
                    <Button title="−" onPress={() => setQty({ ...qty, [i.id]: Math.max(0, (qty[i.id] ?? 0) - 1) })} />
                    <Text style={[s.text, { minWidth: 24, textAlign: "center" }]}>{qty[i.id] ?? 0}</Text>
                    <Button title="+" onPress={() => setQty({ ...qty, [i.id]: Math.min(i.on_hand, (qty[i.id] ?? 0) + 1) })} />
                  </View>
                </View>
              ))}
            </>
          )}
          <Text style={s.label}>Client acceptance</Text>
          <TextInput style={s.input} value={signedBy} onChangeText={setSignedBy} placeholder="Name of the person signing" placeholderTextColor="#888" />
          <SignaturePad onDone={setSignature} />
          <Button primary title={busy ? "Completing…" : "Complete the visit"} disabled={busy || !canFinish} onPress={() => void checkOut()} />
          <Text style={{ color: c.muted, fontSize: 14 }}>Your position is recorded when you check in and out.</Text>
        </>
      )}

      {v.status === "completed" && (
        <View style={s.card}>
          <Text style={s.text}>Completed{v.checkout_at ? ` ${new Date(v.checkout_at).toLocaleString()}` : ""}.</Text>
          {v.document_ref && <Text style={s.muted}>Acceptance document {v.document_ref}</Text>}
          {v.signed_by && <Text style={s.muted}>Signed by {v.signed_by}</Text>}
        </View>
      )}
    </ScrollView>
  );
}
