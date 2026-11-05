"use client";
import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { day, when } from "@/lib/format";
import type { Answers, FormField } from "@/lib/form-templates";
import { FormFiller } from "@/components/FormFiller";
import { Err, Field, Pill, Table, useAction } from "@/components/ui";

interface Req {
  id: string; client: string; title: string; form_code: string; message: string; fields: FormField[]; answers: Answers; status: string; due_date: string | null;
  sent_by: string; sent_at: string; submitted_by: string | null; submitted_at: string | null; on_behalf: boolean; reviewed_by: string | null; review_note: string | null;
  history: { action: string; actor: string; note: string | null; at: string }[];
}

const show = (v: unknown) => (v === true ? "Yes" : v === false ? "No" : v === undefined || v === null || v === "" ? "–" : String(v));

export default function FormPage() {
  const { id } = useParams<{ id: string }>();
  const me = useMe();
  const staff = me.scope === "staff";
  const reviewer = me.roles.some((r) => ["ceo", "manager", "sales", "support", "engineer"].includes(r));
  const [r, setR] = useState<Req | null>(null);
  const [loadErr, setLoadErr] = useState("");
  const [note, setNote] = useState("");
  const [paper, setPaper] = useState(false);
  const load = useCallback(() => raw.get<Req>(`/v1/form-requests/${id}`).then(setR).catch((e) => setLoadErr(e.message)), [id]);
  useEffect(() => { void load(); }, [load]);
  const act = useAction(load);
  if (loadErr && !r) return <p className="error">{loadErr}</p>;
  if (!r) return <p className="muted">Loading…</p>;
  const open = r.status === "sent" || r.status === "returned";
  return (
    <>
      <h1>{r.title}</h1>
      <p className="meta"><Pill text={r.status} /> · {r.client} · sent {day(r.sent_at)} by {r.sent_by}{r.due_date && ` · please return by ${day(r.due_date)}`}{r.form_code && ` · ${r.form_code}`}</p>
      {r.message && <p className="card">{r.message}</p>}
      {r.status === "returned" && r.review_note && <p className="notice" role="status">Returned for changes: {r.review_note}</p>}
      <Err msg={act.err} />
      {open && (!staff || paper) && (
        <>
          {staff && <p className="notice">Keying in a paper form the client returned. It will be recorded as entered by you from paper.</p>}
          <FormFiller key={r.status + r.id} fields={r.fields} initial={r.answers} busy={act.busy} error=""
            submitLabel={staff ? "Record as submitted" : "Submit"}
            onSave={staff ? undefined : (a) => act.run(() => raw.put(`/v1/form-requests/${id}/answers`, { answers: a }))}
            onSubmit={(a) => act.run(() => raw.post(`/v1/form-requests/${id}/${staff ? "enter" : "submit"}`, { answers: a }))} />
        </>
      )}
      {open && staff && !paper && (
        <div className="actions">
          <button className="btn" onClick={() => setPaper(true)}>Key in a paper form</button>
          <button className="btn" disabled={act.busy} onClick={() => act.run(() => raw.post(`/v1/form-requests/${id}/cancel`))}>Cancel this request</button>
        </div>
      )}
      {!open && (
        <>
          <h2>Answers</h2>
          <Table head={["Question", "Answer"]}>
            {r.fields.map((q) => <tr key={q.key}><td>{q.label}</td><td className="wrap">{show(r.answers[q.key])}</td></tr>)}
          </Table>
          <p className="muted small">{r.submitted_at && `Submitted ${when(r.submitted_at)} by ${r.submitted_by}${r.on_behalf ? " (keyed in from a paper form)" : ""}.`}{r.reviewed_by && ` Reviewed by ${r.reviewed_by}.`}</p>
        </>
      )}
      {r.status === "submitted" && reviewer && (
        <div className="card form">
          <Field id="rv-note" label="Note to the client (required to return the form)"><textarea id="rv-note" rows={2} value={note} onChange={(e) => setNote(e.target.value)} /></Field>
          <div className="actions">
            <button className="btn primary" disabled={act.busy} onClick={() => act.run(() => raw.post(`/v1/form-requests/${id}/accept`, { note }))}>Accept</button>
            <button className="btn" disabled={act.busy || !note.trim()} onClick={() => act.run(() => raw.post(`/v1/form-requests/${id}/return`, { note }))}>Return for changes</button>
          </div>
        </div>
      )}
      {staff && (
        <>
          <h2>History</h2>
          <Table head={["When", "What", "Who", "Note"]}>
            {r.history.map((h, i) => <tr key={i}><td>{when(h.at)}</td><td>{h.action.replace(/_/g, " ")}</td><td>{h.actor}</td><td className="wrap">{h.note ?? ""}</td></tr>)}
          </Table>
        </>
      )}
    </>
  );
}
