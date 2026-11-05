"use client";
import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { day, when, xaf } from "@/lib/format";
import { PdfActions } from "@/components/PdfActions";
import { Err, Field, Pill, Table, useAction } from "@/components/ui";

interface Clause { position: number; title: string; body: string }
interface Sla { severity: string; respond_minutes: number; resolve_minutes: number }
interface Link { kind: string; ref_id: string; label: string; linked_by: string; linked_at: string }
interface Event { action: string; actor: string; detail: Record<string, unknown>; at: string }
interface Contract {
  id: string; client: string; number: string; title: string; kind: string; status: string; start_date: string; end_date: string; term_months: number; days_left: number;
  value: number; billing: string; period_amount: number; auto_renew: boolean; renewal_term_months: number; renewal_notice_days: number; notes: string; ended_reason: string | null;
  clauses: Clause[]; sla: Sla[]; links: Link[]; history: Event[];
}

const SEV = ["P1", "P2", "P3", "P4"];
const span = (m: number) => (m % 1440 === 0 ? `${m / 1440} day(s)` : m % 60 === 0 ? `${m / 60} hour(s)` : `${m} min`);

export default function ContractPage() {
  const { id } = useParams<{ id: string }>();
  const me = useMe();
  const staff = me.scope === "staff";
  const writer = me.roles.some((r) => ["ceo", "manager", "sales"].includes(r));
  const [c, setC] = useState<Contract | null>(null);
  const [loadErr, setLoadErr] = useState("");
  const [clauses, setClauses] = useState<Clause[]>([]);
  const [sla, setSla] = useState<Record<string, { respond: string; resolve: string }>>({});
  const [reason, setReason] = useState("");
  const [months, setMonths] = useState("");
  const [link, setLink] = useState({ kind: "project", ref_id: "", label: "" });
  const load = useCallback(() => raw.get<Contract>(`/v1/contracts/${id}`).then((x) => {
    setC(x);
    setClauses(x.clauses.map((k) => ({ ...k })));
    setSla(Object.fromEntries(x.sla.map((s) => [s.severity, { respond: String(s.respond_minutes), resolve: String(s.resolve_minutes) }])));
  }).catch((e) => setLoadErr(e.message)), [id]);
  useEffect(() => { void load(); }, [load]);
  const act = useAction(load);
  if (loadErr && !c) return <p className="error">{loadErr}</p>;
  if (!c) return <p className="muted">Loading…</p>;
  const draft = c.status === "draft";
  const live = c.status === "active" || c.status === "suspended";
  const post = (path: string, body: unknown = {}) => act.run(() => raw.post(path, body));
  return (
    <>
      <h1>{c.number} · {c.title}</h1>
      <p className="meta">
        <Pill text={c.status} tone={c.status === "suspended" ? "warn" : c.status === "expired" || c.status === "terminated" ? "bad" : undefined} /> · {c.client} · {c.kind} · {day(c.start_date)} to {day(c.end_date)} ({c.term_months} months
        {c.status === "active" && `, ${c.days_left} days left`}) · {xaf(c.value)} · {c.billing}, {xaf(c.period_amount)} per period
        {c.auto_renew ? ` · renews automatically for ${c.renewal_term_months} months` : " · does not renew automatically"}
      </p>
      {c.ended_reason && <p className="notice">Ended: {c.ended_reason}</p>}
      <div className="actions">
        <PdfActions path={`/v1/contracts/${c.id}/pdf`} reference={c.number} title="Contract schedule" />
        {writer && draft && <button className="btn primary" disabled={act.busy} onClick={() => post(`/v1/contracts/${id}/activate`)}>Activate</button>}
        {writer && c.status === "active" && <button className="btn" disabled={act.busy} onClick={() => post(`/v1/contracts/${id}/renew`, months ? { months: Number(months) } : {})}>Renew{months ? ` by ${months} months` : ` by ${c.renewal_term_months} months`}</button>}
        {writer && c.status === "expired" && <button className="btn primary" disabled={act.busy} onClick={() => post(`/v1/contracts/${id}/renew`, months ? { months: Number(months) } : {})}>Renew</button>}
        {writer && c.status === "suspended" && <button className="btn" disabled={act.busy} onClick={() => post(`/v1/contracts/${id}/resume`)}>Resume</button>}
      </div>
      {writer && (c.status === "active" || c.status === "expired") && (
        <p><label className="small">Renew by (months, blank = {c.renewal_term_months}) <input type="number" min="1" max="120" value={months} onChange={(e) => setMonths(e.target.value)} style={{ width: "6rem" }} /></label></p>
      )}
      <Err msg={act.err} />

      <h2>Response and resolution times</h2>
      {writer && !(c.status === "expired" || c.status === "terminated") ? (
        <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(() => raw.put(`/v1/contracts/${id}/sla`, { sla: SEV.filter((s) => sla[s]?.respond && sla[s]?.resolve).map((s) => ({ severity: s, respond_minutes: Number(sla[s].respond), resolve_minutes: Number(sla[s].resolve) })) })); }}>
          <p className="muted small">In minutes. While the contract is active, tickets from this client use these times instead of the standard ones. Leave a severity blank to keep the standard target.</p>
          {SEV.map((s) => (
            <div className="grid2" key={s}>
              <Field id={`sla-r-${s}`} label={`${s}: respond within (minutes)`}><input id={`sla-r-${s}`} type="number" min="1" value={sla[s]?.respond ?? ""} onChange={(e) => setSla({ ...sla, [s]: { respond: e.target.value, resolve: sla[s]?.resolve ?? "" } })} /></Field>
              <Field id={`sla-v-${s}`} label={`${s}: resolve within (minutes)`}><input id={`sla-v-${s}`} type="number" min="1" value={sla[s]?.resolve ?? ""} onChange={(e) => setSla({ ...sla, [s]: { respond: sla[s]?.respond ?? "", resolve: e.target.value } })} /></Field>
            </div>
          ))}
          <button className="btn" disabled={act.busy}>Save times</button>
        </form>
      ) : (
        <Table head={["Severity", "Respond within", "Resolve within"]} empty={c.sla.length ? undefined : "The standard support targets apply."}>
          {c.sla.map((s) => <tr key={s.severity}><td>{s.severity}</td><td>{span(s.respond_minutes)}</td><td>{span(s.resolve_minutes)}</td></tr>)}
        </Table>
      )}

      <h2>Clauses</h2>
      {writer && draft ? (
        <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(() => raw.put(`/v1/contracts/${id}/clauses`, { clauses: clauses.filter((k) => k.title.trim() || k.body.trim()) })); }}>
          {clauses.map((k, i) => (
            <fieldset key={i} className="lines">
              <legend>Clause {i + 1}</legend>
              <input aria-label={`Clause ${i + 1} title`} placeholder="Title" value={k.title} onChange={(e) => setClauses(clauses.map((x, j) => (j === i ? { ...x, title: e.target.value } : x)))} />
              <textarea aria-label={`Clause ${i + 1} text`} rows={3} placeholder="Text" value={k.body} onChange={(e) => setClauses(clauses.map((x, j) => (j === i ? { ...x, body: e.target.value } : x)))} />
              <button type="button" className="btn ghost" onClick={() => setClauses(clauses.filter((_, j) => j !== i))}>Remove</button>
            </fieldset>
          ))}
          <button type="button" className="btn" onClick={() => setClauses([...clauses, { position: clauses.length + 1, title: "", body: "" }])}>Add clause</button>
          <button className="btn primary" disabled={act.busy}>Save clauses</button>
        </form>
      ) : (
        <ol className="clauses">{c.clauses.map((k) => <li key={k.position}><strong>{k.title}</strong><p>{k.body}</p></li>)}{c.clauses.length === 0 && <p className="muted">No clauses.</p>}</ol>
      )}

      {staff && (
        <>
          <h2>Covered by this contract</h2>
          <Table head={["Kind", "Reference", "Label", "Linked by", ""]} empty={c.links.length ? undefined : "Nothing linked yet."}>
            {c.links.map((l) => (
              <tr key={l.kind + l.ref_id}><td>{l.kind}</td><td><code>{l.ref_id.slice(0, 8)}</code></td><td>{l.label || "–"}</td><td>{l.linked_by}</td>
                <td>{writer && <button className="btn ghost" onClick={() => act.run(() => raw.del(`/v1/contracts/${id}/links/${l.kind}/${l.ref_id}`))}>Remove</button>}</td></tr>
            ))}
          </Table>
          {writer && !(c.status === "expired" || c.status === "terminated") && (
            <form className="inline" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post(`/v1/contracts/${id}/links`, link); setLink({ ...link, ref_id: "", label: "" }); }); }}>
              <Field id="lk-kind" label="Kind"><select id="lk-kind" value={link.kind} onChange={(e) => setLink({ ...link, kind: e.target.value })}>{["project", "asset", "invoice", "ticket"].map((k) => <option key={k}>{k}</option>)}</select></Field>
              <Field id="lk-ref" label="Id"><input id="lk-ref" required value={link.ref_id} onChange={(e) => setLink({ ...link, ref_id: e.target.value })} placeholder="Paste the id from its page" /></Field>
              <Field id="lk-label" label="Label"><input id="lk-label" value={link.label} onChange={(e) => setLink({ ...link, label: e.target.value })} /></Field>
              <button className="btn">Link</button>
            </form>
          )}
          {writer && live && (
            <>
              <h2>{c.status === "active" ? "Suspend or terminate" : "Terminate"}</h2>
              <div className="inline">
                <Field id="ct-reason" label="Reason (required)"><input id="ct-reason" value={reason} onChange={(e) => setReason(e.target.value)} /></Field>
                {c.status === "active" && <button className="btn" disabled={act.busy || !reason.trim()} onClick={() => post(`/v1/contracts/${id}/suspend`, { reason })}>Suspend</button>}
                <button className="btn danger" disabled={act.busy || !reason.trim()} onClick={() => { if (confirm("Terminate this contract? This cannot be undone.")) post(`/v1/contracts/${id}/terminate`, { reason }); }}>Terminate</button>
              </div>
            </>
          )}
          <h2>History</h2>
          <Table head={["When", "What", "Who", "Detail"]} empty={c.history.length ? undefined : "No history."}>
            {c.history.map((h, i) => <tr key={i}><td>{when(h.at)}</td><td>{h.action.replace(/_/g, " ")}</td><td>{h.actor}</td><td className="wrap">{Object.entries(h.detail ?? {}).map(([k, v]) => `${k}: ${String(v)}`).join(" · ")}</td></tr>)}
          </Table>
        </>
      )}
    </>
  );
}
