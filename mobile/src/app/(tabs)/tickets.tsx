import { useCallback, useMemo, useState } from "react";
import { FlatList, RefreshControl, Text, View } from "react-native";
import { router, useFocusEffect } from "expo-router";
import { EmptyState, ErrorState, Notice, PressCard, Screen, ScreenHeader, Segmented, SevBadge, SkeletonRows, SlaChip, SlaRingView, Pill, sevColor } from "../../components/ui";
import { api } from "../../lib/api";
import { isOpen } from "../../lib/home-core";
import { useLive } from "../../lib/live";
import { FadeIn } from "../../lib/motion";
import { useSession } from "../../lib/session";
import { sp, useTheme } from "../../lib/theme";
import type { Ticket } from "../../lib/types";

const FILTERS = ["Open", "Resolved", "All"] as const;
type Filter = (typeof FILTERS)[number];
const shortId = (id: string) => id.slice(0, 8).toUpperCase();
const fmt = (s: string) => new Date(s).toLocaleString("en-GB", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" });

export default function Tickets() {
  const { s, c } = useTheme();
  const { me, pendingDrafts, syncDrafts } = useSession();
  const staff = me?.scope === "staff";
  const [rows, setRows] = useState<Ticket[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [filter, setFilter] = useState<Filter>("Open");
  const [updated, setUpdated] = useState<Date | null>(null);
  const load = useCallback(async () => {
    setBusy(true);
    try {
      await syncDrafts();
      setRows((await api.tickets()).data);
      setErr("");
      setUpdated(new Date());
    } catch (e) {
      setErr((e as Error).message);
    }
    setBusy(false);
    setLoaded(true);
  }, [syncDrafts]);
  useFocusEffect(useCallback(() => void load(), [load]));
  useLive((e) => { if (e.subject.startsWith("ticket.")) void load(); });

  const counts = useMemo(() => ({ Open: rows.filter(isOpen).length, Resolved: rows.filter((t) => t.status === "resolved" || t.status === "closed").length, All: rows.length }), [rows]);
  const shown = useMemo(() => (filter === "All" ? rows : filter === "Open" ? rows.filter(isOpen) : rows.filter((t) => t.status === "resolved" || t.status === "closed")), [rows, filter]);

  if (me?.scope === "other") {
    return <Screen><View style={s.pad}><EmptyState icon="lock" title="Not available in the app" text="Your role does not use the mobile app. Please use the web portal." /></View></Screen>;
  }
  return (
    <Screen updated={updated}>
      <FlatList
        contentContainerStyle={{ padding: sp.lg, gap: sp.sm }}
        data={loaded ? shown : []}
        keyExtractor={(t) => t.id}
        refreshControl={<RefreshControl refreshing={busy && loaded} onRefresh={load} tintColor={c.gold} colors={[c.gold]} progressBackgroundColor={c.surface} />}
        ListHeaderComponent={
          <View style={{ gap: sp.md, marginBottom: sp.sm }}>
            <ScreenHeader overline={staff ? "Support operations" : "Your support requests"} title={staff ? "Queue" : "Tickets"} />
            <Segmented options={FILTERS} value={filter} onChange={setFilter} counts={loaded ? counts : undefined} />
            {pendingDrafts > 0 && <Notice icon="sync">{pendingDrafts} report{pendingDrafts === 1 ? "" : "s"} saved on this phone, waiting to send. They go out automatically when you are back online.</Notice>}
            {!!err && <ErrorState message={err} onRetry={load} />}
            {!loaded && busy && <SkeletonRows n={5} />}
          </View>
        }
        ListEmptyComponent={loaded && !err ? (
          <EmptyState icon={filter === "Open" ? "shieldCheck" : "inbox"} title={filter === "Open" ? "All quiet" : "Nothing here"} text={filter === "Open" ? "No open tickets. Pull down to refresh." : "No tickets in this view yet."} />
        ) : undefined}
        renderItem={({ item: t, index }) => (
          <FadeIn index={index}>
            <PressCard rail={sevColor(c, t.severity)} onPress={() => router.push(`/ticket/${t.id}`)} label={`${t.severity} ${t.subject}`}>
              <View style={{ flexDirection: "row", alignItems: "center", gap: sp.md }}>
                <View style={{ flex: 1, gap: 6 }}>
                  <View style={{ flexDirection: "row", alignItems: "center", gap: 8 }}>
                    <SevBadge sev={t.severity} size={22} />
                    <Text style={[s.mono, { color: c.goldText }]}>{shortId(t.id)}</Text>
                    <Pill text={t.status.replace("_", " ")} tone={t.status === "closed" ? "neutral" : t.status === "resolved" ? "ok" : t.status === "in_progress" ? "info" : "gold"} />
                  </View>
                  <Text style={[s.text, s.semibold, { fontSize: 16 }]} numberOfLines={2}>{t.subject}</Text>
                  <SlaChip t={t} />
                  <Text style={[s.mono, { color: c.muted, fontSize: 11 }]} numberOfLines={1}>{fmt(t.created_at)}{t.assignee ? ` · ${t.assignee}` : " · unassigned"}</Text>
                </View>
                {!t.resolved_at && t.sla_due_at && <SlaRingView t={t} size={50} />}
              </View>
            </PressCard>
          </FadeIn>
        )}
      />
    </Screen>
  );
}
