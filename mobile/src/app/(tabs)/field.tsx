import { useCallback, useState } from "react";
import { FlatList, Pressable, RefreshControl, Text, View } from "react-native";
import { router, useFocusEffect } from "expo-router";
import { Pill } from "../../components/ui";
import { api } from "../../lib/api";
import { useLive } from "../../lib/live";
import { useSession } from "../../lib/session";
import { useTheme } from "../../lib/theme";
import type { Intervention } from "../../lib/types";

export default function Field() {
  const { s, c } = useTheme();
  const { pendingActions, syncDrafts } = useSession();
  const [rows, setRows] = useState<Intervention[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const load = useCallback(async () => {
    setBusy(true);
    try {
      await syncDrafts();
      setRows((await api.interventions()).data);
      setErr("");
    } catch (e) {
      setErr((e as Error).message);
    }
    setBusy(false);
  }, [syncDrafts]);
  useFocusEffect(useCallback(() => void load(), [load]));
  useLive((e) => { if (e.subject.startsWith("intervention.")) void load(); });
  return (
    <FlatList
      style={s.screen}
      contentContainerStyle={s.pad}
      data={rows}
      keyExtractor={(i) => i.id}
      refreshControl={<RefreshControl refreshing={busy} onRefresh={load} />}
      ListHeaderComponent={
        <View style={{ gap: 8 }}>
          {pendingActions > 0 && <View style={s.card}><Text style={s.text}>{pendingActions} field update(s) saved on this phone, waiting for a connection. They keep their real times.</Text></View>}
          {!!err && <View style={[s.card, { borderColor: c.bad }]}><Text style={s.error}>Could not load: {err}</Text><Text style={s.muted}>Pull down to try again.</Text></View>}
        </View>
      }
      ListEmptyComponent={busy || err ? undefined : <View style={s.card}><Text style={s.text}>No site visits assigned to you.</Text><Text style={s.muted}>Pull down to refresh.</Text></View>}
      renderItem={({ item: i }) => (
        <Pressable accessibilityRole="button" onPress={() => router.push(`/intervention/${i.id}`)} style={s.card}>
          <Text style={[s.text, s.semibold]}>{i.summary || "Site visit"}</Text>
          <View style={s.row}><Pill text={i.status.replace("_", " ")} tone={i.status === "completed" ? "ok" : undefined} /></View>
          <Text style={s.muted}>{i.scheduled_for ? new Date(i.scheduled_for).toLocaleString() : "Not scheduled"}</Text>
        </Pressable>
      )}
    />
  );
}
