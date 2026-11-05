"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { api, raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { day, xaf } from "@/lib/format";
import type { Client } from "@/lib/types";
import { Err, Field, Pill, Table, useAction } from "@/components/ui";

export interface ContractRow {
  id: string; client: string; number: string; title: string; kind: string; status: string; start_date: string; end_date: string;
  term_months: number; days_left: number; value: number; billing: string; period_amount: number; auto_renew: boolean;
}

const blank = { tenant_id: "", title: "", kind: "maintenance", start_date: "", end_date: "", value: "", billing: "monthly", period_amount: "", auto_renew: false, renewal_term_months: "12", renewal_notice_days: "60" };

export default function Contracts() {
  const me = useMe();
  const writer = me.roles.some((r) => ["ceo", "manager", "sales"].includes(r));
  const staff = me.scope === "staff";
  const [rows, setRows] = useState<ContractRow[]>([]);
  const [soon, setSoon] = useState<ContractRow[]>([]);
  const [clients, setClients] = useState<Client[]>([]);
  const [f, setF] = useState(blank);
  const [loadErr, setLoadErr] = useState("");
  const load = async () => {
    try {
      setRows((await raw.get<{ data: ContractRow[] }>("/v1/contracts")).data);
      if (staff) setSoon((await raw.get<{ data: ContractRow[] }>("/v1/contracts/expiring?days=90")).data);
    } catch (e) { setLoadErr((e as Error).message); }
  };
  const act = useAction(async () => { setF(blank); await load(); });
  useEffect(() => { void load(); if (writer) api.clients().then((r) => setClients(r.data)).catch(() => {}); /* eslint-disable-next-line react-hooks/exhaustive-deps */ }, [writer]);
  return (
    <>
      <h1>{staff ? "Contracts" : "Your service contracts"}</h1>
      <Err msg={loadErr} />
      {soon.length > 0 && (
        <p className="notice" role="status">
          {soon.length} contract(s) end within 90 days: {soon.map((c) => `${c.number} (${c.days_left} days)`).join(", ")}
        </p>
      )}
      <Table head={["Number", ...(staff ? ["Client"] : []), "Title", "Status", "Ends", "Left", "Term", "Value"]} empty={rows.length ? undefined : "No contracts yet."}>
        {rows.map((c) => (
          <tr key={c.id}>
            <td><Link href={`/contracts/${c.id}`}>{c.number}</Link></td>
            {staff && <td>{c.client}</td>}
            <td className="wrap">{c.title}</td>
            <td><Pill text={c.status} tone={c.status === "suspended" ? "warn" : c.status === "expired" || c.status === "terminated" ? "bad" : undefined} /></td>
            <td>{day(c.end_date)}</td>
            <td className={c.status === "active" && c.days_left <= 90 ? "tl bad" : ""}>{c.status === "active" ? `${c.days_left} days` : "–"}</td>
            <td>{c.term_months} months{c.auto_renew && " · auto-renews"}</td>
            <td className="num">{xaf(c.value)}</td>
          </tr>
        ))}
      </Table>
      {writer && (
        <>
          <h2>New contract</h2>
          <p className="muted small">A contract starts as a draft. Add its clauses and the response times it promises, then activate it.</p>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => {
            const c = await raw.post<{ id: string }>("/v1/contracts", {
              tenant_id: f.tenant_id, title: f.title, kind: f.kind, start_date: f.start_date, end_date: f.end_date,
              value: Math.round(Number(f.value || 0)), billing: f.billing, period_amount: Math.round(Number(f.period_amount || 0)),
              auto_renew: f.auto_renew, renewal_term_months: Number(f.renewal_term_months), renewal_notice_days: Number(f.renewal_notice_days),
            });
            location.assign(`/contracts/${c.id}`);
          }); }}>
            <Field id="ct-client" label="Client">
              <select id="ct-client" required value={f.tenant_id} onChange={(e) => setF({ ...f, tenant_id: e.target.value })}>
                <option value="">Select…</option>
                {clients.map((c) => <option key={c.tenant_id} value={c.tenant_id}>{c.legal_name}</option>)}
              </select>
            </Field>
            <Field id="ct-title" label="Title"><input id="ct-title" required value={f.title} onChange={(e) => setF({ ...f, title: e.target.value })} placeholder="Five-year maintenance of the server room" /></Field>
            <div className="grid2">
              <Field id="ct-kind" label="Kind"><select id="ct-kind" value={f.kind} onChange={(e) => setF({ ...f, kind: e.target.value })}>{["maintenance", "support", "supply", "service", "licence"].map((k) => <option key={k}>{k}</option>)}</select></Field>
              <Field id="ct-bill" label="Billing"><select id="ct-bill" value={f.billing} onChange={(e) => setF({ ...f, billing: e.target.value })}>{["monthly", "quarterly", "annual", "one_off"].map((k) => <option key={k}>{k}</option>)}</select></Field>
            </div>
            <div className="grid2">
              <Field id="ct-start" label="Starts"><input id="ct-start" type="date" required value={f.start_date} onChange={(e) => setF({ ...f, start_date: e.target.value })} /></Field>
              <Field id="ct-end" label="Ends"><input id="ct-end" type="date" required value={f.end_date} onChange={(e) => setF({ ...f, end_date: e.target.value })} /></Field>
            </div>
            <div className="grid2">
              <Field id="ct-value" label="Total value (XAF)"><input id="ct-value" type="number" min="0" step="1" value={f.value} onChange={(e) => setF({ ...f, value: e.target.value })} /></Field>
              <Field id="ct-per" label="Per billing period (XAF)"><input id="ct-per" type="number" min="0" step="1" value={f.period_amount} onChange={(e) => setF({ ...f, period_amount: e.target.value })} /></Field>
            </div>
            <label className="check"><input type="checkbox" checked={f.auto_renew} onChange={(e) => setF({ ...f, auto_renew: e.target.checked })} /> Renews automatically at the end of the term</label>
            <div className="grid2">
              <Field id="ct-rt" label="Renewal term (months)"><input id="ct-rt" type="number" min="1" max="120" value={f.renewal_term_months} onChange={(e) => setF({ ...f, renewal_term_months: e.target.value })} /></Field>
              <Field id="ct-rn" label="Notice before the end (days)"><input id="ct-rn" type="number" min="0" max="365" value={f.renewal_notice_days} onChange={(e) => setF({ ...f, renewal_notice_days: e.target.value })} /></Field>
            </div>
            <Err msg={act.err} />
            <button className="btn primary" disabled={act.busy}>Create draft</button>
          </form>
        </>
      )}
    </>
  );
}
