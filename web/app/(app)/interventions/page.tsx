"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { api, raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { when } from "@/lib/format";
import type { Client } from "@/lib/types";
import { Err, Field, Pill, Table, useAction } from "@/components/ui";

interface Intervention { id: string; tenant_id: string; engineer: string; status: string; scheduled_for: string | null; summary: string; checkin_at: string | null; checkout_at: string | null; document_ref: string | null; has_signature: boolean }

export default function Interventions() {
  const me = useMe();
  const dispatcher = me.roles.some((r) => ["support", "manager", "ceo"].includes(r));
  const [rows, setRows] = useState<Intervention[]>([]);
  const [clients, setClients] = useState<Client[]>([]);
  const [f, setF] = useState({ tenant_id: "", engineer: "", scheduled_for: "", summary: "" });
  const [loadErr, setLoadErr] = useState("");
  const load = () => raw.get<{ data: Intervention[] }>("/v1/interventions").then((r) => setRows(r.data)).catch((e) => setLoadErr(e.message));
  const act = useAction(async () => { setF({ tenant_id: "", engineer: "", scheduled_for: "", summary: "" }); await load(); });
  useEffect(() => { void load(); if (dispatcher) api.clients().then((r) => setClients(r.data)).catch(() => {}); }, [dispatcher]);
  return (
    <>
      <h1>{me.scope === "client" ? "Site visits" : "Field interventions"}</h1>
      <p className="muted">Engineers check in, work and complete these on the mobile app, with GPS and the client's signature.</p>
      <Err msg={loadErr} />
      <Table head={["Summary", "Engineer", "Scheduled", "Status", "Document"]} empty={rows.length ? undefined : "Nothing here yet."}>
        {rows.map((i) => (
          <tr key={i.id}>
            <td><Link href={`/interventions/${i.id}`}>{i.summary || "Intervention"}</Link></td><td>{i.engineer}</td><td>{when(i.scheduled_for)}</td>
            <td><Pill text={i.status} tone={i.status === "completed" ? "ok" : undefined} /></td><td>{i.document_ref ?? "–"}</td>
          </tr>
        ))}
      </Table>
      {dispatcher && (
        <>
          <h2>Dispatch an engineer</h2>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(() => raw.post("/v1/interventions", { tenant_id: f.tenant_id, engineer: f.engineer, summary: f.summary, scheduled_for: f.scheduled_for ? new Date(f.scheduled_for).toISOString() : undefined })); }}>
            <Field id="iv-client" label="Client">
              <select id="iv-client" required value={f.tenant_id} onChange={(e) => setF({ ...f, tenant_id: e.target.value })}>
                <option value="">Select…</option>
                {clients.map((c) => <option key={c.tenant_id} value={c.tenant_id}>{c.legal_name}</option>)}
              </select>
            </Field>
            <Field id="iv-eng" label="Engineer (account id)"><input id="iv-eng" required value={f.engineer} onChange={(e) => setF({ ...f, engineer: e.target.value })} /></Field>
            <Field id="iv-when" label="Scheduled for"><input id="iv-when" type="datetime-local" value={f.scheduled_for} onChange={(e) => setF({ ...f, scheduled_for: e.target.value })} /></Field>
            <Field id="iv-sum" label="Summary"><input id="iv-sum" value={f.summary} onChange={(e) => setF({ ...f, summary: e.target.value })} /></Field>
            <Err msg={act.err} />
            <button className="btn primary" disabled={act.busy}>Dispatch</button>
          </form>
        </>
      )}
    </>
  );
}
