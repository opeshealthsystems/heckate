import { useCallback, useState } from "react";
import { Pressable, View } from "react-native";
import { Stack, useFocusEffect, useLocalSearchParams } from "expo-router";
import { ErrorBanner } from "../../components/Banner";
import { Button } from "../../components/Button";
import { Loading } from "../../components/EmptyState";
import { useGate } from "../../components/Header";
import { Icon } from "../../components/Icon";
import { Row, Section, SwitchRow } from "../../components/List";
import { AttachmentsSection, PhotoUploadRow } from "../../components/Photos";
import { Screen } from "../../components/Screen";
import { SeverityText, SlaText, STATUS_NAME } from "../../components/Status";
import { Text } from "../../components/Text";
import { Field } from "../../components/TextField";
import { api } from "../../lib/api";
import { haptic } from "../../lib/haptics";
import { useLive } from "../../lib/live";
import { useNow } from "../../lib/now";
import { useSession } from "../../lib/session";
import { sp } from "../../lib/theme";
import type { Comment, Status, Ticket } from "../../lib/types";

const stamp = (s: string | null) => (s ? new Date(s).toLocaleString("en-GB", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" }) : "–");
const RATINGS: [number, string][] = [[5, "Excellent"], [4, "Good"], [3, "Okay"], [2, "Poor"], [1, "Very poor"]];

export default function TicketScreen() {
  const gate = useGate();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { me } = useSession();
  const now = useNow();
  const staff = me?.scope === "staff";
  const [t, setT] = useState<Ticket | null>(null);
  const [comments, setComments] = useState<Comment[]>([]);
  const [body, setBody] = useState("");
  const [internal, setInternal] = useState(false);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [photoKey, setPhotoKey] = useState(0);
  const load = useCallback(async () => {
    try {
      const [tk, cs] = await Promise.all([api.ticket(id), api.comments(id)]);
      setT(tk);
      setComments(cs.data);
      setErr("");
    } catch (e) {
      setErr((e as Error).message);
    }
  }, [id]);
  useFocusEffect(useCallback(() => void load(), [load]));
  // New replies and status changes appear without pulling to refresh.
  useLive((e) => { if (e.subject.startsWith("ticket.") && e.data.id === id) void load(); });
  if (gate) return gate;
  const run = (p: Promise<unknown>) => { setBusy(true); return p.then(() => { haptic.success(); return load(); }).catch((e) => { haptic.error(); setErr(e.message); }).finally(() => setBusy(false)); };

  if (!t) {
    return (
      <Screen>
        <Stack.Screen options={{ title: "Ticket" }} />
        {err ? <><ErrorBanner message={err} /><Button title="Try again" onPress={() => void load()} /></> : <Loading />}
      </Screen>
    );
  }
  const move = (to: Status) => run(api.updateTicket(id, { status: to }, t.version));
  const first = t.first_response_at;
  const open = t.status === "open" || t.status === "in_progress";
  const primary: { title: string; to: Status } | null = staff && t.status === "open" ? { title: "Start work", to: "in_progress" }
    : staff && t.status === "in_progress" ? { title: "Mark resolved", to: "resolved" }
    : t.status === "resolved" ? { title: staff ? "Close ticket" : "Confirm resolved", to: "closed" } : null;

  return (
    <Screen keyboard>
      <Stack.Screen options={{ title: "Ticket" }} />
      {!!err && <ErrorBanner message={err} />}
      <View style={{ gap: sp.xs, paddingHorizontal: sp.xs }}>
        <Text v="title" accessibilityRole="header">{t.subject}</Text>
      </View>

      <Section header="Details">
        <Row title="Status" value={STATUS_NAME[t.status]} />
        <Row title="Severity" right={<SeverityText severity={t.severity} />} />
        {t.sla_due_at && <Row title="Deadline" right={<SlaText ticket={t} now={now} v="body" />} />}
        <Row title="Assignee" value={t.assignee ?? "Unassigned"} />
        <Row title="Reporter" value={t.reporter} />
        <Row title="Opened" value={stamp(t.created_at)} />
        {t.resolved_at && <Row title="Resolved" value={stamp(t.resolved_at)} />}
      </Section>

      {!!t.description && (
        <Section header="Description">
          <Row title={t.description} titleLines={0} />
        </Section>
      )}

      {(primary || t.status === "resolved" || (staff && t.status === "open")) && (
        <View style={{ gap: sp.xs }}>
          {primary && <Button kind="primary" title={primary.title} disabled={busy} onPress={() => void move(primary.to)} />}
          {staff && t.status === "open" && <Button title="Mark resolved" disabled={busy} onPress={() => void move("resolved")} />}
          {t.status === "resolved" && <Button title="Reopen" disabled={busy} onPress={() => void move("in_progress")} />}
        </View>
      )}

      <Section header={`Conversation${comments.length ? ` · ${comments.length}` : ""}`}>
        {comments.map((m) => (
          <Row
            key={m.id}
            title={m.author}
            titleLines={1}
            subtitle={
              <View style={{ gap: 4 }}>
                <Text selectable>{m.body}</Text>
                <Text v="footnote" tone="label2">{stamp(m.created_at)}{m.internal ? " · Internal note" : ""}</Text>
              </View>
            }
          />
        ))}
        {!comments.length && <Row title="No replies yet" />}
      </Section>

      {open && (
        <Section header="Add comment">
          <View style={{ flexDirection: "row", alignItems: "flex-end", paddingRight: sp.xs }}>
            <View style={{ flex: 1 }}>
              <Field multiline placeholder="Write a reply" returnKeyType="default" value={body} onChangeText={setBody} />
            </View>
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="Send comment"
              disabled={!body.trim() || busy}
              hitSlop={4}
              onPress={() => void run(api.addComment(id, body.trim(), internal, `c-${Date.now()}-${Math.random().toString(36).slice(2)}`).then(() => setBody("")))}
              android_ripple={{ color: "#00000014", borderless: true, radius: 22 }}
              style={({ pressed }) => ({ width: 44, height: 44, alignItems: "center", justifyContent: "center", marginBottom: sp.xs, opacity: !body.trim() || busy ? 0.4 : pressed ? 0.5 : 1 })}
            >
              <Icon name="send" size={24} tone="tint" />
            </Pressable>
          </View>
          {staff && <SwitchRow title="Internal note" subtitle="Only staff can see it." value={internal} onValueChange={setInternal} />}
        </Section>
      )}

      <AttachmentsSection entityType="ticket" entityId={id} refreshKey={photoKey}>
        {t.status !== "closed" && <PhotoUploadRow entityType="ticket" entityId={id} onUploaded={() => setPhotoKey((k) => k + 1)} />}
      </AttachmentsSection>

      {!staff && (t.status === "resolved" || t.status === "closed") && (
        <Section header={t.rating ? "Your rating" : "Rate this support"} footer={t.rating ? `You rated this ${t.rating} out of 5.` : undefined}>
          {!t.rating && RATINGS.map(([n, label]) => <Row key={n} title={`${n} · ${label}`} onPress={() => void run(api.rate(id, n))} />)}
        </Section>
      )}
    </Screen>
  );
}
