import { useCallback, useState } from "react";
import { KeyboardAvoidingView, Platform, RefreshControl, ScrollView, StyleSheet, Switch, Text, View } from "react-native";
import { router, useFocusEffect, useLocalSearchParams } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { AttachmentList } from "../../components/AttachmentList";
import { PhotoButtons } from "../../components/PhotoButtons";
import { Button, Card, DetailHeader, ErrorState, Input, KV, Pill, Screen, SEV_NAME, SevBadge, Skeleton, SlaRingView } from "../../components/ui";
import { api } from "../../lib/api";
import { haptic } from "../../lib/haptics";
import { useLive } from "../../lib/live";
import { FadeIn } from "../../lib/motion";
import { useSession } from "../../lib/session";
import { slaRing } from "../../lib/sla";
import { sp, useTheme } from "../../lib/theme";
import type { Comment, Status, Ticket } from "../../lib/types";

const stamp = (s: string | null) => (s ? new Date(s).toLocaleString("en-GB", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit", second: "2-digit" }) : "–");

export default function TicketScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { s, c } = useTheme();
  const { me } = useSession();
  const insets = useSafeAreaInsets();
  const staff = me?.scope === "staff";
  const [t, setT] = useState<Ticket | null>(null);
  const [comments, setComments] = useState<Comment[]>([]);
  const [body, setBody] = useState("");
  const [internal, setInternal] = useState(false);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [photoKey, setPhotoKey] = useState(0);
  const [updated, setUpdated] = useState<Date | null>(null);
  const load = useCallback(async () => {
    try {
      const [tk, cs] = await Promise.all([api.ticket(id), api.comments(id)]);
      setT(tk);
      setComments(cs.data);
      setErr("");
      setUpdated(new Date());
    } catch (e) {
      setErr((e as Error).message);
    }
  }, [id]);
  useFocusEffect(useCallback(() => void load(), [load]));
  // New replies and status changes appear without pulling to refresh.
  useLive((e) => { if (e.subject.startsWith("ticket.") && e.data.id === id) void load(); });
  const run = (p: Promise<unknown>) => { setBusy(true); return p.then(() => { haptic.success(); return load(); }).catch((e) => { haptic.error(); setErr(e.message); }).finally(() => setBusy(false)); };

  const back = () => (router.canGoBack() ? router.back() : router.replace("/(tabs)/tickets"));
  if (!t) {
    return (
      <Screen>
        <View style={{ padding: sp.lg, gap: sp.md }}>
          <DetailHeader onBack={back} title="Ticket" />
          {err ? <ErrorState message={err} onRetry={load} /> : <><Skeleton h={170} r={12} /><Skeleton h={90} r={12} /></>}
        </View>
      </Screen>
    );
  }
  const move = (to: Status) => run(api.updateTicket(id, { status: to }, t.version));
  const r = slaRing(t);
  const first = t.first_response_at;
  return (
    <Screen updated={updated}>
      <KeyboardAvoidingView style={{ flex: 1 }} behavior={Platform.OS === "ios" ? "padding" : undefined}>
        <ScrollView contentContainerStyle={{ padding: sp.lg, paddingBottom: insets.bottom + sp.xl, gap: sp.lg }} keyboardShouldPersistTaps="handled" refreshControl={<RefreshControl refreshing={false} onRefresh={load} tintColor={c.gold} />}>
          <DetailHeader onBack={back} overline={`TKT ${t.id.slice(0, 8).toUpperCase()} · ${t.channel.toUpperCase()}`} title={t.subject} />
          <View style={[s.row, { gap: 8 }]}>
            <SevBadge sev={t.severity} />
            <Text style={[s.muted, { color: c.text2 }]}>{SEV_NAME[t.severity]}</Text>
            <Pill text={t.status.replace("_", " ")} tone={t.status === "closed" ? "neutral" : t.status === "resolved" ? "ok" : t.status === "in_progress" ? "info" : "gold"} />
          </View>

          <FadeIn>
            <Card style={{ gap: sp.md }}>
              <View style={{ flexDirection: "row", alignItems: "center", gap: sp.lg }}>
                <SlaRingView t={t} size={132} />
                <View style={{ flex: 1 }}>
                  <KV k="Respond" v={stamp(t.respond_due_at)} mono />
                  <KV k="Reply" v={first ? stamp(first) : t.respond_due_at && new Date(t.respond_due_at).getTime() < Date.now() ? "Overdue" : "Pending"} mono />
                  <KV k="Resolve" v={stamp(t.sla_due_at)} mono />
                  {t.resolved_at && <KV k="Resolved" v={stamp(t.resolved_at)} mono />}
                </View>
              </View>
              <View style={{ height: StyleSheet.hairlineWidth * 2, backgroundColor: c.line }} />
              <KV k="Opened" v={stamp(t.created_at)} mono />
              <KV k="Reporter" v={t.reporter} mono />
              <KV k="Assignee" v={t.assignee ?? "Unassigned"} mono />
              {r.state === "bad" && !t.resolved_at && <Text style={[s.error, { marginTop: 4 }]}>This ticket is past its resolution deadline.</Text>}
            </Card>
          </FadeIn>

          {!!t.description && (
            <Card>
              <Text style={s.overline}>Description</Text>
              <Text style={s.text}>{t.description}</Text>
            </Card>
          )}
          {!!err && <ErrorState message={err} onRetry={load} />}

          <View style={{ gap: sp.sm }}>
            {staff && t.status === "open" && <Button icon="wrench" title="Start work" disabled={busy} onPress={() => move("in_progress")} />}
            {staff && (t.status === "open" || t.status === "in_progress") && <Button primary icon="check" title="Mark resolved" disabled={busy} onPress={() => move("resolved")} />}
            {t.status === "resolved" && <Button primary icon="check" title={staff ? "Close ticket" : "Confirm resolved"} disabled={busy} onPress={() => move("closed")} />}
            {t.status === "resolved" && <Button title="Reopen" icon="sync" disabled={busy} onPress={() => move("in_progress")} />}
          </View>

          <AttachmentList entityType="ticket" entityId={id} refreshKey={photoKey} />
          {t.status !== "closed" && <PhotoButtons entityType="ticket" entityId={id} onUploaded={() => setPhotoKey((k) => k + 1)} />}

          <View style={{ gap: sp.sm }}>
            <View style={{ flexDirection: "row", alignItems: "center", gap: sp.sm }}>
              <Text style={[s.overline, { color: c.goldText }]} accessibilityRole="header">Conversation</Text>
              <View style={{ flex: 1, height: StyleSheet.hairlineWidth * 2, backgroundColor: c.line }} />
              <Text style={[s.mono, { color: c.muted }]}>{String(comments.length).padStart(2, "0")}</Text>
            </View>
            {comments.map((m, i) => (
              <View key={m.id} style={{ flexDirection: "row", gap: sp.md }}>
                <View style={{ alignItems: "center", width: 12 }}>
                  <View style={{ width: 10, height: 10, borderRadius: 5, marginTop: 6, backgroundColor: m.internal ? c.warn : c.gold }} />
                  {i < comments.length - 1 && <View style={{ flex: 1, width: StyleSheet.hairlineWidth * 2, backgroundColor: c.line, marginTop: 2 }} />}
                </View>
                <View style={[{ flex: 1, backgroundColor: c.surface, borderRadius: 8, padding: sp.md, gap: 4, borderWidth: StyleSheet.hairlineWidth * 2, borderColor: m.internal ? c.warn : c.line }, m.internal && { borderStyle: "dashed" }]}>
                  <View style={{ flexDirection: "row", justifyContent: "space-between", alignItems: "center", gap: 8 }}>
                    <Text style={[s.mono, { color: c.text2, flexShrink: 1 }]} numberOfLines={1}>{m.author}</Text>
                    {m.internal && <Pill text="Internal" tone="warn" icon="lock" />}
                  </View>
                  <Text style={s.text}>{m.body}</Text>
                  <Text style={[s.mono, { color: c.muted, fontSize: 11 }]}>{stamp(m.created_at)}</Text>
                </View>
              </View>
            ))}
            {!comments.length && <Text style={s.muted}>No replies yet.</Text>}
          </View>

          {t.status !== "closed" && (
            <Card style={{ gap: sp.sm }}>
              <Input label="Reply" multiline placeholder="Write a reply" value={body} onChangeText={setBody} />
              {staff && (
                <View style={[s.row, { justifyContent: "space-between" }]}>
                  <View style={{ flexDirection: "row", gap: 8, alignItems: "center" }}>
                    <Switch value={internal} onValueChange={(v) => { haptic.select(); setInternal(v); }} trackColor={{ true: c.goldFill, false: c.line }} thumbColor={c.fg} accessibilityLabel="Internal note" />
                    <Text style={s.text}>Internal note</Text>
                  </View>
                </View>
              )}
              <Button primary icon="send" title="Send" disabled={!body.trim() || busy} onPress={() => run(api.addComment(id, body.trim(), internal, `c-${Date.now()}-${Math.random().toString(36).slice(2)}`).then(() => setBody("")))} />
            </Card>
          )}

          {!staff && (t.status === "resolved" || t.status === "closed") && (
            <Card>
              <Text style={s.overline}>{t.rating ? `You rated this ${t.rating}/5` : "Rate this support"}</Text>
              {!t.rating && (
                <View style={[s.row, { gap: 8 }]}>
                  {[1, 2, 3, 4, 5].map((n) => <Button key={n} small title={String(n)} onPress={() => run(api.rate(id, n))} style={{ minWidth: 52 }} />)}
                </View>
              )}
            </Card>
          )}
        </ScrollView>
      </KeyboardAvoidingView>
    </Screen>
  );
}
