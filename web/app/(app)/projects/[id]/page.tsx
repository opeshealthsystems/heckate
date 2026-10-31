"use client";
import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { day, xaf } from "@/lib/format";
import { Err, Field, Pill, Table, useAction } from "@/components/ui";
import { ProjectProgress } from "@/components/ProjectProgress";

interface Task { id: string; title: string; status: string; assignee: string | null; due_date: string | null }
interface Project { id: string; name: string; status: string; manager: string | null; budget: number | null; start_date: string | null; end_date: string | null; tasks: Task[] }

export default function ProjectPage() {
  const { id } = useParams<{ id: string }>();
  const me = useMe();
  const canEdit = me.roles.some((r) => ["manager", "ceo", "engineer"].includes(r));
  const isManager = me.roles.some((r) => ["manager", "ceo"].includes(r));
  const [p, setP] = useState<Project | null>(null);
  const [title, setTitle] = useState("");
  const [loadErr, setLoadErr] = useState("");
  const load = useCallback(() => raw.get<Project>(`/v1/projects/${id}`).then(setP).catch((e) => setLoadErr(e.message)), [id]);
  useEffect(() => { void load(); }, [load]);
  const act = useAction(load);
  if (loadErr && !p) return <p className="error">{loadErr}</p>;
  if (!p) return <p className="muted">Loading…</p>;
  return (
    <>
      <h1>{p.name}</h1>
      <p className="meta"><Pill text={p.status} /> · manager {p.manager ?? "–"} · {day(p.start_date)} to {day(p.end_date)} · budget {xaf(p.budget)}</p>
      {isManager && (
        <div className="actions">
          {["planning", "active", "on_hold", "done", "cancelled"].filter((s) => s !== p.status).map((s) => (
            <button key={s} className="btn" disabled={act.busy} onClick={() => act.run(() => raw.patch(`/v1/projects/${id}`, { status: s }))}>Mark {s.replace("_", " ")}</button>
          ))}
        </div>
      )}
      <Err msg={act.err} />
      <ProjectProgress id={id} />
      <h2>Tasks</h2>
      <Table head={["Task", "Assignee", "Due", "Status"]} empty={(p.tasks ?? []).length ? undefined : "No tasks yet."}>
        {(p.tasks ?? []).map((t) => (
          <tr key={t.id}>
            <td>{t.title}</td><td>{t.assignee ?? "–"}</td><td>{day(t.due_date)}</td>
            <td>
              {canEdit ? (
                <select aria-label={`Status of ${t.title}`} value={t.status} onChange={(e) => act.run(() => raw.patch(`/v1/tasks/${t.id}`, { status: e.target.value }))}>
                  {["todo", "doing", "done"].map((s) => <option key={s}>{s}</option>)}
                </select>
              ) : <Pill text={t.status} />}
            </td>
          </tr>
        ))}
      </Table>
      {canEdit && (
        <form className="inline" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post(`/v1/projects/${id}/tasks`, { title }); setTitle(""); }); }}>
          <Field id="t-title" label="Add a task"><input id="t-title" required value={title} onChange={(e) => setTitle(e.target.value)} /></Field>
          <button className="btn primary" disabled={act.busy}>Add</button>
        </form>
      )}
    </>
  );
}
