"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { raw, api } from "@/lib/api";
import { useMe } from "@/lib/session";
import { xaf, day } from "@/lib/format";
import type { Quote } from "@/lib/sales-types";
import type { Client } from "@/lib/types";
import { Err, Field, LinesEditor, Pill, Table, emptyLine, toLines, useAction, type LineInput } from "@/components/ui";

export default function Quotes() {
  const me = useMe();
  const canWrite = me.roles.some((r) => ["sales", "manager", "ceo"].includes(r));
  const [rows, setRows] = useState<Quote[]>([]);
  const [clients, setClients] = useState<Client[]>([]);
  const [f, setF] = useState({ tenant_id: "", title: "", valid_until: "", notes: "" });
  const [lines, setLines] = useState<LineInput[]>([emptyLine()]);
  const [loadErr, setLoadErr] = useState("");
  const load = () => raw.get<{ data: Quote[] }>("/v1/quotes").then((r) => setRows(r.data)).catch((e) => setLoadErr(e.message));
  const act = useAction(async () => {
    setF({ tenant_id: "", title: "", valid_until: "", notes: "" });
    setLines([emptyLine()]);
    await load();
  });
  useEffect(() => {
    void load();
    if (canWrite) api.clients().then((r) => setClients(r.data)).catch(() => {});
  }, [canWrite]);
  return (
    <>
      <h1>Quotations</h1>
      <Err msg={loadErr} />
      <Table head={["Reference", "Title", "Status", "Valid until", "Total"]} empty={rows.length ? undefined : "No quotations yet."}>
        {rows.map((q) => (
          <tr key={q.id}>
            <td>{q.ref ?? "draft"}</td>
            <td><Link href={`/quotes/${q.id}`}>{q.title}</Link></td>
            <td><Pill text={q.status} tone={q.status === "approved" ? "ok" : q.status === "rejected" ? "bad" : undefined} /></td>
            <td>{day(q.valid_until)}</td>
            <td className="num">{xaf(q.total)}</td>
          </tr>
        ))}
      </Table>
      {canWrite && (
        <>
          <h2>New quotation</h2>
          <form className="card form" onSubmit={(e) => {
            e.preventDefault();
            void act.run(() => raw.post("/v1/quotes", { ...f, valid_until: f.valid_until || undefined, lines: toLines(lines) }));
          }}>
            <Field id="q-client" label="Client">
              <select id="q-client" required value={f.tenant_id} onChange={(e) => setF({ ...f, tenant_id: e.target.value })}>
                <option value="">Select…</option>
                {clients.map((c) => <option key={c.tenant_id} value={c.tenant_id}>{c.legal_name}</option>)}
              </select>
            </Field>
            <Field id="q-title" label="Title"><input id="q-title" required value={f.title} onChange={(e) => setF({ ...f, title: e.target.value })} /></Field>
            <Field id="q-valid" label="Valid until"><input id="q-valid" type="date" value={f.valid_until} onChange={(e) => setF({ ...f, valid_until: e.target.value })} /></Field>
            <LinesEditor lines={lines} onChange={setLines} />
            <Err msg={act.err} />
            <button className="btn primary" disabled={act.busy}>Create draft</button>
          </form>
        </>
      )}
    </>
  );
}
