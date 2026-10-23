import { useCallback, useState } from "react";
import { Pressable, RefreshControl, ScrollView, Text, View } from "react-native";
import { router, useFocusEffect } from "expo-router";
import { PostureCard } from "../../components/charts";
import { Empty, Kpi, RowItem, Section } from "../../components/dash";
import { Icon } from "../../components/Icon";
import { Button, EmptyState, ErrorState, Pill, Screen, ScreenHeader, Segmented, SevBadge, SkeletonRows, SlaChip, SlaRingView, sevColor } from "../../components/ui";
import { api, listOf } from "../../lib/api";
import { accessFor, approvalsFor, atRisk, clientActions, clientKpis, hoursThisWeek, isOpen, latestDocuments, nextDeadlines, queueFor, sevCounts, type Claim, type ContractRow, type FormRow, type InvoiceRow, type Leave, type QuoteRow, type QueueTab, type Sheet } from "../../lib/home-core";
import { FadeIn } from "../../lib/motion";
import { useSession } from "../../lib/session";
import { sp, useTheme } from "../../lib/theme";
import { postureScore } from "../../lib/sla";
import type { Client, DocumentEntry, SupportReport, Ticket } from "../../lib/types";

const xaf = (n: number) => `${new Intl.NumberFormat("fr-FR").format(n)} XAF`;
const day = (s: string) => new Date(s).toLocaleDateString();
const roleLabel = (r: string) => (r === "ceo" ? "CEO" : r.charAt(0).toUpperCase() + r.slice(1).replace(/_/g, " "));
const KIND: Record<string, string> = { quote: "Quotation", invoice: "Invoice", ticket: "Ticket", form: "Form", contract: "Contract" };
const shortId = (id: string) => id.slice(0, 8).toUpperCase();

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
  const [docs, setDocs] = useState<DocumentEntry[]>([]);
  const [tab, setTab] = useState<QueueTab>("Mine");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [updated, setUpdated] = useState<Date | null>(null);

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
        if (a.documents) jobs.push(api.documents(20).then((r) => setDocs(r.data)).catch(() => {}));
      }
      await Promise.all(jobs);
      setUpdated(new Date());
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
        <View style={s.pad}><EmptyState icon="lock" title="Not available in the app" text="Your role does not use the mobile app. Please use the web portal." /></View>
      </Screen>
    );
  }

  const today = new Date().toLocaleDateString("en-GB", { weekday: "long", day: "numeric", month: "long" });
  const open = tickets.filter(isOpen);
  const clientName = new Map(clients.map((x) => [x.tenant_id, x.legal_name]));
  const risk = atRisk(open);
  const counts = sevCounts(open);
  const refresh = <RefreshControl refreshing={busy && loaded} onRefresh={load} tintColor={c.gold} colors={[c.gold]} progressBackgroundColor={c.surface} />;
  const first = !loaded && busy;
  const score = postureScore(open.length, risk);
  const tone = open.length === 0 ? "neutral" : risk === 0 ? "ok" : risk >= Math.ceil(open.length / 2) ? "bad" : "warn";

  const deadlines = nextDeadlines(open, 3);
  const deadlineBlock = deadlines.length > 0 && (
    <Section title="Next SLA deadlines" count={deadlines.length}>
      {deadlines.map((t, i) => (
        <Pressable key={t.id} accessibilityRole="button" onPress={() => router.push(`/ticket/${t.id}`)} style={({ pressed }) => ({ flexDirection: "row", alignItems: "center", gap: sp.md, padding: sp.md, backgroundColor: pressed ? c.raised : "transparent", borderBottomColor: c.lineSoft, borderBottomWidth: i === deadlines.length - 1 ? 0 : 1 })}>
          <SlaRingView t={t} size={52} />
          <View style={{ flex: 1, gap: 3 }}>
            <Text style={[s.mono, { color: c.goldText, fontSize: 11 }]}>{shortId(t.id)}</Text>
            <Text style={[s.text, s.semibold, { fontSize: 15 }]} numberOfLines={1}>{t.subject}</Text>
            <SlaChip t={t} />
          </View>
          <SevBadge sev={t.severity} />
        </Pressable>
      ))}
    </Section>
  );

  if (staff) {
    const queue = queueFor(open, tab, me.sub);
    const approvals = approvalsFor(me.sub, leave, claims, sheets, day, xaf);
    const mineCount = open.filter((t) => t.assignee === me.sub).length;
    return (
      <Screen updated={updated}>
        <ScrollView contentContainerStyle={{ padding: sp.lg, gap: sp.lg }} refreshControl={refresh}>
          <ScreenHeader overline={`${today} · ${me.roles.map(roleLabel).join(", ")}`} title="My day" />
          {!!err && <ErrorState message={err} onRetry={load} />}
          {first ? <SkeletonRows n={3} h={110} /> : (
            <>
              <FadeIn>
                <PostureCard
                  title="SLA posture" score={score === null ? "–" : `${score}%`} scoreLabel="within SLA" fraction={score === null ? 0 : score / 100} tone={tone}
                  counts={counts}
                  caption={score === null ? "No open tickets in your scope." : "Share of open tickets that are not overdue, breached or within 4 h of their deadline."}
                  stats={[
                    { label: "Open tickets", value: report ? report.open : open.length },
                    { label: "At risk", value: risk, tone: risk ? "bad" : "ok" },
                    { label: "P1 open", value: report ? report.p1_open : counts.P1, tone: (report ? report.p1_open : counts.P1) ? "bad" : undefined },
                  ]}
                />
              </FadeIn>
              <FadeIn index={1}>
                <View style={{ flexDirection: "row", flexWrap: "wrap", gap: sp.sm }}>
                  <Kpi icon="user" label="Assigned to me" value={mineCount} />
                  <Kpi icon="alert" label="SLA breached" value={report ? report.sla_breached : "–"} tone={report && report.sla_breached ? "bad" : undefined} sub={report ? "in total" : "Not in your scope"} />
                  {(access.reviewer || access.approver) && <Kpi icon="docCheck" label="Approvals waiting" value={approvals.length} />}
                  {access.timeUser && <Kpi icon="clock" label="Hours this week" value={hoursThisWeek(mine).toFixed(1)} />}
                  {report && <Kpi icon="shieldCheck" label="Avg rating" value={report.avg_rating == null ? "–" : report.avg_rating.toFixed(1)} sub="out of 5" />}
                </View>
              </FadeIn>
              <FadeIn index={2}>{deadlineBlock}</FadeIn>
              <FadeIn index={3}>
                <Section title="Ticket queue" count={queue.length}>
                  <View style={{ padding: sp.sm, borderBottomColor: c.lineSoft, borderBottomWidth: 1 }}>
                    <Segmented options={["Mine", "Team", "Unassigned"] as const} value={tab} onChange={setTab} />
                  </View>
                  {queue.slice(0, 8).map((t, i, a) => (
                    <RowItem key={t.id} last={i === a.length - 1} rail={sevColor(c, t.severity)} mono={shortId(t.id)} title={t.subject}
                      sub={<View style={{ gap: 3 }}>{!!clientName.get(t.tenant_id) && <Text style={[s.muted, { fontSize: 13 }]}>{clientName.get(t.tenant_id)}</Text>}<SlaChip t={t} /></View>}
                      onPress={() => router.push(`/ticket/${t.id}`)} right={<SevBadge sev={t.severity} />} />
                  ))}
                  {!queue.length && <Empty text="Nothing here. The queue for this view is clear." />}
                </Section>
              </FadeIn>
              {(access.reviewer || access.approver) && (
                <FadeIn index={4}>
                  <Section title="Approvals" count={approvals.length}>
                    {approvals.slice(0, 6).map((a, i, all) => <RowItem key={a.key} last={i === all.length - 1} title={a.text} />)}
                    {!approvals.length && <Empty text="Nothing waiting for you." />}
                    {approvals.length > 0 && <Text style={[s.muted, { padding: sp.md, paddingTop: 0, fontSize: 13 }]}>Review these in the web portal.</Text>}
                  </Section>
                </FadeIn>
              )}
            </>
          )}
        </ScrollView>
      </Screen>
    );
  }

  const k = clientKpis(tickets, invoices);
  const actions = clientActions(access, { quotes, invoices, tickets, forms, contracts });
  const clientTone = k.sla === null ? (k.open ? "ok" : "neutral") : k.sla === 100 ? "ok" : k.sla >= 80 ? "warn" : "bad";
  return (
    <Screen updated={updated}>
      <ScrollView contentContainerStyle={{ padding: sp.lg, gap: sp.lg }} refreshControl={refresh}>
        <ScreenHeader overline={today} title="Overview" />
        {!!err && <ErrorState message={err} onRetry={load} />}
        {first ? <SkeletonRows n={3} h={110} /> : (
          <>
            <FadeIn>
              <PostureCard
                title="Service posture" score={k.sla === null ? "–" : `${k.sla}%`} scoreLabel="SLA this month" fraction={k.sla === null ? 0 : k.sla / 100} tone={clientTone}
                counts={counts}
                caption={k.sla === null ? "No ticket has been resolved this month yet." : k.sla === 100 ? "Every ticket resolved this month met its SLA." : `${k.resolved - k.met} of ${k.resolved} tickets resolved this month missed their SLA.`}
                stats={[
                  { label: "Open tickets", value: k.open },
                  { label: "Incidents (P1/P2)", value: k.incidents, tone: k.incidents ? "warn" : "ok" },
                  ...(access.money ? [{ label: "Balance due", value: xaf(k.balance), tone: k.owing ? ("warn" as const) : undefined }] : []),
                ]}
              />
            </FadeIn>
            <FadeIn index={1}>{deadlineBlock}</FadeIn>
            <FadeIn index={2}>
              <Section title="Awaiting your action" count={actions.length}>
                {actions.slice(0, 12).map((a, i, all) => (
                  <RowItem key={a.key} last={i === all.length - 1} mono={a.sub || undefined} title={a.title.replace(/^[^·]+· /, "")}
                    onPress={a.kind === "ticket" ? () => router.push(`/ticket/${a.key.slice(1)}`) : undefined}
                    right={<Pill text={KIND[a.kind]} tone={a.kind === "invoice" ? "warn" : a.kind === "ticket" ? "gold" : "neutral"} />} />
                ))}
                {!actions.length && <EmptyState icon="shieldCheck" title="All clear" text="Nothing is waiting for you." />}
                {actions.some((a) => a.kind !== "ticket") && (
                  <View style={{ flexDirection: "row", gap: 8, padding: sp.md, alignItems: "center" }}>
                    <Icon name="info" size={14} color={c.muted} />
                    <Text style={[s.muted, { fontSize: 13, flex: 1 }]}>Approve, pay or sign in the web portal.</Text>
                  </View>
                )}
              </Section>
            </FadeIn>
            {access.documents && (
              <FadeIn index={3}>
                <Section title="Latest documents" count={docs.length}>
                  {latestDocuments(docs, 3).map((d, i, all) => (
                    <RowItem key={d.ref} last={i === all.length - 1} mono={d.ref} title={d.title} sub={`${d.code} · ${day(d.issued_at)}`}
                      onPress={() => router.push("/(tabs)/documents")}
                      right={<Pill text={d.status} tone={d.status === "valid" ? "ok" : "bad"} />} />
                  ))}
                  {!docs.length && <EmptyState icon="docCheck" title="No documents yet" text="Quotations, invoices and receipts issued to you appear here." />}
                </Section>
              </FadeIn>
            )}
            <FadeIn index={4}>
              <View style={{ flexDirection: "row", flexWrap: "wrap", gap: sp.sm }}>
                <Kpi icon="tickets" label="Resolved this month" value={k.resolved} sub={k.resolved ? `${k.met} within SLA` : "None yet"} tone={k.resolved && k.met === k.resolved ? "ok" : undefined} />
                {access.money && <Kpi icon="documents" label="Invoices owing" value={k.owing} sub={k.owing ? xaf(k.balance) : "Nothing owed"} />}
              </View>
            </FadeIn>
            <Button icon="alert" title="Report an incident" onPress={() => router.push("/(tabs)/report")} />
          </>
        )}
      </ScrollView>
    </Screen>
  );
}

