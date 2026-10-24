import { useCallback, useState } from "react";
import { router, Stack, useFocusEffect } from "expo-router";
import { ErrorBanner, SyncBanner } from "../../../components/Banner";
import { EmptyState, Loading } from "../../../components/EmptyState";
import { Row, Section } from "../../../components/List";
import { Screen } from "../../../components/Screen";
import { api } from "../../../lib/api";
import { useLive } from "../../../lib/live";
import { useSession } from "../../../lib/session";
import type { Intervention } from "../../../lib/types";

const when = (s: string | null) => (s ? new Date(s).toLocaleString("en-GB", { weekday: "short", day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" }) : "Not scheduled");
const clock = (s: string) => new Date(s).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" });

function VisitRow({ v }: { v: Intervention }) {
  return (
    <Row
      title={v.summary || "Site visit"}
      subtitle={v.status === "in_progress" && v.checkin_at ? `On site since ${clock(v.checkin_at)}` : v.status === "completed" && v.checkout_at ? `Completed ${when(v.checkout_at)}` : when(v.scheduled_for)}
      onPress={() => router.push(`/intervention/${v.id}`)}
    />
  );
}

export default function Field() {
  const { syncDrafts } = useSession();
  const [rows, setRows] = useState<Intervention[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
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
    setLoaded(true);
  }, [syncDrafts]);
  useFocusEffect(useCallback(() => void load(), [load]));
  useLive((e) => { if (e.subject.startsWith("intervention.")) void load(); });

  const onSite = rows.filter((r) => r.status === "in_progress");
  const scheduled = rows.filter((r) => r.status === "scheduled");
  const done = rows.filter((r) => r.status === "completed");
  return (
    <Screen webTitle="Field" refreshing={busy && loaded} onRefresh={load}>
      <Stack.Screen options={{ title: "Field" }} />
      <SyncBanner />
      {!!err && <ErrorBanner message={err} />}
      {!loaded && busy ? <Loading /> : (
        <>
          <Section header={onSite.length ? "On site" : undefined}>{onSite.map((v) => <VisitRow key={v.id} v={v} />)}</Section>
          <Section header={scheduled.length ? "Scheduled" : undefined}>{scheduled.map((v) => <VisitRow key={v.id} v={v} />)}</Section>
          <Section header={done.length ? "Completed" : undefined}>{done.map((v) => <VisitRow key={v.id} v={v} />)}</Section>
          {loaded && !err && rows.length === 0 && <EmptyState icon="location" title="No site visits" text="Nothing is assigned to you. Pull down to refresh." />}
        </>
      )}
    </Screen>
  );
}
