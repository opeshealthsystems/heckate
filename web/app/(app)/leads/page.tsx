"use client";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { Lead } from "@/lib/types";

const STAGES = ["new", "contacted", "qualified", "proposal", "won", "lost"];

export default function Leads() {
  const [rows, setRows] = useState<Lead[]>([]);
  const [err, setErr] = useState("");
  const load = () => api.leads().then((r) => setRows(r.data)).catch((e) => setErr(e.message));
  useEffect(() => { void load(); }, []);
  return (
    <>
      <h1>Leads</h1>
      {err && <p className="error">{err}</p>}
      <div className="scroll">
        <table>
          <thead><tr><th>Name</th><th>Company</th><th>Contact</th><th>Message</th><th>Stage</th></tr></thead>
          <tbody>
            {rows.map((l) => (
              <tr key={l.id}>
                <td>{l.name}</td><td>{l.company ?? "–"}</td>
                <td>{l.email}{l.phone && <><br />{l.phone}</>}</td>
                <td className="wrap">{l.message}</td>
                <td>
                  <select aria-label={`Stage for ${l.name}`} value={l.stage} onChange={(e) => api.updateLead(l.id, { stage: e.target.value }).then(load).catch((x) => setErr(x.message))}>
                    {STAGES.map((s) => <option key={s}>{s}</option>)}
                  </select>
                </td>
              </tr>
            ))}
            {!rows.length && !err && <tr><td colSpan={5} className="muted">No leads yet.</td></tr>}
          </tbody>
        </table>
      </div>
    </>
  );
}
