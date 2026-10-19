"use client";
import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { day, xaf } from "@/lib/format";
import { Err, Field, Pill, Table, useAction } from "@/components/ui";

interface Emp { id: string; actor: string; full_name: string; position: string; department: string; hire_date: string; status: string }
interface Priv { date_of_birth: string | null; national_id: string; cnps_number: string; phone: string; personal_email: string; address: string; emergency_name: string; emergency_phone: string; bank_name: string; bank_account: string; updated_by: string }
interface Contract { id: string; type: string; position: string; start_date: string; end_date: string | null; probation_end: string | null; notice_days: number; weekly_hours: number; status: string; signed_on: string | null }
interface File { employee: Emp; private?: Priv; contracts: Contract[]; pay_history: { effective_from: string; base_salary: number; allowances: number }[]; leave: { entitlement: number; taken: number; balance: number } }

const blankPriv: Priv = { date_of_birth: "", national_id: "", cnps_number: "", phone: "", personal_email: "", address: "", emergency_name: "", emergency_phone: "", bank_name: "", bank_account: "", updated_by: "" };
const blankContract = { type: "CDI", position: "", start_date: "", end_date: "", probation_end: "", notice_days: "30", weekly_hours: "40", signed_on: "" };

export default function EmployeeFile() {
  const { id } = useParams<{ id: string }>();
  const me = useMe();
  const hr = me.roles.some((r) => ["hr", "ceo"].includes(r));
  const [f, setF] = useState<File | null>(null);
  const [loadErr, setLoadErr] = useState("");
  const [priv, setPriv] = useState<Priv>(blankPriv);
  const [c, setC] = useState(blankContract);
  const load = useCallback(() => raw.get<File>(`/v1/employees/${id}/file`).then((x) => { setF(x); setPriv({ ...blankPriv, ...(x.private ?? {}), date_of_birth: x.private?.date_of_birth?.slice(0, 10) ?? "" }); }).catch((e) => setLoadErr(e.message)), [id]);
  useEffect(() => { void load(); }, [load]);
  const act = useAction(load);
  if (loadErr && !f) return <p className="error">{loadErr}</p>;
  if (!f) return <p className="muted">Loading…</p>;
  const e = f.employee;
  const text = (k: keyof Priv, label: string, type = "text") => (
    <Field id={`pv-${k}`} label={label}><input id={`pv-${k}`} type={type} readOnly={!hr} value={String(priv[k] ?? "")} onChange={(ev) => setPriv({ ...priv, [k]: ev.target.value })} /></Field>
  );
  return (
    <>
      <p><Link href="/hr">← People</Link></p>
      <h1>{e.full_name}</h1>
      <p className="meta"><Pill text={e.status} /> · {e.position || "no position"} · {e.department || "no department"} · hired {day(e.hire_date)} · account <code>{e.actor}</code> · annual leave {f.leave.balance} of {f.leave.entitlement} days left</p>
      <Err msg={act.err} />

      <h2>Contracts</h2>
      <Table head={["Type", "Position", "From", "To", "Probation until", "Notice", "Hours/week", "Signed", "Status", ""]} empty={f.contracts.length ? undefined : "No employment contract recorded."}>
        {f.contracts.map((k) => (
          <tr key={k.id}><td>{k.type}</td><td>{k.position}</td><td>{day(k.start_date)}</td><td>{k.end_date ? day(k.end_date) : "open-ended"}</td><td>{day(k.probation_end)}</td><td>{k.notice_days} days</td><td className="num">{k.weekly_hours}</td><td>{day(k.signed_on)}</td>
            <td><Pill text={k.status} tone={k.status === "ended" ? "bad" : undefined} /></td>
            <td>{hr && k.status === "draft" && <button className="btn primary" disabled={act.busy} onClick={() => act.run(() => raw.post(`/v1/employment-contracts/${k.id}/activate`))}>Activate</button>}
              {hr && k.status === "active" && <button className="btn" disabled={act.busy} onClick={() => { if (confirm("End this contract?")) void act.run(() => raw.post(`/v1/employment-contracts/${k.id}/end`)); }}>End</button>}</td></tr>
        ))}
      </Table>
      {hr && (
        <form className="card form" onSubmit={(ev) => { ev.preventDefault(); void act.run(async () => { await raw.post(`/v1/employees/${id}/contracts`, { type: c.type, position: c.position, start_date: c.start_date, end_date: c.end_date || undefined, probation_end: c.probation_end || undefined, notice_days: Number(c.notice_days), weekly_hours: Number(c.weekly_hours), signed_on: c.signed_on || undefined }); setC(blankContract); }); }}>
          <p className="muted small">Draft the contract, record the date it was signed, then activate it. Activating makes it the live contract and updates the position above. The signed paper stays in the document register.</p>
          <div className="grid2">
            <Field id="ec-type" label="Type"><select id="ec-type" value={c.type} onChange={(ev) => setC({ ...c, type: ev.target.value })}>{["CDI", "CDD", "internship", "consultant"].map((t) => <option key={t}>{t}</option>)}</select></Field>
            <Field id="ec-pos" label="Position"><input id="ec-pos" required value={c.position} onChange={(ev) => setC({ ...c, position: ev.target.value })} /></Field>
          </div>
          <div className="grid2">
            <Field id="ec-start" label="Starts"><input id="ec-start" type="date" required value={c.start_date} onChange={(ev) => setC({ ...c, start_date: ev.target.value })} /></Field>
            <Field id="ec-end" label={c.type === "CDD" ? "Ends (required for a CDD)" : "Ends (blank = open-ended)"}><input id="ec-end" type="date" required={c.type === "CDD"} value={c.end_date} onChange={(ev) => setC({ ...c, end_date: ev.target.value })} /></Field>
          </div>
          <div className="grid2">
            <Field id="ec-prob" label="Probation until"><input id="ec-prob" type="date" value={c.probation_end} onChange={(ev) => setC({ ...c, probation_end: ev.target.value })} /></Field>
            <Field id="ec-sign" label="Signed on"><input id="ec-sign" type="date" value={c.signed_on} onChange={(ev) => setC({ ...c, signed_on: ev.target.value })} /></Field>
          </div>
          <div className="grid2">
            <Field id="ec-notice" label="Notice (days)"><input id="ec-notice" type="number" min="0" value={c.notice_days} onChange={(ev) => setC({ ...c, notice_days: ev.target.value })} /></Field>
            <Field id="ec-hours" label="Hours per week"><input id="ec-hours" type="number" min="1" max="80" step="0.5" value={c.weekly_hours} onChange={(ev) => setC({ ...c, weekly_hours: ev.target.value })} /></Field>
          </div>
          <button className="btn primary" disabled={act.busy}>Draft contract</button>
        </form>
      )}

      {f.pay_history.length > 0 && (
        <>
          <h2>Pay history</h2>
          <Table head={["From", "Base salary", "Allowances"]}>
            {f.pay_history.map((p) => <tr key={p.effective_from}><td>{day(p.effective_from)}</td><td className="num">{xaf(p.base_salary)}</td><td className="num">{xaf(p.allowances)}</td></tr>)}
          </Table>
          <p className="muted small">Pay is set under Payroll, where a second person validates the rules before any run.</p>
        </>
      )}

      {(f.private || hr) && (
        <>
          <h2>Personal details</h2>
          <form className="card form" onSubmit={(ev) => { ev.preventDefault(); void act.run(() => raw.put(`/v1/employees/${id}/private`, { ...priv, date_of_birth: priv.date_of_birth || "" })); }}>
            <div className="grid2">{text("date_of_birth", "Date of birth", "date")}{text("national_id", "National ID number")}</div>
            <div className="grid2">{text("cnps_number", "CNPS number")}{text("phone", "Phone")}</div>
            {text("personal_email", "Personal email", "email")}
            {text("address", "Address")}
            <div className="grid2">{text("emergency_name", "Emergency contact")}{text("emergency_phone", "Emergency phone")}</div>
            <div className="grid2">{text("bank_name", "Bank (salary paid to)")}{text("bank_account", "Account number")}</div>
            {f.private?.updated_by && <p className="muted small">Last updated by {f.private.updated_by}. Only HR, the CEO and this person can see these details.</p>}
            {hr && <button className="btn primary" disabled={act.busy}>Save details</button>}
          </form>
        </>
      )}
    </>
  );
}
