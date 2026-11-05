import { useCallback, useState } from "react";
import { Stack, useFocusEffect } from "expo-router";
import { Banner, ErrorBanner } from "../components/Banner";
import { EmptyState, Loading } from "../components/EmptyState";
import { useGate } from "../components/Header";
import { Row, Section } from "../components/List";
import { Screen } from "../components/Screen";
import { api, listOf } from "../lib/api";
import { haptic } from "../lib/haptics";
import { accessFor, type Claim, type Leave, type Sheet } from "../lib/home-core";
import { confirmDestructive } from "../lib/dialogs";
import { newKey } from "../lib/queue-core";
import { useSession } from "../lib/session";

const day = (s: string) => new Date(s).toLocaleDateString("en-GB", { day: "2-digit", month: "short" });
const xaf = (n: number) => `${new Intl.NumberFormat("fr-FR").format(n)} XAF`;
const KIND: Record<string, string> = { annual: "Annual leave", sick: "Sick leave", unpaid: "Unpaid leave", other: "Other leave" };

type Item = { kind: "leave" | "expenses" | "timesheets"; id: string; who: string; title: string; detail: string; reject: boolean };

/** Leave, expense and timesheet approvals, decided on the phone. Nobody sees or decides their own requests. */
export default function Approvals() {
  const gate = useGate();
  const { me } = useSession();
  const [items, setItems] = useState<Item[]>([]);
  const [err, setErr] = useState("");
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState("");
  const [loaded, setLoaded] = useState(false);

  const load = useCallback(async () => {
    if (!me) return;
    const a = accessFor(me);
    try {
      const [leave, claims, sheets] = await Promise.all([
        a.reviewer ? listOf<Leave & { end_date?: string }>("/v1/leave?all=1") : Promise.resolve([]),
        a.reviewer ? listOf<Claim & { category?: string }>("/v1/expenses?all=1") : Promise.resolve([]),
        a.approver ? listOf<Sheet & { note?: string }>("/v1/timesheets?all=1") : Promise.resolve([]),
      ]);
      setItems([
        ...leave.filter((l) => l.status === "pending" && l.actor !== me.sub).map((l) => ({
          kind: "leave" as const, id: l.id, who: l.actor, reject: true,
          title: `${KIND[l.kind] ?? "Leave"} · ${l.days} day${l.days === 1 ? "" : "s"}`,
          detail: `${l.actor} · from ${day(l.start_date)}${l.end_date ? ` to ${day(l.end_date)}` : ""}`,
        })),
        ...claims.filter((c) => c.status === "submitted" && c.actor !== me.sub).map((c) => ({
          kind: "expenses" as const, id: c.id, who: c.actor, reject: true,
          title: `Expense · ${xaf(c.amount)}`, detail: `${c.actor} · ${c.category ? `${c.category}: ` : ""}${c.description}`,
        })),
        ...sheets.filter((s) => s.status === "submitted" && s.actor !== me.sub).map((s) => ({
          kind: "timesheets" as const, id: s.id, who: s.actor, reject: false,
          title: `Timesheet · ${s.hours} h`, detail: `${s.actor} · ${day(s.work_date)}${s.note ? ` · ${s.note}` : ""}`,
        })),
      ]);
      setErr("");
    } catch (e) {
      setErr((e as Error).message);
    }
    setLoaded(true);
  }, [me]);
  useFocusEffect(useCallback(() => void load(), [load]));
  if (gate) return gate;

  const decide = async (it: Item, action: "approve" | "reject") => {
    setBusy(it.id);
    setMsg("");
    try {
      await api.decide(it.kind, it.id, action, newKey("decide"));
      haptic.success();
      setMsg(`${it.title} for ${it.who} ${action === "approve" ? "approved" : "rejected"}.`);
      await load();
    } catch (e) {
      haptic.error();
      setErr((e as Error).message);
    }
    setBusy("");
  };

  return (
    <Screen refreshing={false} onRefresh={load}>
      <Stack.Screen options={{ title: "Approvals" }} />
      {!!err && <ErrorBanner message={err} />}
      {!!msg && <Banner icon="checkCircle">{msg}</Banner>}
      {!loaded ? <Loading /> : items.length === 0 ? (
        <EmptyState icon="checkCircle" title="Nothing to approve" text="New leave requests, expense claims and timesheets from your team appear here." />
      ) : items.map((it) => (
        <Section key={it.kind + it.id}>
          <Row title={it.title} subtitle={it.detail} titleLines={2} />
          <Row icon="checkCircle" title="Approve" action disabled={busy === it.id} onPress={() => void decide(it, "approve")} />
          {it.reject && (
            <Row icon="close" title="Reject" destructive disabled={busy === it.id}
              onPress={() => confirmDestructive(`Reject this request from ${it.who}?`, "They will see that it was rejected.", "Reject", () => void decide(it, "reject"))} />
          )}
        </Section>
      ))}
    </Screen>
  );
}
