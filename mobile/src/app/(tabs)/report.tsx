import { useState } from "react";
import { Pressable, ScrollView, Text, TextInput, View } from "react-native";
import { router } from "expo-router";
import { Button } from "../../components/ui";
import { api, ApiError, uploadPhoto } from "../../lib/api";
import { pickPhoto, type Photo } from "../../lib/photos";
import { addDraft, classify } from "../../lib/drafts";
import { useSession } from "../../lib/session";
import { useTheme } from "../../lib/theme";
import type { Severity } from "../../lib/types";

const LEVELS: [Severity, string][] = [
  ["P1", "Critical – service is down"],
  ["P2", "High – major impact"],
  ["P3", "Medium – degraded"],
  ["P4", "Low – question or minor"],
];

export default function Report() {
  const { s, c } = useTheme();
  const { me, refreshDrafts } = useSession();
  const [severity, setSeverity] = useState<Severity>("P3");
  const [subject, setSubject] = useState("");
  const [description, setDescription] = useState("");
  const [tenant, setTenant] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");
  const [photos, setPhotos] = useState<Photo[]>([]);
  const staff = me?.scope === "staff";

  async function submit() {
    setBusy(true);
    setMsg("");
    const body = { subject: subject.trim(), description, severity, channel: "app" as const };
    const key = `app-${Date.now()}-${Math.random().toString(36).slice(2)}`;
    try {
      const t = await api.createTicket(staff ? { ...body, tenant_id: tenant.trim() } : body, key);
      // Photos go up once the ticket exists; a failed upload must not lose the report itself.
      let failed = 0;
      for (const p of photos) {
        try { await uploadPhoto("ticket", t.id, p); } catch { failed++; }
      }
      setSubject("");
      setDescription("");
      setPhotos([]);
      if (failed) setMsg(`Your report was sent, but ${failed} photo(s) could not be uploaded. Add them from the ticket.`);
      router.push(`/ticket/${t.id}`);
    } catch (e) {
      const status = e instanceof ApiError ? e.status : null;
      if (!staff && classify(status) === "retry") {
        await addDraft(body); // no connection: keep it and send automatically later (photos are not kept offline)
        await refreshDrafts();
        setSubject("");
        setDescription("");
        setPhotos([]);
        setMsg(photos.length ? "You are offline. Your report is saved and will be sent automatically; add the photos from the ticket afterwards." : "You are offline. Your report is saved and will be sent automatically.");
      } else {
        setMsg((e as Error).message);
      }
    }
    setBusy(false);
  }

  return (
    <ScrollView style={s.screen} contentContainerStyle={s.pad} keyboardShouldPersistTaps="handled">
      <Text style={s.h1}>{staff ? "New ticket" : "Report an incident"}</Text>
      {staff && (
        <>
          <Text style={s.label}>Client tenant id</Text>
          <TextInput style={s.input} autoCapitalize="none" value={tenant} onChangeText={setTenant} />
        </>
      )}
      <Text style={s.label}>How urgent is it?</Text>
      {LEVELS.map(([v, label]) => (
        <Pressable key={v} accessibilityRole="radio" accessibilityState={{ selected: severity === v }} onPress={() => setSeverity(v)} style={[s.card, severity === v && { borderColor: c.brand, borderWidth: 2 }]}>
          <Text style={s.text}>{v} · {label}</Text>
        </Pressable>
      ))}
      {severity === "P1" && <Text style={{ color: c.warn }}>A P1 pages our on-call engineer immediately.</Text>}
      <Text style={s.label}>Subject</Text>
      <TextInput style={s.input} maxLength={200} value={subject} onChangeText={setSubject} />
      <Text style={s.label}>What is happening?</Text>
      <TextInput style={[s.input, { minHeight: 120, textAlignVertical: "top" }]} multiline value={description} onChangeText={setDescription} />
      {!staff && (
        <>
          <Text style={s.label}>Photos or screenshots ({photos.length})</Text>
          <View style={s.row}>
            <Button title="Take a photo" onPress={() => void pickPhoto("camera").then((p) => p && setPhotos((cur) => [...cur, p])).catch((e) => setMsg(e.message))} />
            <Button title="Choose a photo" onPress={() => void pickPhoto("library").then((p) => p && setPhotos((cur) => [...cur, p])).catch((e) => setMsg(e.message))} />
          </View>
        </>
      )}
      {!!msg && <Text style={msg.startsWith("You are offline") ? s.muted : s.error}>{msg}</Text>}
      <View>
        <Button primary title={busy ? "Sending…" : "Send"} disabled={busy || !subject.trim() || (staff && !tenant.trim())} onPress={submit} />
      </View>
    </ScrollView>
  );
}
