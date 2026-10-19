"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { useMe } from "@/lib/session";
import type { Client, Severity } from "@/lib/types";

const SEVERITIES: [Severity, string][] = [
  ["P1", "P1 – Critical: service down"],
  ["P2", "P2 – High: major impact"],
  ["P3", "P3 – Medium: degraded"],
  ["P4", "P4 – Low: question or minor"],
];

export default function NewTicket() {
  const me = useMe();
  const router = useRouter();
  const staff = me.scope === "staff";
  const [clients, setClients] = useState<Client[]>([]);
  const [f, setF] = useState({ tenant_id: "", subject: "", description: "", severity: "P3" as Severity });
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  useEffect(() => {
    if (staff) api.clients().then((r) => setClients(r.data)).catch(() => {});
  }, [staff]);
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      const t = await api.createTicket({ ...f, tenant_id: staff ? f.tenant_id : undefined, channel: "portal" });
      router.push(`/tickets/${t.id}`);
    } catch (x) {
      setErr((x as Error).message);
      setBusy(false);
    }
  }
  return (
    <>
      <h1>{staff ? "New ticket" : "Report an incident"}</h1>
      <form className="card form" onSubmit={submit}>
        {staff && (
          <>
            <label htmlFor="client">Client</label>
            <select id="client" required value={f.tenant_id} onChange={(e) => setF({ ...f, tenant_id: e.target.value })}>
              <option value="">Select…</option>
              {clients.map((c) => <option key={c.tenant_id} value={c.tenant_id}>{c.legal_name}</option>)}
            </select>
          </>
        )}
        <label htmlFor="sev">Severity</label>
        <select id="sev" value={f.severity} onChange={(e) => setF({ ...f, severity: e.target.value as Severity })}>
          {SEVERITIES.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
        </select>
        {f.severity === "P1" && <p className="notice">A P1 pages our on-call engineer immediately.</p>}
        <label htmlFor="subject">Subject</label>
        <input id="subject" required maxLength={200} value={f.subject} onChange={(e) => setF({ ...f, subject: e.target.value })} />
        <label htmlFor="desc">What is happening?</label>
        <textarea id="desc" rows={6} value={f.description} onChange={(e) => setF({ ...f, description: e.target.value })} />
        {err && <p className="error">{err}</p>}
        <button className="btn primary" disabled={busy}>{busy ? "Sending…" : "Submit"}</button>
      </form>
    </>
  );
}
