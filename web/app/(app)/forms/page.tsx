"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { api, raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { day } from "@/lib/format";
import type { Client } from "@/lib/types";
import { FORM_TEMPLATES, type FormField } from "@/lib/form-templates";
import { ExportButton } from "@/components/ExportButton";
import { Err, Field, Pill, Table, useAction } from "@/components/ui";

interface Row { id: string; client: string; title: string; form_code: string; status: string; sent_at: string; due_date: string | null; submitted_at: string | null; on_behalf: boolean }

const tone = (s: string) => (s === "accepted" ? "ok" : s === "returned" ? "warn" : s === "cancelled" ? "bad" : undefined);

export default function Forms() {
  const me = useMe();
  const staff = me.scope === "staff";
  const [rows, setRows] = useState<Row[]>([]);
  const [clients, setClients] = useState<Client[]>([]);
  const [loadErr, setLoadErr] = useState("");
  const [f, setF] = useState({ tenant_id: "", template: FORM_TEMPLATES[0].id, title: FORM_TEMPLATES[0].title, form_code: "", message: "", due_date: "" });
  const [fields, setFields] = useState<FormField[]>(FORM_TEMPLATES[0].fields);
  const load = () => raw.get<{ data: Row[] }>("/v1/form-requests").then((r) => setRows(r.data)).catch((e) => setLoadErr(e.message));
  const act = useAction(async () => { await load(); });
  useEffect(() => { void load(); if (staff) api.clients().then((r) => setClients(r.data)).catch(() => {}); }, [staff]);
  const waiting = rows.filter((r) => r.status === "sent" || r.status === "returned");
  return (
    <>
      <h1>{staff ? "Client forms" : "Forms to fill"}</h1>
      <Err msg={loadErr} />
      {!staff && waiting.length > 0 && <p className="notice" role="status">{waiting.length} form(s) waiting for you.</p>}
      {staff && <div className="actions"><ExportButton path="/v1/exports/form-requests" filename="client-forms" label="Export CSV" /></div>}
      <Table head={["Form", ...(staff ? ["Client"] : []), "Status", "Sent", "Due", "Submitted"]} empty={rows.length ? undefined : staff ? "No forms sent yet." : "Nothing to fill in."}>
        {rows.map((r) => (
          <tr key={r.id}>
            <td><Link href={`/forms/${r.id}`}>{r.title}</Link>{r.form_code && <span className="muted small"> · {r.form_code}</span>}</td>
            {staff && <td>{r.client}</td>}
            <td><Pill text={r.status} tone={tone(r.status)} />{r.on_behalf && <span className="muted small"> · from paper</span>}</td>
            <td>{day(r.sent_at)}</td><td>{day(r.due_date)}</td><td>{day(r.submitted_at)}</td>
          </tr>
        ))}
      </Table>
      {staff && (
        <>
          <h2>Send a form</h2>
          <p className="muted small">The client fills it in online from their portal, or you can key in a paper form they returned. Every change is kept in the form's history.</p>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => {
            const created = await raw.post<{ id: string }>("/v1/form-requests", { tenant_id: f.tenant_id, title: f.title, form_code: f.form_code, message: f.message, due_date: f.due_date || undefined, fields });
            location.assign(`/forms/${created.id}`);
          }); }}>
            <Field id="fr-client" label="Client">
              <select id="fr-client" required value={f.tenant_id} onChange={(e) => setF({ ...f, tenant_id: e.target.value })}>
                <option value="">Select…</option>
                {clients.map((c) => <option key={c.tenant_id} value={c.tenant_id}>{c.legal_name}</option>)}
              </select>
            </Field>
            <Field id="fr-tpl" label="Start from">
              <select id="fr-tpl" value={f.template} onChange={(e) => { const t = FORM_TEMPLATES.find((x) => x.id === e.target.value)!; setF({ ...f, template: t.id, title: t.title }); setFields(t.fields); }}>
                {FORM_TEMPLATES.map((t) => <option key={t.id} value={t.id}>{t.title}</option>)}
              </select>
            </Field>
            <div className="grid2">
              <Field id="fr-title" label="Title"><input id="fr-title" required value={f.title} onChange={(e) => setF({ ...f, title: e.target.value })} /></Field>
              <Field id="fr-code" label="Pack form code (optional)"><input id="fr-code" value={f.form_code} onChange={(e) => setF({ ...f, form_code: e.target.value })} placeholder="e.g. HS-FRM-14" /></Field>
            </div>
            <Field id="fr-msg" label="Message to the client"><textarea id="fr-msg" rows={2} value={f.message} onChange={(e) => setF({ ...f, message: e.target.value })} /></Field>
            <Field id="fr-due" label="Please return by"><input id="fr-due" type="date" value={f.due_date} onChange={(e) => setF({ ...f, due_date: e.target.value })} /></Field>
            <fieldset className="lines">
              <legend>Questions ({fields.length})</legend>
              {fields.map((q, i) => (
                <div className="line" key={q.key}>
                  <input aria-label={`Question ${i + 1}`} value={q.label} onChange={(e) => setFields(fields.map((x, j) => (j === i ? { ...x, label: e.target.value } : x)))} />
                  <label className="check small"><input type="checkbox" checked={!!q.required} onChange={(e) => setFields(fields.map((x, j) => (j === i ? { ...x, required: e.target.checked } : x)))} /> required</label>
                  <span className="muted small">{q.type}</span>
                  <button type="button" className="btn ghost" aria-label={`Remove question ${i + 1}`} onClick={() => setFields(fields.filter((_, j) => j !== i))}>Remove</button>
                </div>
              ))}
            </fieldset>
            <Err msg={act.err} />
            <button className="btn primary" disabled={act.busy || fields.length === 0}>Send to client</button>
          </form>
        </>
      )}
    </>
  );
}
