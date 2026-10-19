"use client";
import { useEffect, useState } from "react";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { day } from "@/lib/format";
import { Err, Field, Pill, Table, useAction } from "@/components/ui";

interface TS { id: string; actor: string; work_date: string; hours: number; note: string; status: string; approved_by: string | null }

export default function Timesheets() {
  const me = useMe();
  const reviewer = me.roles.some((r) => ["manager", "ceo", "hr", "finance"].includes(r));
  const approver = me.roles.some((r) => ["manager", "ceo"].includes(r));
  const [all, setAll] = useState(false);
  const [rows, setRows] = useState<TS[]>([]);
  const [f, setF] = useState({ work_date: new Date().toISOString().slice(0, 10), hours: "8", note: "" });
  const [loadErr, setLoadErr] = useState("");
  const load = () => raw.get<{ data: TS[] }>(`/v1/timesheets${all ? "?all=1" : ""}`).then((r) => setRows(r.data)).catch((e) => setLoadErr(e.message));
  const act = useAction(load);
  useEffect(() => { void load(); }, [all]); // eslint-disable-line react-hooks/exhaustive-deps
  return (
    <>
      <h1>Timesheets</h1>
      {reviewer && <label className="check"><input type="checkbox" checked={all} onChange={(e) => setAll(e.target.checked)} /> Show everyone's</label>}
      <Err msg={loadErr || act.err} />
      <Table head={["Date", ...(all ? ["Who"] : []), "Hours", "Note", "Status", ""]} empty={rows.length ? undefined : "No time logged."}>
        {rows.map((t) => (
          <tr key={t.id}>
            <td>{day(t.work_date)}</td>{all && <td>{t.actor}</td>}<td className="num">{t.hours}</td><td className="wrap">{t.note}</td>
            <td><Pill text={t.status} tone={t.status === "approved" ? "ok" : undefined} /></td>
            <td>
              {t.actor === me.sub && t.status === "draft" && <button className="btn" disabled={act.busy} onClick={() => act.run(() => raw.post(`/v1/timesheets/${t.id}/submit`))}>Submit</button>}
              {approver && t.actor !== me.sub && t.status === "submitted" && <button className="btn primary" disabled={act.busy} onClick={() => act.run(() => raw.post(`/v1/timesheets/${t.id}/approve`))}>Approve</button>}
            </td>
          </tr>
        ))}
      </Table>
      <h2>Log time</h2>
      <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post("/v1/timesheets", { work_date: f.work_date, hours: Number(f.hours), note: f.note }); setF({ ...f, note: "" }); }); }}>
        <div className="grid2">
          <Field id="ts-date" label="Date"><input id="ts-date" type="date" required value={f.work_date} onChange={(e) => setF({ ...f, work_date: e.target.value })} /></Field>
          <Field id="ts-hours" label="Hours"><input id="ts-hours" type="number" min="0.25" max="24" step="0.25" required value={f.hours} onChange={(e) => setF({ ...f, hours: e.target.value })} /></Field>
        </div>
        <Field id="ts-note" label="What did you work on?"><input id="ts-note" value={f.note} onChange={(e) => setF({ ...f, note: e.target.value })} /></Field>
        <button className="btn primary" disabled={act.busy}>Add</button>
      </form>
    </>
  );
}
