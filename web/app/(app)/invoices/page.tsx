"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { xaf, day } from "@/lib/format";
import type { Invoice } from "@/lib/sales-types";
import { api } from "@/lib/api";
import type { Client } from "@/lib/types";
import { Err, Field, LinesEditor, Pill, Table, emptyLine, toLines, useAction, type LineInput } from "@/components/ui";

export const invoiceTone = (s: string) => (s === "paid" ? "ok" : s === "pending_approval" ? "bad" : undefined);

export default function Invoices() {
  const me = useMe();
  const canWrite = me.roles.some((r) => ["finance", "sales", "manager", "ceo"].includes(r));
  const [rows, setRows] = useState<Invoice[]>([]);
  const [clients, setClients] = useState<Client[]>([]);
  const [f, setF] = useState({ tenant_id: "", title: "", due_date: "" });
  const [lines, setLines] = useState<LineInput[]>([emptyLine()]);
  const [loadErr, setLoadErr] = useState("");
  const load = () => raw.get<{ data: Invoice[] }>("/v1/invoices").then((r) => setRows(r.data)).catch((e) => setLoadErr(e.message));
  const act = useAction(async () => { setF({ tenant_id: "", title: "", due_date: "" }); setLines([emptyLine()]); await load(); });
  useEffect(() => {
    void load();
    if (canWrite) api.clients().then((r) => setClients(r.data)).catch(() => {});
  }, [canWrite]);
  return (
    <>
      <h1>Invoices</h1>
      <Err msg={loadErr} />
      <Table head={["Reference", "Title", "Status", "Due", "Total", "Balance"]} empty={rows.length ? undefined : "No invoices yet."}>
        {rows.map((v) => (
          <tr key={v.id}>
            <td>{v.ref ?? "draft"}</td>
            <td><Link href={`/invoices/${v.id}`}>{v.title}</Link></td>
            <td><Pill text={v.status} tone={invoiceTone(v.status)} /></td>
            <td>{day(v.due_date)}</td>
            <td className="num">{xaf(v.total)}</td>
            <td className="num">{v.status === "draft" || v.status === "pending_approval" ? "–" : xaf(v.balance)}</td>
          </tr>
        ))}
      </Table>
      {canWrite && (
        <>
          <h2>New invoice</h2>
          <form className="card form" onSubmit={(e) => {
            e.preventDefault();
            void act.run(() => raw.post("/v1/invoices", { ...f, due_date: f.due_date || undefined, lines: toLines(lines) }));
          }}>
            <Field id="i-client" label="Client">
              <select id="i-client" required value={f.tenant_id} onChange={(e) => setF({ ...f, tenant_id: e.target.value })}>
                <option value="">Select…</option>
                {clients.map((c) => <option key={c.tenant_id} value={c.tenant_id}>{c.legal_name}</option>)}
              </select>
            </Field>
            <Field id="i-title" label="Title"><input id="i-title" required value={f.title} onChange={(e) => setF({ ...f, title: e.target.value })} /></Field>
            <Field id="i-due" label="Due date"><input id="i-due" type="date" value={f.due_date} onChange={(e) => setF({ ...f, due_date: e.target.value })} /></Field>
            <LinesEditor lines={lines} onChange={setLines} />
            <Err msg={act.err} />
            <button className="btn primary" disabled={act.busy}>Create draft</button>
          </form>
        </>
      )}
    </>
  );
}
