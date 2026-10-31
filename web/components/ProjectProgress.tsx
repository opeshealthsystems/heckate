"use client";
import { useCallback, useEffect, useState } from "react";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { day, xaf } from "@/lib/format";
import { unpricedNote } from "@/lib/labour";
import { Err, Field, Pill, Table, useAction } from "./ui";

interface Milestone { id: string; title: string; due_date: string | null; status: string; late: boolean }
interface Cost { id: string; cost_date: string; category: string; description: string; amount: number }
export interface Progress {
  percent: number; tasks_done: number; tasks_total: number; milestones_done: number; milestones_total: number; late_milestones: number; behind_schedule: boolean;
  time_elapsed_percent: number | null; budget?: number | null; actual?: number; labour?: { cost: number; priced_hours: number; unpriced_hours: number }; remaining?: number | null; over_budget?: boolean; milestones: Milestone[]; costs?: Cost[];
}

export function ProgressBar({ percent, label }: { percent: number; label?: string }) {
  return (
    <div className="bar" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={percent} aria-label={label ?? "Progress"}><i style={{ width: `${percent}%` }} /></div>
  );
}

const CATEGORIES = ["labour", "materials", "travel", "subcontract", "other"];

/** Progress, milestones and budget against actual cost for one project. Clients see progress and milestones, never money. */
export function ProjectProgress({ id }: { id: string }) {
  const me = useMe();
  const manager = me.roles.some((r) => ["ceo", "manager"].includes(r));
  const field = me.roles.some((r) => ["ceo", "manager", "engineer"].includes(r));
  const coster = me.roles.some((r) => ["ceo", "manager", "finance"].includes(r));
  const [p, setP] = useState<Progress | null>(null);
  const [err, setErr] = useState("");
  const [ms, setMs] = useState({ title: "", due_date: "" });
  const [cost, setCost] = useState({ cost_date: new Date().toISOString().slice(0, 10), category: "materials", description: "", amount: "" });
  const load = useCallback(() => raw.get<Progress>(`/v1/projects/${id}/progress`).then(setP).catch((e) => setErr(e.message)), [id]);
  useEffect(() => { void load(); }, [load]);
  const act = useAction(load);
  if (!p) return <Err msg={err} />;
  return (
    <>
      <h2>Progress</h2>
      <div className="card">
        <p><strong>{p.percent}% done</strong> · {p.tasks_done} of {p.tasks_total} tasks · {p.milestones_done} of {p.milestones_total} milestones
          {p.time_elapsed_percent != null && ` · ${p.time_elapsed_percent}% of the time has passed`}
          {p.behind_schedule && <> <Pill text="behind schedule" tone="bad" /></>}</p>
        <ProgressBar percent={p.percent} label="Project progress" />
        {p.budget != null && p.actual != null && (
          <p className="small">Budget {xaf(p.budget)} · spent {xaf(p.actual)} · {p.over_budget ? <strong className="tl bad">over budget by {xaf(-(p.remaining ?? 0))}</strong> : <>{xaf(p.remaining ?? 0)} left</>}</p>
        )}
        {p.labour && (
          <p className="small">Labour from timesheets {xaf(p.labour.cost)} ({p.labour.priced_hours} priced hours, included in spent){p.labour.unpriced_hours > 0 && <> · <strong className="tl bad">{unpricedNote(p.labour.unpriced_hours)}</strong></>}</p>
        )}
        {p.budget != null && p.actual != null && p.budget > 0 && <ProgressBar percent={Math.min(100, Math.round((p.actual * 100) / p.budget))} label="Budget used" />}
      </div>
      <Err msg={act.err} />
      <h2>Milestones</h2>
      <Table head={["Milestone", "Due", "Status", ""]} empty={p.milestones.length ? undefined : "No milestones yet."}>
        {p.milestones.map((m) => (
          <tr key={m.id}>
            <td>{m.title}</td><td className={m.late ? "tl bad" : ""}>{day(m.due_date)}{m.late && " · late"}</td><td><Pill text={m.status === "done" ? "done" : "pending"} tone={m.status === "done" ? "ok" : undefined} /></td>
            <td>{field && m.status === "pending" && <button className="btn" disabled={act.busy} onClick={() => act.run(() => raw.post(`/v1/milestones/${m.id}/done`))}>Mark done</button>}</td>
          </tr>
        ))}
      </Table>
      {manager && (
        <form className="inline" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post(`/v1/projects/${id}/milestones`, { title: ms.title, due_date: ms.due_date || undefined }); setMs({ title: "", due_date: "" }); }); }}>
          <Field id="ms-title" label="Add a milestone"><input id="ms-title" required value={ms.title} onChange={(e) => setMs({ ...ms, title: e.target.value })} /></Field>
          <Field id="ms-due" label="Due"><input id="ms-due" type="date" value={ms.due_date} onChange={(e) => setMs({ ...ms, due_date: e.target.value })} /></Field>
          <button className="btn primary" disabled={act.busy}>Add</button>
        </form>
      )}
      {coster && p.costs && (
        <>
          <h2>Costs</h2>
          <Table head={["Date", "Category", "Description", "Amount", ""]} empty={p.costs.length ? undefined : "No costs recorded."}>
            {p.costs.map((c) => (
              <tr key={c.id}><td>{day(c.cost_date)}</td><td>{c.category}</td><td className="wrap">{c.description}</td><td className="num">{xaf(c.amount)}</td>
                <td><button className="btn ghost" disabled={act.busy} onClick={() => { if (confirm("Delete this cost entry?")) void act.run(() => raw.del(`/v1/costs/${c.id}`)); }}>Delete</button></td></tr>
            ))}
          </Table>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post(`/v1/projects/${id}/costs`, { ...cost, amount: Math.round(Number(cost.amount)) }); setCost({ ...cost, description: "", amount: "" }); }); }}>
            <div className="grid2">
              <Field id="co-date" label="Date"><input id="co-date" type="date" required value={cost.cost_date} onChange={(e) => setCost({ ...cost, cost_date: e.target.value })} /></Field>
              <Field id="co-cat" label="Category"><select id="co-cat" value={cost.category} onChange={(e) => setCost({ ...cost, category: e.target.value })}>{CATEGORIES.map((c) => <option key={c}>{c}</option>)}</select></Field>
            </div>
            <Field id="co-desc" label="Description"><input id="co-desc" required value={cost.description} onChange={(e) => setCost({ ...cost, description: e.target.value })} /></Field>
            <Field id="co-amt" label="Amount (XAF)"><input id="co-amt" type="number" min="1" step="1" required value={cost.amount} onChange={(e) => setCost({ ...cost, amount: e.target.value })} /></Field>
            <button className="btn primary" disabled={act.busy}>Record cost</button>
          </form>
        </>
      )}
    </>
  );
}
