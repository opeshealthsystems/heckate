import { useCallback, useEffect, useMemo, useState } from "react";
import { Platform } from "react-native";
import { router, Stack, useFocusEffect, useLocalSearchParams } from "expo-router";
import { ErrorBanner, SyncBanner } from "../../../components/Banner";
import { EmptyState, Loading } from "../../../components/EmptyState";
import { HeaderButton } from "../../../components/Header";
import { Row, Section } from "../../../components/List";
import { Screen } from "../../../components/Screen";
import { Segmented } from "../../../components/Segmented";
import { Field } from "../../../components/TextField";
import { TicketRow } from "../../../components/TicketRow";
import { api } from "../../../lib/api";
import { atRisk, isOpen, queueFor } from "../../../lib/home-core";
import { useLive } from "../../../lib/live";
import { useNow } from "../../../lib/now";
import { useSession } from "../../../lib/session";
import type { Ticket } from "../../../lib/types";

const FILTERS = ["Open", "Resolved", "All"] as const;
type Filter = (typeof FILTERS)[number];

export default function Tickets() {
  const { me, syncDrafts } = useSession();
  const staff = me?.scope === "staff";
  const now = useNow();
  const [rows, setRows] = useState<Ticket[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [filter, setFilter] = useState<Filter>("Open");
  const [q, setQ] = useState("");
  const params = useLocalSearchParams<{ filter?: string; scope?: string }>();
  useEffect(() => { if (params.filter === "open") setFilter("Open"); }, [params.filter]);
  const scope = params.scope as "mine" | "risk" | "incidents" | undefined;
  const SCOPE_LABEL = { mine: "Showing tickets assigned to you", risk: "Showing tickets at risk", incidents: "Showing P1 and P2 tickets" } as const;
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
    setLoaded(true);
  }, [syncDrafts]);
  useFocusEffect(useCallback(() => void load(), [load]));
  useLive((e) => { if (e.subject.startsWith("ticket.")) void load(); });

  const shown = useMemo(() => {
    const byFilter = filter === "All" ? rows : filter === "Open" ? rows.filter(isOpen) : rows.filter((t) => t.status === "resolved" || t.status === "closed");
    const scoped = scope === "mine" && me ? queueFor(byFilter, "Mine", me.sub) : scope === "risk" ? byFilter.filter((t) => atRisk([t], now) === 1) : scope === "incidents" ? byFilter.filter((t) => t.severity === "P1" || t.severity === "P2") : byFilter;
    const n = q.trim().toLowerCase();
    return n ? scoped.filter((t) => `${t.subject} ${t.assignee ?? ""} ${t.reporter} ${t.severity}`.toLowerCase().includes(n)) : scoped;
  }, [rows, filter, q, scope, me, now]);

  if (me?.scope === "other") {
    return <Screen><EmptyState icon="lock" title="Not available in the app" text="Your role does not use the mobile app. Please use the web portal." /></Screen>;
  }
  const newLabel = staff ? "New ticket" : "Report an incident";
  return (
    <Screen webTitle="Tickets" refreshing={busy && loaded} onRefresh={load}>
      <Stack.Screen
        options={{
          title: "Tickets",
          headerRight: () => <HeaderButton icon="plus" label={newLabel} onPress={() => router.push("/report")} />,
          headerSearchBarOptions: { placeholder: "Search tickets", autoCapitalize: "none", hideWhenScrolling: false, onChangeText: (e) => setQ(e.nativeEvent.text) },
        }}
      />
      <SyncBanner />
      {!!err && <ErrorBanner message={err} />}
      {Platform.OS === "web" && <Section><Field placeholder="Search tickets" value={q} onChangeText={setQ} autoCapitalize="none" returnKeyType="search" /></Section>}
      <Segmented options={FILTERS} value={filter} onChange={setFilter} />
      {!!scope && <Section><Row title={SCOPE_LABEL[scope]} action titleLines={1} value="Show all" onPress={() => router.setParams({ scope: "", filter: "" })} /></Section>}
      {!loaded && busy ? <Loading /> : shown.length > 0 ? (
        <Section footer={`${shown.length} ticket${shown.length === 1 ? "" : "s"}`}>
          {shown.map((t) => <TicketRow key={t.id} ticket={t} now={now} />)}
        </Section>
      ) : loaded && !err ? (
        q.trim() ? <EmptyState icon="search" title="No results" text={`No ticket matches “${q.trim()}”.`} /> : (
          <EmptyState
            icon="inbox"
            title={filter === "Open" ? "No open tickets" : "No tickets"}
            text={filter === "Open" ? "Everything is quiet. Pull down to refresh, or report a problem." : "Nothing in this view yet."}
            action={filter !== "Resolved" ? { title: newLabel, primary: true, onPress: () => router.push("/report") } : undefined}
          />
        )
      ) : null}
    </Screen>
  );
}
