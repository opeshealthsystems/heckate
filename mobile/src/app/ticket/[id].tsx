import { useCallback, useState } from "react";
import { ScrollView, Switch, Text, TextInput, View } from "react-native";
import { useFocusEffect, useLocalSearchParams } from "expo-router";
import { Button, TicketPills } from "../../components/ui";
import { AttachmentList } from "../../components/AttachmentList";
import { PhotoButtons } from "../../components/PhotoButtons";
import { api } from "../../lib/api";
import { useLive } from "../../lib/live";
import { useSession } from "../../lib/session";
import { useTheme } from "../../lib/theme";
import type { Comment, Status, Ticket } from "../../lib/types";

export default function TicketScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { s, c } = useTheme();
  const { me } = useSession();
  const staff = me?.scope === "staff";
  const [t, setT] = useState<Ticket | null>(null);
  const [comments, setComments] = useState<Comment[]>([]);
  const [body, setBody] = useState("");
  const [internal, setInternal] = useState(false);
  const [err, setErr] = useState("");
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
  const run = (p: Promise<unknown>) => p.then(load).catch((e) => setErr(e.message));
  if (!t) return <View style={s.pad}>{err ? <Text style={s.error}>{err}</Text> : <Text style={s.muted}>Loading…</Text>}</View>;
  const move = (to: Status) => run(api.updateTicket(id, { status: to }, t.version));
  return (
    <ScrollView style={s.screen} contentContainerStyle={s.pad} keyboardShouldPersistTaps="handled">
      <Text style={s.h1}>{t.subject}</Text>
      <TicketPills t={t} />
      {!!t.description && <View style={s.card}><Text style={s.text}>{t.description}</Text></View>}
      {!!err && <Text style={s.error}>{err}</Text>}
      <View style={s.row}>
        {staff && t.status === "open" && <Button title="Start work" onPress={() => move("in_progress")} />}
        {staff && (t.status === "open" || t.status === "in_progress") && <Button primary title="Mark resolved" onPress={() => move("resolved")} />}
        {t.status === "resolved" && <Button primary title={staff ? "Close" : "Confirm resolved"} onPress={() => move("closed")} />}
        {t.status === "resolved" && <Button title="Reopen" onPress={() => move("in_progress")} />}
      </View>
      <AttachmentList entityType="ticket" entityId={id} refreshKey={photoKey} />
      {t.status !== "closed" && <PhotoButtons entityType="ticket" entityId={id} onUploaded={() => setPhotoKey((k) => k + 1)} />}
      <Text style={s.label}>Conversation</Text>
      {comments.map((c) => (
        <View key={c.id} style={[s.card, c.internal && { borderStyle: "dashed" }]}>
          <Text style={s.muted}>{c.author} · {new Date(c.created_at).toLocaleString()}{c.internal ? " · internal" : ""}</Text>
          <Text style={s.text}>{c.body}</Text>
        </View>
      ))}
      {!comments.length && <Text style={s.muted}>No replies yet.</Text>}
      {t.status !== "closed" && (
        <>
          <TextInput style={[s.input, { minHeight: 80, textAlignVertical: "top" }]} multiline placeholder="Write a reply" placeholderTextColor={c.muted} value={body} onChangeText={setBody} />
          {staff && (
            <View style={s.row}>
              <Switch value={internal} onValueChange={setInternal} />
              <Text style={s.text}>Internal note</Text>
            </View>
          )}
          <Button primary title="Send" disabled={!body.trim()} onPress={() => run(api.addComment(id, body.trim(), internal, `c-${Date.now()}-${Math.random().toString(36).slice(2)}`).then(() => setBody("")))} />
        </>
      )}
      {!staff && (t.status === "resolved" || t.status === "closed") && (
        <View style={s.card}>
          <Text style={s.label}>{t.rating ? `You rated this ${t.rating}/5` : "Rate this support"}</Text>
          {!t.rating && (
            <View style={s.row}>
              {[1, 2, 3, 4, 5].map((n) => <Button key={n} title={String(n)} onPress={() => run(api.rate(id, n))} />)}
            </View>
          )}
        </View>
      )}
    </ScrollView>
  );
}
