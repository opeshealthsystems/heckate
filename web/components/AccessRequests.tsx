"use client";
import { useCallback, useEffect, useState } from "react";
import { raw } from "@/lib/api";
import { when } from "@/lib/format";
import { Err, Pill, useAction } from "./ui";

interface Req { id: string; ref: string; title: string; name: string; email: string; organisation: string; reason: string; status: "pending" | "approved" | "denied"; requested_at: string; decided_by: string | null; decided_at: string | null; decision_note: string }
interface Granted { link: string; expires_at: string; max_downloads: number }

/** People who verified a document and asked to see it. A manager grants a time-limited link or refuses, with a note. */
export function AccessRequests() {
  const [rows, setRows] = useState<Req[]>([]);
  const [filter, setFilter] = useState("pending");
  const [note, setNote] = useState<Record<string, string>>({});
  const [days, setDays] = useState<Record<string, string>>({});
  const [granted, setGranted] = useState<(Granted & { id: string }) | null>(null);
  const load = useCallback(() => raw.get<{ data: Req[] }>(`/v1/access-requests${filter ? `?status=${filter}` : ""}`).then((r) => setRows(r.data)).catch(() => {}), [filter]);
  useEffect(() => { void load(); }, [load]);
  const act = useAction(load);
  return (
    <>
      <div className="filters">
        <label className="sr" htmlFor="ar-filter">Show</label>
        <select id="ar-filter" value={filter} onChange={(e) => setFilter(e.target.value)}>
          <option value="pending">Waiting for a decision</option><option value="approved">Approved</option><option value="denied">Refused</option><option value="">All</option>
        </select>
      </div>
      <Err msg={act.err} />
      {granted && (
        <div className="notice ok" role="status">
          <strong>Link created. Copy it now: it is shown only once.</strong>
          <p><code className="hash">{granted.link}</code></p>
          <p className="small">Works {granted.max_downloads} time(s) and expires {when(granted.expires_at)}. The platform does not send email yet, so send this link to the requester yourself.</p>
          <button className="btn" onClick={() => void navigator.clipboard?.writeText(granted.link)}>Copy link</button> <button className="btn" onClick={() => setGranted(null)}>Done</button>
        </div>
      )}
      {!rows.length && <p className="muted">No requests here.</p>}
      <ul className="requests">
        {rows.map((r) => (
          <li key={r.id} className="card">
            <div className="row">
              <strong>{r.name}</strong>
              <Pill text={r.status} tone={r.status === "approved" ? "ok" : r.status === "denied" ? "bad" : "warn"} />
            </div>
            <p className="muted small">{r.email}{r.organisation && ` · ${r.organisation}`} · {when(r.requested_at)}</p>
            <p><code>{r.ref}</code> · {r.title}</p>
            <blockquote>{r.reason}</blockquote>
            {r.status === "pending" ? (
              <div className="inline">
                <label className="sr" htmlFor={`n-${r.id}`}>Note</label>
                <input id={`n-${r.id}`} placeholder="Note (optional)" maxLength={500} value={note[r.id] ?? ""} onChange={(e) => setNote({ ...note, [r.id]: e.target.value })} />
                <label className="sr" htmlFor={`d-${r.id}`}>Days the link works</label>
                <select id={`d-${r.id}`} value={days[r.id] ?? "7"} onChange={(e) => setDays({ ...days, [r.id]: e.target.value })}>{["1", "3", "7", "14", "30"].map((d) => <option key={d} value={d}>{d} day{d === "1" ? "" : "s"}</option>)}</select>
                <button className="btn primary" disabled={act.busy} onClick={() => act.run(async () => { const g = await raw.post<Granted>(`/v1/access-requests/${r.id}/approve`, { days: Number(days[r.id] ?? 7), note: note[r.id] ?? "" }); setGranted({ ...g, id: r.id }); })}>Approve and create link</button>
                <button className="btn" disabled={act.busy} onClick={() => act.run(() => raw.post(`/v1/access-requests/${r.id}/deny`, { note: note[r.id] ?? "" }))}>Refuse</button>
              </div>
            ) : (
              <p className="muted small">{r.status === "approved" ? "Approved" : "Refused"} by {r.decided_by} on {when(r.decided_at)}{r.decision_note && ` · ${r.decision_note}`}</p>
            )}
          </li>
        ))}
      </ul>
    </>
  );
}
