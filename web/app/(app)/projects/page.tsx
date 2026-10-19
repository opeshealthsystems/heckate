"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { api, raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { day, xaf } from "@/lib/format";
import type { Client } from "@/lib/types";
import { Err, Field, Pill, Table, useAction } from "@/components/ui";

interface Project { id: string; tenant_id: string; name: string; status: string; manager: string | null; start_date: string | null; end_date: string | null; budget: number | null }

export default function Projects() {
  const me = useMe();
  const manager = me.roles.some((r) => ["manager", "ceo"].includes(r));
  const [rows, setRows] = useState<Project[]>([]);
  const [clients, setClients] = useState<Client[]>([]);
  const [f, setF] = useState({ tenant_id: "", name: "", manager: "", start_date: "", end_date: "", budget: "" });
  const [loadErr, setLoadErr] = useState("");
  const load = () => raw.get<{ data: Project[] }>("/v1/projects").then((r) => setRows(r.data)).catch((e) => setLoadErr(e.message));
  const act = useAction(async () => { setF({ tenant_id: "", name: "", manager: "", start_date: "", end_date: "", budget: "" }); await load(); });
  useEffect(() => { void load(); if (manager) api.clients().then((r) => setClients(r.data)).catch(() => {}); }, [manager]);
  return (
    <>
      <h1>Projects</h1>
      <Err msg={loadErr} />
      <Table head={["Name", "Status", "Manager", "Start", "End", "Budget"]} empty={rows.length ? undefined : "No projects yet."}>
        {rows.map((p) => (
          <tr key={p.id}><td><Link href={`/projects/${p.id}`}>{p.name}</Link></td><td><Pill text={p.status} /></td><td>{p.manager ?? "–"}</td><td>{day(p.start_date)}</td><td>{day(p.end_date)}</td><td className="num">{xaf(p.budget)}</td></tr>
        ))}
      </Table>
      {manager && (
        <>
          <h2>New project</h2>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(() => raw.post("/v1/projects", { tenant_id: f.tenant_id, name: f.name, manager: f.manager, start_date: f.start_date || undefined, end_date: f.end_date || undefined, budget: f.budget ? Math.round(Number(f.budget)) : undefined })); }}>
            <Field id="pr-client" label="Client">
              <select id="pr-client" required value={f.tenant_id} onChange={(e) => setF({ ...f, tenant_id: e.target.value })}>
                <option value="">Select…</option>
                {clients.map((c) => <option key={c.tenant_id} value={c.tenant_id}>{c.legal_name}</option>)}
              </select>
            </Field>
            <Field id="pr-name" label="Name"><input id="pr-name" required value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} /></Field>
            <Field id="pr-mgr" label="Project manager (account id)"><input id="pr-mgr" value={f.manager} onChange={(e) => setF({ ...f, manager: e.target.value })} /></Field>
            <div className="grid2">
              <Field id="pr-start" label="Start"><input id="pr-start" type="date" value={f.start_date} onChange={(e) => setF({ ...f, start_date: e.target.value })} /></Field>
              <Field id="pr-end" label="End"><input id="pr-end" type="date" value={f.end_date} onChange={(e) => setF({ ...f, end_date: e.target.value })} /></Field>
            </div>
            <Field id="pr-budget" label="Budget (XAF)"><input id="pr-budget" type="number" min="0" step="1" value={f.budget} onChange={(e) => setF({ ...f, budget: e.target.value })} /></Field>
            <Err msg={act.err} />
            <button className="btn primary" disabled={act.busy}>Create</button>
          </form>
        </>
      )}
    </>
  );
}
