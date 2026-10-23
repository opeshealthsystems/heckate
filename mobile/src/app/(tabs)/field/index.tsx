import { useCallback, useMemo, useState } from "react";
import { FlatList, RefreshControl, Text, View } from "react-native";
import { router, useFocusEffect } from "expo-router";
import { Icon } from "../../components/Icon";
import { EmptyState, ErrorState, Notice, Pill, PressCard, Screen, ScreenHeader, SkeletonRows, visitColor, visitTone } from "../../components/ui";
import { api } from "../../lib/api";
import { useLive } from "../../lib/live";
import { FadeIn } from "../../lib/motion";
import { useSession } from "../../lib/session";
import { font, sp, useTheme } from "../../lib/theme";
import type { Intervention } from "../../lib/types";

const when = (s: string | null) => (s ? new Date(s).toLocaleString("en-GB", { weekday: "short", day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" }) : "Not scheduled");

export default function Field() {
  const { s, c } = useTheme();
  const { pendingActions, syncDrafts } = useSession();
  const [rows, setRows] = useState<Intervention[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [updated, setUpdated] = useState<Date | null>(null);
  const load = useCallback(async () => {
    setBusy(true);
    try {
      await syncDrafts();
      setRows((await api.interventions()).data);
      setErr("");
      setUpdated(new Date());
    } catch (e) {
      setErr((e as Error).message);
    }
    setBusy(false);
    setLoaded(true);
  }, [syncDrafts]);
  useFocusEffect(useCallback(() => void load(), [load]));
  useLive((e) => { if (e.subject.startsWith("intervention.")) void load(); });

  const n = useMemo(() => ({ scheduled: rows.filter((r) => r.status === "scheduled").length, onSite: rows.filter((r) => r.status === "in_progress").length, done: rows.filter((r) => r.status === "completed").length }), [rows]);
  return (
    <Screen updated={updated}>
      <FlatList
        contentContainerStyle={{ padding: sp.lg, gap: sp.sm }}
        data={loaded ? rows : []}
        keyExtractor={(i) => i.id}
        refreshControl={<RefreshControl refreshing={busy && loaded} onRefresh={load} tintColor={c.gold} colors={[c.gold]} progressBackgroundColor={c.surface} />}
        ListHeaderComponent={
          <View style={{ gap: sp.md, marginBottom: sp.sm }}>
            <ScreenHeader overline="Site visits" title="Field" />
            <View style={{ flexDirection: "row", gap: sp.sm }}>
              {([["Scheduled", n.scheduled, c.gold], ["On site", n.onSite, c.info], ["Completed", n.done, c.ok]] as const).map(([label, v, col]) => (
                <View key={label} accessible style={{ flex: 1, backgroundColor: c.surface, borderColor: c.line, borderWidth: 1, borderRadius: 8, padding: sp.md, gap: 2, overflow: "hidden" }}>
                  <View style={{ position: "absolute", left: 0, right: 0, top: 0, height: 2, backgroundColor: col }} />
                  <Text style={s.overline}>{label}</Text>
                  <Text style={{ color: c.fg, fontSize: 24, fontFamily: font.monoBold }}>{loaded ? v : "–"}</Text>
                </View>
              ))}
            </View>
            {pendingActions > 0 && <Notice icon="sync">{pendingActions} field update{pendingActions === 1 ? "" : "s"} saved on this phone, waiting for a connection. They keep their real times.</Notice>}
            {!!err && <ErrorState message={err} onRetry={load} />}
            {!loaded && busy && <SkeletonRows n={3} h={96} />}
          </View>
        }
        ListEmptyComponent={loaded && !err ? <EmptyState icon="wrench" title="No site visits" text="Nothing is assigned to you. Pull down to refresh." /> : undefined}
        renderItem={({ item: i, index }) => (
          <FadeIn index={index}>
            <PressCard rail={visitColor(c, i.status)} onPress={() => router.push(`/intervention/${i.id}`)} label={i.summary || "Site visit"}>
              <View style={{ flexDirection: "row", alignItems: "center", gap: 8, justifyContent: "space-between" }}>
                <Text style={[s.mono, { color: c.goldText }]}>VISIT {i.id.slice(0, 8).toUpperCase()}</Text>
                <Pill text={i.status.replace("_", " ")} tone={visitTone(i.status)} />
              </View>
              <Text style={[s.text, s.semibold, { fontSize: 16 }]} numberOfLines={2}>{i.summary || "Site visit"}</Text>
              <View style={{ flexDirection: "row", alignItems: "center", gap: 6 }}>
                <Icon name="calendar" size={14} color={c.muted} />
                <Text style={[s.mono, { color: c.text2 }]}>{when(i.scheduled_for)}</Text>
              </View>
              {i.checkin_at && i.status === "in_progress" && (
                <View style={{ flexDirection: "row", alignItems: "center", gap: 6 }}>
                  <Icon name="field" size={14} color={c.info} />
                  <Text style={[s.mono, { color: c.info }]}>On site since {new Date(i.checkin_at).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" })}</Text>
                </View>
              )}
            </PressCard>
          </FadeIn>
        )}
      />
    </Screen>
  );
}
