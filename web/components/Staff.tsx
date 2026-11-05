"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { day } from "@/lib/format";
import { ExportButton } from "./ExportButton";
import { Err, Field, Pill, Table, useAction } from "./ui";

interface Emp { id: string; actor: string; full_name: string; position: string; department: string; hire_date: string; status: string }

/** The staff register for HR and management, with a way to add a person. Each name opens the employee file. */
export function Staff() {
  const me = useMe();
  const hr = me.roles.some((r) => ["hr", "ceo"].includes(r));
  const [rows, setRows] = useState<Emp[]>([]);
  const [err, setErr] = useState("");
  const [f, setF] = useState({ actor: "", full_name: "", position: "", department: "", hire_date: "" });
  const load = () => raw.get<{ data: Emp[] }>("/v1/employees").then((r) => setRows(r.data)).catch((e) => setErr(e.message));
  useEffect(() => { void load(); }, []);
  const act = useAction(async () => { setF({ actor: "", full_name: "", position: "", department: "", hire_date: "" }); await load(); });
  return (
    <>
      <Err msg={err} />
      <div className="actions"><ExportButton path="/v1/exports/employees" filename="staff-register" label="Export staff register CSV" /></div>
      <Table head={["Name", "Position", "Department", "Hired", "Status"]} empty={rows.length ? undefined : "No employee records yet."}>
        {rows.map((e) => <tr key={e.id}><td><Link href={`/hr/employee/${e.id}`}>{e.full_name}</Link></td><td>{e.position || "–"}</td><td>{e.department || "–"}</td><td>{day(e.hire_date)}</td><td><Pill text={e.status} /></td></tr>)}
      </Table>
      {hr && (
        <>
          <h2>Add an employee</h2>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(() => raw.post("/v1/employees", f)); }}>
            <div className="grid2">
              <Field id="em-name" label="Full name"><input id="em-name" required value={f.full_name} onChange={(e) => setF({ ...f, full_name: e.target.value })} /></Field>
              <Field id="em-actor" label="Sign-in account id"><input id="em-actor" required value={f.actor} onChange={(e) => setF({ ...f, actor: e.target.value })} /></Field>
            </div>
            <div className="grid2">
              <Field id="em-pos" label="Position"><input id="em-pos" value={f.position} onChange={(e) => setF({ ...f, position: e.target.value })} /></Field>
              <Field id="em-dep" label="Department"><input id="em-dep" value={f.department} onChange={(e) => setF({ ...f, department: e.target.value })} /></Field>
            </div>
            <Field id="em-hire" label="Hire date"><input id="em-hire" type="date" required value={f.hire_date} onChange={(e) => setF({ ...f, hire_date: e.target.value })} /></Field>
            <Err msg={act.err} />
            <button className="btn primary" disabled={act.busy}>Add</button>
          </form>
        </>
      )}
    </>
  );
}
