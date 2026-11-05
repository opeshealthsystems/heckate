"use client";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { AccountRequest } from "@/lib/types";

// People who asked for an account from the mobile sign-in screen. Approving creates the account in the
// approver's organisation and emails the person a link to set their password.
export default function AccessRequests() {
  const [status, setStatus] = useState("pending");
  const [rows, setRows] = useState<AccountRequest[]>([]);
  const [roles, setRoles] = useState<string[]>([]);
  const [pick, setPick] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState("");
  const [err, setErr] = useState("");
  const [msg, setMsg] = useState("");
  const load = () => api.accountRequests(status).then((r) => { setRows(r.data); setRoles(r.roles); }).catch((e) => setErr(e.message));
  useEffect(() => { void load(); }, [status]); // eslint-disable-line react-hooks/exhaustive-deps

  const act = async (q: AccountRequest, approve: boolean) => {
    setErr(""); setMsg("");
    if (approve && !pick[q.id]) { setErr(`Choose a role for ${q.name} first.`); return; }
    let note = "";
    if (!approve) {
      const n = window.prompt(`Reject ${q.name}? Optional internal note:`);
      if (n === null) return;
      note = n;
    }
    setBusy(q.id);
    try {
      if (approve) { await api.approveAccountRequest(q.id, pick[q.id]); setMsg(`${q.email} approved. They have been emailed a link to set their password.`); }
      else { await api.rejectAccountRequest(q.id, note); setMsg(`${q.email} rejected.`); }
      await load();
    } catch (e) { setErr(e instanceof Error ? e.message : String(e)); }
    setBusy("");
  };

  return (
    <>
      <h1>Access requests</h1>
      <p className="muted">People who asked for an account from the mobile app. Approving creates their account in your organisation.</p>
      <label>Show{" "}
        <select value={status} onChange={(e) => setStatus(e.target.value)} aria-label="Filter by status">
          <option value="pending">Pending</option><option value="approved">Approved</option><option value="rejected">Rejected</option><option value="all">All</option>
        </select>
      </label>
      {err && <p className="error">{err}</p>}
      {msg && <p>{msg}</p>}
      <div className="scroll">
        <table>
          <thead><tr><th>Name</th><th>Contact</th><th>Organisation</th><th>Reason</th><th>Received</th><th>{status === "pending" ? "Decision" : "Status"}</th></tr></thead>
          <tbody>
            {rows.map((q) => (
              <tr key={q.id}>
                <td>{q.name}</td>
                <td>{q.email}{q.phone && <><br />{q.phone}</>}</td>
                <td>{q.company ?? "–"}</td>
                <td className="wrap">{q.reason || "–"}</td>
                <td>{new Date(q.created_at).toLocaleString()}</td>
                <td>
                  {q.status === "pending" ? (
                    <>
                      <select aria-label={`Role for ${q.name}`} value={pick[q.id] ?? ""} onChange={(e) => setPick({ ...pick, [q.id]: e.target.value })}>
                        <option value="">Role…</option>
                        {roles.map((r) => <option key={r}>{r}</option>)}
                      </select>{" "}
                      <button disabled={busy === q.id} onClick={() => act(q, true)}>Approve</button>{" "}
                      <button disabled={busy === q.id} onClick={() => act(q, false)}>Reject</button>
                    </>
                  ) : (
                    <>{q.status}{q.role && ` · ${q.role}`}{q.note && <><br /><span className="muted">{q.note}</span></>}</>
                  )}
                </td>
              </tr>
            ))}
            {!rows.length && !err && <tr><td colSpan={6} className="muted">No {status === "all" ? "" : status} requests.</td></tr>}
          </tbody>
        </table>
      </div>
    </>
  );
}
