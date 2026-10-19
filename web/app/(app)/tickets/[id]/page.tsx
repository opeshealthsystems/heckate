"use client";
import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import { useMe } from "@/lib/session";
import type { Comment, Status, Ticket } from "@/lib/types";
import { SeverityBadge, SlaChip, StatusBadge } from "@/components/Badges";

export default function TicketPage() {
  const { id } = useParams<{ id: string }>();
  const me = useMe();
  const staff = me.scope === "staff";
  const [t, setT] = useState<Ticket | null>(null);
  const [comments, setComments] = useState<Comment[]>([]);
  const [body, setBody] = useState("");
  const [internal, setInternal] = useState(false);
  const [assignee, setAssignee] = useState("");
  const [rating, setRating] = useState(5);
  const [err, setErr] = useState("");
  const reload = useCallback(async () => {
    try {
      const [tk, cs] = await Promise.all([api.ticket(id), api.comments(id)]);
      setT(tk);
      setComments(cs.data);
      setAssignee(tk.assignee ?? "");
    } catch (e) {
      setErr((e as Error).message);
    }
  }, [id]);
  useEffect(() => { void reload(); }, [reload]);
  const run = (p: Promise<unknown>) => p.then(reload).catch((e) => setErr(e.message));
  if (err && !t) return <p className="error">{err}</p>;
  if (!t) return <p className="muted">Loading…</p>;
  const move = (s: Status) => run(api.updateTicket(id, { status: s }, `W/"${t.version}"`));
  return (
    <>
      <h1>{t.subject}</h1>
      <p className="meta">
        <SeverityBadge s={t.severity} /> <StatusBadge s={t.status} /> <SlaChip due={t.sla_due_at} done={!!t.resolved_at} breached={t.sla_breached} />
        {" "}· opened {new Date(t.created_at).toLocaleString()} · assignee {t.assignee ?? "unassigned"}
      </p>
      {t.description && <p className="card pre">{t.description}</p>}
      {err && <p className="error">{err}</p>}

      <div className="actions">
        {staff && t.status === "open" && <button className="btn" onClick={() => move("in_progress")}>Start work</button>}
        {staff && (t.status === "open" || t.status === "in_progress") && <button className="btn primary" onClick={() => move("resolved")}>Mark resolved</button>}
        {t.status === "resolved" && <button className="btn primary" onClick={() => move("closed")}>{staff ? "Close" : "Confirm resolved"}</button>}
        {t.status === "resolved" && <button className="btn" onClick={() => move("in_progress")}>Reopen</button>}
      </div>
      {staff && t.status !== "closed" && (
        <form className="inline" onSubmit={(e) => { e.preventDefault(); void run(api.updateTicket(id, { assignee })); }}>
          <label htmlFor="as">Assign to</label>
          <input id="as" value={assignee} onChange={(e) => setAssignee(e.target.value)} />
          <button className="btn">Save</button>
        </form>
      )}

      <h2>Conversation</h2>
      <ul className="thread">
        {comments.map((c) => (
          <li key={c.id} className={c.internal ? "internal" : ""}>
            <div className="muted small">{c.author} · {new Date(c.created_at).toLocaleString()}{c.internal && " · internal note"}</div>
            <div className="pre">{c.body}</div>
          </li>
        ))}
        {!comments.length && <li className="muted">No replies yet.</li>}
      </ul>
      {t.status !== "closed" && (
        <form
          className="form"
          onSubmit={(e) => {
            e.preventDefault();
            void run(api.addComment(id, body, internal).then(() => setBody("")));
          }}
        >
          <label htmlFor="reply">Reply</label>
          <textarea id="reply" rows={3} required value={body} onChange={(e) => setBody(e.target.value)} />
          {staff && (
            <label className="check"><input type="checkbox" checked={internal} onChange={(e) => setInternal(e.target.checked)} /> Internal note (hidden from client)</label>
          )}
          <button className="btn primary">Send</button>
        </form>
      )}

      {!staff && (t.status === "resolved" || t.status === "closed") && (
        <section className="card">
          <h2>Rate this support</h2>
          {t.rating ? (
            <p>You rated this {t.rating}/5. Thank you.</p>
          ) : (
            <form className="inline" onSubmit={(e) => { e.preventDefault(); void run(api.rate(id, rating, "")); }}>
              <label htmlFor="rt">Rating</label>
              <select id="rt" value={rating} onChange={(e) => setRating(Number(e.target.value))}>
                {[5, 4, 3, 2, 1].map((n) => <option key={n}>{n}</option>)}
              </select>
              <button className="btn primary">Submit</button>
            </form>
          )}
        </section>
      )}
    </>
  );
}
