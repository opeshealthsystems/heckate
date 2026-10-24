import { useState } from "react";
import { View } from "react-native";
import { router, Stack } from "expo-router";
import { Banner } from "../components/Banner";
import { Button } from "../components/Button";
import { HeaderButton, useGate } from "../components/Header";
import { Icon } from "../components/Icon";
import { Row, Section } from "../components/List";
import { AddPhotoRow } from "../components/Photos";
import { Screen } from "../components/Screen";
import { Field } from "../components/TextField";
import { api, ApiError, uploadPhoto } from "../lib/api";
import { addDraft, classify } from "../lib/drafts";
import { haptic } from "../lib/haptics";
import type { Photo } from "../lib/photos";
import { useSession } from "../lib/session";
import { sp } from "../lib/theme";
import type { Severity } from "../lib/types";

const LEVELS: [Severity, string, string][] = [
  ["P1", "Critical", "Service is down"],
  ["P2", "High", "Major impact"],
  ["P3", "Medium", "Degraded"],
  ["P4", "Low", "Question or minor"],
];

const close = () => (router.canGoBack() ? router.back() : router.replace("/(tabs)/tickets"));

export default function Report() {
  const gate = useGate();
  const { me, refreshDrafts } = useSession();
  const [severity, setSeverity] = useState<Severity>("P3");
  const [subject, setSubject] = useState("");
  const [description, setDescription] = useState("");
  const [tenant, setTenant] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");
  const [photos, setPhotos] = useState<Photo[]>([]);
  const staff = me?.scope === "staff";
  if (gate) return gate;

  async function submit() {
    setBusy(true);
    setMsg("");
    const body = { subject: subject.trim(), description, severity, channel: "app" as const };
    const key = `app-${Date.now()}-${Math.random().toString(36).slice(2)}`;
    try {
      const t = await api.createTicket(staff ? { ...body, tenant_id: tenant.trim() } : body, key);
      // Photos go up once the ticket exists; a failed upload must not lose the report itself.
      for (const p of photos) {
        try { await uploadPhoto("ticket", t.id, p); } catch { /* the ticket screen offers to add photos again */ }
      }
      haptic.success();
      setSubject("");
      setDescription("");
      setPhotos([]);
      router.replace(`/ticket/${t.id}`);
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

  const title = staff ? "New ticket" : "Report an incident";
  return (
    <Screen keyboard>
      <Stack.Screen options={{ title, headerLeft: () => <HeaderButton title="Cancel" label="Cancel" onPress={close} /> }} />
      {staff && (
        <Section header="Client">
          <Field label="Tenant id" placeholder="Client tenant id" mono autoCapitalize="none" autoCorrect={false} autoComplete="off" returnKeyType="next" value={tenant} onChangeText={setTenant} />
        </Section>
      )}

      <Section header="How urgent is it?" footer={severity === "P1" ? "A critical report pages our on-call engineer immediately." : undefined}>
        {LEVELS.map(([v, name, hint]) => (
          <Row
            key={v}
            title={name}
            subtitle={hint}
            accessibilityLabel={`${v} ${name}, ${hint}${severity === v ? ", selected" : ""}`}
            right={severity === v ? <Icon name="check" size={20} tone="tint" /> : <View style={{ width: 20 }} />}
            onPress={() => { haptic.select(); setSeverity(v); }}
          />
        ))}
      </Section>

      <Section header="Subject">
        <Field placeholder="Short summary" maxLength={200} returnKeyType="next" value={subject} onChangeText={setSubject} />
      </Section>
      <Section header="What is happening?">
        <Field multiline placeholder="Describe the problem" value={description} onChangeText={setDescription} />
      </Section>

      {!staff && (
        <Section header="Photos" footer="Add a photo or screenshot of the problem.">
          <AddPhotoRow subtitle={photos.length ? `${photos.length} attached` : undefined} onError={setMsg} onPhoto={(p) => setPhotos((cur) => [...cur, p])} />
        </Section>
      )}

      {!!msg && (msg.startsWith("You are offline") ? <Banner icon="offline">{msg}</Banner> : <Banner icon="warning" tone="red">{msg}</Banner>)}
      <View style={{ paddingBottom: sp.sm }}>
        <Button kind="primary" title="Send report" loading={busy} disabled={!subject.trim() || (staff && !tenant.trim())} onPress={() => void submit()} />
      </View>
    </Screen>
  );
}
