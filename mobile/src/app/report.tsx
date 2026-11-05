import { useEffect, useMemo, useState } from "react";
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
import type { Client, Severity } from "../lib/types";

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
  // Staff raise tickets for a client: pick the company by name instead of typing its id.
  const [clients, setClients] = useState<Client[]>([]);
  const [find, setFind] = useState("");
  useEffect(() => { if (staff) api.clients().then((r) => setClients(r.data)).catch((e) => setMsg((e as Error).message)); }, [staff]);
  const picked = clients.find((x) => x.tenant_id === tenant);
  const matches = useMemo(() => {
    const n = find.trim().toLowerCase();
    return (n ? clients.filter((x) => x.legal_name.toLowerCase().includes(n)) : clients).slice(0, 6);
  }, [clients, find]);
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
        await addDraft(body, photos); // no connection: keep the report and its photos and send them automatically later
        await refreshDrafts();
        haptic.warning();
        setSubject("");
        setDescription("");
        setPhotos([]);
        setMsg(photos.length ? "You are offline. Your report and photos are saved and will be sent automatically." : "You are offline. Your report is saved and will be sent automatically.");
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
        <Section header="Client" footer={!picked && clients.length > 6 ? "Type part of the name to find a client." : undefined}>
          {picked ? (
            <Row title={picked.legal_name} subtitle="Selected client" right={<Icon name="check" size={20} tone="tint" />} onPress={() => setTenant("")} accessibilityLabel={`${picked.legal_name}, selected. Tap to change.`} />
          ) : (
            <>
              <Field placeholder="Search clients" autoCapitalize="none" autoCorrect={false} returnKeyType="search" value={find} onChangeText={setFind} />
              {matches.map((x) => <Row key={x.tenant_id} title={x.legal_name} onPress={() => { haptic.select(); setTenant(x.tenant_id); setFind(""); }} />)}
              {!matches.length && <Row title={clients.length ? "No client matches" : "Loading clients…"} />}
            </>
          )}
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
