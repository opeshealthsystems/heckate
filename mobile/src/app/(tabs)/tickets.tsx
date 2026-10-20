import { useCallback, useState } from "react";
import { FlatList, Pressable, RefreshControl, Text, View } from "react-native";
import { router, useFocusEffect } from "expo-router";
import { TicketPills } from "../../components/ui";
import { api } from "../../lib/api";
import { useLive } from "../../lib/live";
import { useSession } from "../../lib/session";
import { useTheme } from "../../lib/theme";
import type { Ticket } from "../../lib/types";

export default function Tickets() {
  const { s, c } = useTheme();
  const { me, pendingDrafts, syncDrafts } = useSession();
  const [rows, setRows] = useState<Ticket[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const load = useCallback(async () => {
    setBusy(true);
    try {
      await syncDrafts();
      setRows((await api.tickets()).data);
      setErr("");
    } catch (e) {
      setErr((e as Error).message);
    }
    setBusy(false);
  }, [syncDrafts]);
  useFocusEffect(useCallback(() => void load(), [load]));
  useLive((e) => { if (e.subject.startsWith("ticket.")) void load(); });

  if (me?.scope === "other") return <View style={s.pad}><Text style={s.text}>Your role does not use the mobile app. Please use the web portal.</Text></View>;
  return (
    <FlatList
      style={s.screen}
      contentContainerStyle={s.pad}
      data={rows}
      keyExtractor={(t) => t.id}
      refreshControl={<RefreshControl refreshing={busy} onRefresh={load} />}
      ListHeaderComponent={
        <View style={{ gap: 8 }}>
          {pendingDrafts > 0 && <View style={s.card}><Text style={s.text}>{pendingDrafts} report(s) waiting to send. They will be sent when you are back online.</Text></View>}
          {!!err && <View style={[s.card, { borderColor: c.bad }]}><Text style={s.error}>Could not load: {err}</Text><Text style={s.muted}>Pull down to try again.</Text></View>}
        </View>
      }
      ListEmptyComponent={busy || err ? undefined : <View style={s.card}><Text style={s.text}>No tickets yet.</Text><Text style={s.muted}>Pull down to refresh.</Text></View>}
      renderItem={({ item: t }) => (
        <Pressable accessibilityRole="button" onPress={() => router.push(`/ticket/${t.id}`)} style={s.card}>
          <Text style={[s.text, s.semibold]}>{t.subject}</Text>
          <TicketPills t={t} />
          <Text style={s.muted}>{new Date(t.created_at).toLocaleString()}{t.assignee ? ` · ${t.assignee}` : ""}</Text>
        </Pressable>
      )}
    />
  );
}
