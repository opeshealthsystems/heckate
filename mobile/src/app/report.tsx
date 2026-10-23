import { useState } from "react";
import { KeyboardAvoidingView, Platform, Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { router } from "expo-router";
import { Icon } from "../../components/Icon";
import { Button, Input, Notice, Screen, ScreenHeader, SevBadge, sevColor } from "../../components/ui";
import { api, ApiError, uploadPhoto } from "../../lib/api";
import { addDraft, classify } from "../../lib/drafts";
import { haptic } from "../../lib/haptics";
import { pickPhoto, type Photo } from "../../lib/photos";
import { useSession } from "../../lib/session";
import { font, radius, sp, useTheme } from "../../lib/theme";
import type { Severity } from "../../lib/types";

const LEVELS: [Severity, string, string][] = [
  ["P1", "Critical", "Service is down"],
  ["P2", "High", "Major impact"],
  ["P3", "Medium", "Degraded"],
  ["P4", "Low", "Question or minor"],
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
      haptic.success();
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
        haptic.warning();
        setSubject("");
        setDescription("");
        setPhotos([]);
        setMsg(photos.length ? "You are offline. Your report is saved and will be sent automatically; add the photos from the ticket afterwards." : "You are offline. Your report is saved and will be sent automatically.");
      } else {
        haptic.error();
        setMsg((e as Error).message);
      }
    }
    setBusy(false);
  }

  return (
    <Screen>
      <KeyboardAvoidingView style={{ flex: 1 }} behavior={Platform.OS === "ios" ? "padding" : undefined}>
        <ScrollView contentContainerStyle={{ padding: sp.lg, gap: sp.md }} keyboardShouldPersistTaps="handled">
          <ScreenHeader overline={staff ? "On behalf of a client" : "Incident intake"} title={staff ? "New ticket" : "Report an incident"} />
          {staff && <Input label="Client tenant id" mono autoCapitalize="none" autoCorrect={false} value={tenant} onChangeText={setTenant} />}

          <Text style={s.label}>How urgent is it?</Text>
          <View style={{ gap: sp.sm }} accessibilityRole="radiogroup">
            {LEVELS.map(([v, name, hint]) => {
              const on = severity === v;
              const col = sevColor(c, v);
              return (
                <Pressable key={v} accessibilityRole="radio" accessibilityState={{ selected: on }} accessibilityLabel={`${v} ${name}, ${hint}`} onPress={() => { haptic.select(); setSeverity(v); }}
                  style={{ minHeight: 56, flexDirection: "row", alignItems: "center", gap: sp.md, padding: sp.md, paddingLeft: sp.lg, backgroundColor: on ? c.raised : c.surface, borderRadius: radius.md, borderWidth: on ? 1.5 : StyleSheet.hairlineWidth * 2, borderColor: on ? c.gold : c.line, overflow: "hidden" }}>
                  <View style={{ position: "absolute", left: 0, top: 0, bottom: 0, width: 3, backgroundColor: col }} />
                  <SevBadge sev={v} />
                  <View style={{ flex: 1 }}>
                    <Text style={{ color: c.fg, fontSize: 16, fontFamily: font.semibold }}>{name}</Text>
                    <Text style={s.muted}>{hint}</Text>
                  </View>
                  {on && <Icon name="check" size={20} color={c.goldText} stroke={2.2} />}
                </Pressable>
              );
            })}
          </View>
          {severity === "P1" && <Notice icon="alert" tone="warn">A P1 pages our on-call engineer immediately.</Notice>}

          <Input label="Subject" maxLength={200} value={subject} onChangeText={setSubject} />
          <Input label="What is happening?" multiline value={description} onChangeText={setDescription} />
          {!staff && (
            <View style={{ gap: 8 }}>
              <Text style={s.label}>Photos or screenshots · {photos.length}</Text>
              <View style={{ flexDirection: "row", gap: 8 }}>
                <Button small icon="camera" title="Take photo" style={{ flex: 1 }} onPress={() => void pickPhoto("camera").then((p) => p && setPhotos((cur) => [...cur, p])).catch((e) => setMsg(e.message))} />
                <Button small icon="image" title="Choose photo" style={{ flex: 1 }} onPress={() => void pickPhoto("library").then((p) => p && setPhotos((cur) => [...cur, p])).catch((e) => setMsg(e.message))} />
              </View>
            </View>
          )}
          {!!msg && (msg.startsWith("You are offline") ? <Notice icon="sync">{msg}</Notice> : <Notice tone="bad" icon="alert">{msg}</Notice>)}
          <Button primary icon="send" title={busy ? "Sending…" : "Send report"} loading={busy} disabled={!subject.trim() || (staff && !tenant.trim())} onPress={submit} />
        </ScrollView>
      </KeyboardAvoidingView>
    </Screen>
  );
}
