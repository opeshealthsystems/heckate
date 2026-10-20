import { useCallback, useState } from "react";
import { Pressable, RefreshControl, ScrollView, Text, View } from "react-native";
import { router, useFocusEffect } from "expo-router";
import { Empty, Kpi, RowItem, Section } from "../../components/dash";
import { Pill, TicketPills } from "../../components/ui";
import { api, listOf } from "../../lib/api";
import { accessFor, approvalsFor, atRisk, clientActions, clientKpis, hoursThisWeek, isOpen, queueFor, type Claim, type ContractRow, type FormRow, type InvoiceRow, type Leave, type QuoteRow, type QueueTab, type Sheet } from "../../lib/home-core";
import { useSession } from "../../lib/session";
import { useTheme } from "../../lib/theme";
import type { Client, SupportReport, Ticket } from "../../lib/types";

const xaf = (n: number) => `${new Intl.NumberFormat("fr-FR").format(n)} XAF`;
const day = (s: string) => new Date(s).toLocaleDateString();
const roleLabel = (r: string) => (r === "ceo" ? "CEO" : r.charAt(0).toUpperCase() + r.slice(1).replace(/_/g, " "));
const KIND: Record<string, string> = { quote: "Quotation", invoice: "Invoice", ticket: "Ticket", form: "Form", contract: "Contract" };

export default function Home() {
  const { s, c } = useTheme();
  const { me } = useSession();
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [report, setReport] = useState<SupportReport | null>(null);
  const [clients, setClients] = useState<Client[]>([]);
  const [leave, setLeave] = useState<Leave[]>([]);
  const [claims, setClaims] = useState<Claim[]>([]);
  const [sheets, setSheets] = useState<Sheet[]>([]);
  const [mine, setMine] = useState<Sheet[]>([]);
  const [quotes, setQuotes] = useState<QuoteRow[]>([]);
  const [invoices, setInvoices] = useState<InvoiceRow[]>([]);
  const [forms, setForms] = useState<FormRow[]>([]);
  const [contracts, setContracts] = useState<ContractRow[]>([]);
  const [tab, setTab] = useState<QueueTab>("Mine");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const access = me ? accessFor(me) : null;
  const staff = me?.scope === "staff";

  const load = useCallback(async () => {
    if (!me) return;
    const a = accessFor(me);
    if (!a.home) return;
    setBusy(true);
    try {
      const tk = a.tickets ? (await api.tickets()).data : [];
      setTickets(tk);
      if (me.scope === "staff" && !tk.some((t) => isOpen(t) && t.assignee === me.sub)) setTab("Team");
      setErr("");
      // Each list is read only when this role may read it; a refusal shows as empty.
      const jobs: Promise<unknown>[] = [];
      if (a.report) jobs.push(api.report().then(setReport).catch(() => {}));
      if (a.clientsLookup) jobs.push(api.clients().then((r) => setClients(r.data)).catch(() => {}));
      if (me.scope === "staff") {
        if (a.reviewer) jobs.push(listOf<Leave>("/v1/leave?all=1").then(setLeave), listOf<Claim>("/v1/expenses?all=1").then(setClaims));
        if (a.approver) jobs.push(listOf<Sheet>("/v1/timesheets?all=1").then(setSheets));
        if (a.timeUser) jobs.push(listOf<Sheet>("/v1/timesheets").then(setMine));
      } else {
        if (a.money) jobs.push(listOf<QuoteRow>("/v1/quotes").then(setQuotes), listOf<InvoiceRow>("/v1/invoices").then(setInvoices));
        if (a.forms) jobs.push(listOf<FormRow>("/v1/form-requests").then(setForms));
        if (a.contracts) jobs.push(listOf<ContractRow>("/v1/contracts").then(setContracts));
      }
      await Promise.all(jobs);
    } catch (e) {
      setErr((e as Error).message);
    }
    setBusy(false);
  }, [me]);
  useFocusEffect(useCallback(() => void load(), [load]));

  if (!me || !access?.home) return <View style={s.pad}><Text style={s.text}>Your role does not use the mobile app. Please use the web portal.</Text></View>;

  const today = new Date().toLocaleDateString("en-GB", { weekday: "long", day: "numeric", month: "long" });
  const open = tickets.filter(isOpen);
  const clientName = new Map(clients.map((x) => [x.tenant_id, x.legal_name]));
  const header = (title: string, meta: string) => (
    <View style={{ gap: 2 }}>
      <Text style={s.muted}>{meta}</Text>
      <Text style={s.h1} accessibilityRole="header">{title}</Text>
    </View>
  );
  const errBox = !!err && <View style={[s.card, { borderColor: c.bad }]}><Text style={s.error}>Could not load: {err}</Text><Text style={s.muted}>Pull down to try again.</Text></View>;

  if (staff) {
    const queue = queueFor(open, tab, me.sub);
    const risk = atRisk(open);
    const approvals = approvalsFor(me.sub, leave, claims, sheets, day, xaf);
    return (
      <ScrollView style={s.screen} contentContainerStyle={s.pad} refreshControl={<RefreshControl refreshing={busy} onRefresh={load} />}>
        {header("My day", `${today} · ${me.roles.map(roleLabel).join(", ")}`)}
        {errBox}
        <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
          <Kpi label="Assigned to me" value={open.filter((t) => t.assignee === me.sub).length} />
          <Kpi label="SLA at risk" value={risk} tone={risk ? "bad" : undefined} sub={report ? `${report.sla_breached} breached in total` : undefined} />
          {(access.reviewer || access.approver) && <Kpi label="Approvals waiting" value={approvals.length} />}
          {access.timeUser && <Kpi label="Hours this week" value={hoursThisWeek(mine).toFixed(1)} />}
          {report && <Kpi label="Open tickets" value={report.open} sub={`${report.p1_open} P1`} />}
          {report && <Kpi label="Avg rating" value={report.avg_rating == null ? "–" : report.avg_rating.toFixed(1)} sub="out of 5" />}
        </View>
        <Section title="Ticket queue">
          <View style={[s.row, { padding: 10, borderBottomColor: c.line, borderBottomWidth: 1 }]}>
            {(["Mine", "Team", "Unassigned"] as QueueTab[]).map((x) => (
              <Pressable key={x} accessibilityRole="tab" accessibilityState={{ selected: tab === x }} onPress={() => setTab(x)} style={{ paddingHorizontal: 12, paddingVertical: 8, borderRadius: 999, backgroundColor: tab === x ? c.brand : "transparent", borderWidth: 1, borderColor: tab === x ? c.brand : c.line }}>
                <Text style={{ color: tab === x ? c.brandFg : c.brand, fontSize: 14, fontFamily: "IBMPlexSans_600SemiBold" }}>{x}</Text>
              </Pressable>
            ))}
          </View>
          {queue.slice(0, 8).map((t, i, a) => (
            <RowItem key={t.id} last={i === a.length - 1} title={t.subject} sub={clientName.get(t.tenant_id)} onPress={() => router.push(`/ticket/${t.id}`)} right={<TicketPills t={t} />} />
          ))}
          {!queue.length && <Empty text={busy ? "Loading…" : "Nothing here."} />}
        </Section>
        {(access.reviewer || access.approver) && (
          <Section title="Approvals">
            {approvals.slice(0, 6).map((a, i, all) => <RowItem key={a.key} last={i === all.length - 1} title={a.text} />)}
            {!approvals.length && <Empty text="Nothing waiting for you." />}
            {approvals.length > 0 && <Text style={[s.muted, { padding: 14, paddingTop: 0 }]}>Review these in the web portal.</Text>}
          </Section>
        )}
      </ScrollView>
    );
  }

  const k = clientKpis(tickets, invoices);
  const actions = clientActions(access, { quotes, invoices, tickets, forms, contracts });
  return (
    <ScrollView style={s.screen} contentContainerStyle={s.pad} refreshControl={<RefreshControl refreshing={busy} onRefresh={load} />}>
      {header("Overview", today)}
      {errBox}
      <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
        <Kpi label="Open tickets" value={k.open} sub={k.incidents ? `${k.incidents} incident${k.incidents === 1 ? "" : "s"} in progress` : "No incidents"} />
        <Kpi label="SLA this month" value={k.sla === null ? "–" : `${k.sla}%`} tone={k.sla === 100 ? "ok" : undefined} sub={k.sla === null ? "Nothing resolved yet" : k.sla === 100 ? "All targets met" : `${k.resolved - k.met} missed`} />
        {access.money && <Kpi label="Balance due" value={xaf(k.balance)} sub={k.owing ? `${k.owing} invoice${k.owing === 1 ? "" : "s"}` : "Nothing owed"} />}
      </View>
      <Section title="Awaiting your action">
        {actions.slice(0, 12).map((a, i, all) => (
          <RowItem key={a.key} last={i === all.length - 1} title={a.title} sub={a.sub}
            onPress={a.kind === "ticket" ? () => router.push(`/ticket/${a.key.slice(1)}`) : undefined}
            right={<Pill text={KIND[a.kind]} tone={a.kind === "invoice" ? "warn" : undefined} />} />
        ))}
        {!actions.length && <Empty text={busy ? "Loading…" : "Nothing is waiting for you."} />}
        {actions.some((a) => a.kind !== "ticket") && <Text style={[s.muted, { padding: 14, paddingTop: 0 }]}>Approve, pay or sign in the web portal.</Text>}
      </Section>
    </ScrollView>
  );
}
