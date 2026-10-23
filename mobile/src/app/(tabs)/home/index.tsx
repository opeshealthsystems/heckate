import { useCallback, useState } from "react";
import { View } from "react-native";
import { router, Stack, useFocusEffect } from "expo-router";
import { ErrorBanner, SyncBanner } from "../../../components/Banner";
import { EmptyState, Loading } from "../../../components/EmptyState";
import { Row, Section } from "../../../components/List";
import { Screen } from "../../../components/Screen";
import { Segmented } from "../../../components/Segmented";
import { DocumentRow } from "../../../components/DocumentRow";
import { SeverityText, SlaText } from "../../../components/Status";
import { TicketRow } from "../../../components/TicketRow";
import { api, listOf } from "../../../lib/api";
import { accessFor, approvalsFor, atRisk, clientActions, clientKpis, hoursThisWeek, isOpen, latestDocuments, needsAttention, queueFor, type Claim, type ContractRow, type FormRow, type InvoiceRow, type Leave, type QuoteRow, type QueueTab, type Sheet } from "../../../lib/home-core";
import { useNow } from "../../../lib/now";
import { useSession } from "../../../lib/session";
import type { Client, DocumentEntry, SupportReport, Ticket } from "../../../lib/types";

const xaf = (n: number) => `${new Intl.NumberFormat("fr-FR").format(n)} XAF`;
const day = (s: string) => new Date(s).toLocaleDateString();
const KIND: Record<string, string> = { quote: "Quotation", invoice: "Invoice", ticket: "Ticket", form: "Form", contract: "Contract" };
const QUEUE_LABEL: Record<QueueTab, string> = { Mine: "Mine", Team: "Team", Unassigned: "Unassigned" };

export default function Home() {
  const { me } = useSession();
  const now = useNow();
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
  const [docs, setDocs] = useState<DocumentEntry[]>([]);
  const [tab, setTab] = useState<QueueTab>("Mine");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);

  const access = me ? accessFor(me) : null;
  const staff = me?.scope === "staff";
  const title = staff ? "My day" : "Overview";

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
        if (a.documents) jobs.push(api.documents(20).then((r) => setDocs(r.data)).catch(() => {}));
      }
      await Promise.all(jobs);
    } catch (e) {
      setErr((e as Error).message);
    }
    setBusy(false);
    setLoaded(true);
  }, [me]);
  useFocusEffect(useCallback(() => void load(), [load]));

  if (!me || !access?.home) {
    return (
      <Screen>
        <EmptyState icon="lock" title="Not available in the app" text="Your role does not use the mobile app. Please use the web portal." />
      </Screen>
    );
  }

  const open = tickets.filter(isOpen);
  const clientName = new Map(clients.map((x) => [x.tenant_id, x.legal_name]));
  const risk = atRisk(open, now);

  const head = (
    <>
      <Stack.Screen options={{ title }} />
      <SyncBanner />
      {!!err && <ErrorBanner message={err} />}
    </>
  );

  if (staff) {
    const queue = queueFor(open, tab, me.sub);
    const attention = needsAttention(open, me.sub, now);
    const approvals = approvalsFor(me.sub, leave, claims, sheets, day, xaf);
    const mineCount = open.filter((t) => t.assignee === me.sub).length;
    return (
      <Screen webTitle={title} refreshing={busy && loaded} onRefresh={load}>
        {head}
        {!loaded && busy ? <Loading /> : (
          <>
            <Section header="Needs attention">
              {attention.map((t) => <TicketRow key={t.id} ticket={t} now={now} client={clientName.get(t.tenant_id)} />)}
              {!attention.length && <Row icon="check" iconTone="label2" title="Nothing needs attention" subtitle="No ticket assigned to you or waiting for an owner is at risk." />}
            </Section>

            <Section header="Overview">
              <Row title="Open tickets" value={String(report ? report.open : open.length)} onPress={() => router.push("/(tabs)/tickets")} />
              <Row title="At risk" value={String(risk)} />
              <Row title="Assigned to me" value={String(mineCount)} />
              {report && <Row title="SLA breached" value={String(report.sla_breached)} />}
              {(access.reviewer || access.approver) && <Row title="Approvals waiting" value={String(approvals.length)} />}
              {access.timeUser && <Row title="Hours this week" value={hoursThisWeek(mine).toFixed(1)} />}
            </Section>

            <Section header="Queue" footer={queue.length > 5 ? `Showing 5 of ${queue.length}.` : undefined}>
              <Segmented inset options={["Mine", "Team", "Unassigned"] as const} labels={QUEUE_LABEL} value={tab} onChange={setTab} />
              {queue.slice(0, 5).map((t) => <TicketRow key={t.id} ticket={t} now={now} client={clientName.get(t.tenant_id)} />)}
              {!queue.length && <Row title="Nothing here" subtitle="The queue for this view is clear." />}
              <Row title="Show all tickets" action onPress={() => router.push("/(tabs)/tickets")} />
            </Section>

            {(access.reviewer || access.approver) && approvals.length > 0 && (
              <Section header="Approvals" footer="Review these in the web portal.">
                {approvals.slice(0, 6).map((a) => <Row key={a.key} title={a.text} titleLines={3} />)}
              </Section>
            )}
          </>
        )}
      </Screen>
    );
  }

  const k = clientKpis(tickets, invoices);
  const actions = clientActions(access, { quotes, invoices, tickets, forms, contracts });
  const byId = new Map(tickets.map((t) => [t.id, t]));
  return (
    <Screen webTitle={title} refreshing={busy && loaded} onRefresh={load}>
      {head}
      {!loaded && busy ? <Loading /> : (
        <>
          <Section header="Awaiting your action" footer={actions.some((a) => a.kind !== "ticket") ? "Approve, pay or sign in the web portal." : undefined}>
            {actions.slice(0, 8).map((a) => {
              const t = a.kind === "ticket" ? byId.get(a.key.slice(1)) : undefined;
              return (
                <Row
                  key={a.key}
                  title={a.title.replace(/^[^·]+· /, "")}
                  subtitle={t ? <View style={{ gap: 2 }}><SeverityText severity={t.severity} suffix="Open" /><SlaText ticket={t} now={now} /></View> : `${KIND[a.kind]}${a.sub ? ` · ${a.sub}` : ""}`}
                  onPress={a.kind === "ticket" ? () => router.push(`/ticket/${a.key.slice(1)}`) : undefined}
                />
              );
            })}
            {!actions.length && <Row icon="check" iconTone="label2" title="Nothing is waiting for you" />}
          </Section>

          <Section header="Overview">
            <Row title="Open tickets" value={String(k.open)} onPress={() => router.push("/(tabs)/tickets")} />
            <Row title="Incidents (P1 and P2)" value={String(k.incidents)} />
            {access.money && <Row title="Balance due" value={xaf(k.balance)} />}
          </Section>

          {access.documents && (
            <Section header="Latest documents">
              {latestDocuments(docs, 3).map((d) => <DocumentRow key={d.ref} doc={d} />)}
              {!docs.length && <Row title="No documents yet" subtitle="Quotations, invoices and receipts issued to you appear here." />}
              {docs.length > 0 && <Row title="Show all documents" action onPress={() => router.push("/(tabs)/documents")} />}
            </Section>
          )}

          <Section>
            <Row icon="plus" title="Report an incident" action onPress={() => router.push("/report")} />
          </Section>
        </>
      )}
    </Screen>
  );
}
