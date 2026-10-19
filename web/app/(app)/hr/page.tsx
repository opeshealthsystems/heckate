"use client";
import { useCallback, useEffect, useState } from "react";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { day, xaf } from "@/lib/format";
import { Err, Field, Pill, Table, Tabs, useAction } from "@/components/ui";
import { Attachments } from "@/components/Attachments";

interface Me { full_name: string; position: string; annual_leave_entitlement: number; annual_leave_taken: number; annual_leave_balance: number }
interface Leave { id: string; actor: string; kind: string; start_date: string; end_date: string; days: number; status: string }
interface Claim { id: string; actor: string; expense_date: string; category: string; description: string; amount: number; status: string; decided_by: string | null }

export default function HR() {
  const me = useMe();
  const reviewer = me.roles.some((r) => ["hr", "manager", "ceo"].includes(r));
  const payer = me.roles.some((r) => ["finance", "manager", "ceo"].includes(r));
  const [tab, setTab] = useState("My leave");
  const [rec, setRec] = useState<Me | null>(null);
  const [recErr, setRecErr] = useState("");
  const [leave, setLeave] = useState<Leave[]>([]);
  const [claims, setClaims] = useState<Claim[]>([]);
  const [lf, setLf] = useState({ kind: "annual", start_date: "", end_date: "", reason: "" });
  const [cf, setCf] = useState({ expense_date: new Date().toISOString().slice(0, 10), category: "", description: "", amount: "" });
  const all = tab === "Approvals" ? "?all=1" : "";
  const load = useCallback(async () => {
    raw.get<Me>("/v1/employees/me").then(setRec).catch((e) => setRecErr(e.message));
    raw.get<{ data: Leave[] }>(`/v1/leave${all}`).then((r) => setLeave(r.data)).catch(() => {});
    raw.get<{ data: Claim[] }>(`/v1/expenses${all}`).then((r) => setClaims(r.data)).catch(() => {});
  }, [all]);
  useEffect(() => { void load(); }, [load]);
  const act = useAction(load);
  const tabs = ["My leave", "My expenses", ...(reviewer || payer ? ["Approvals"] : [])];
  return (
    <>
      <h1>People</h1>
      {rec ? (
        <p className="meta">{rec.full_name} · annual leave: {rec.annual_leave_balance} of {rec.annual_leave_entitlement} days left ({rec.annual_leave_taken} taken this year)</p>
      ) : (
        <p className="notice">{recErr || "Loading…"} {recErr && "Ask HR to create your employee record."}</p>
      )}
      <Err msg={act.err} />
      <Tabs tabs={tabs} value={tab} onChange={setTab} />
      {tab === "My leave" && (
        <>
          <Table head={["From", "To", "Days", "Kind", "Status", ""]} empty={leave.length ? undefined : "No leave requests."}>
            {leave.map((l) => (
              <tr key={l.id}><td>{day(l.start_date)}</td><td>{day(l.end_date)}</td><td className="num">{l.days}</td><td>{l.kind}</td>
                <td><Pill text={l.status} tone={l.status === "approved" ? "ok" : l.status === "rejected" ? "bad" : undefined} /></td>
                <td>{l.status === "pending" && <button className="btn" onClick={() => act.run(() => raw.post(`/v1/leave/${l.id}/cancel`))}>Cancel</button>}</td></tr>
            ))}
          </Table>
          <h2>Request leave</h2>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post("/v1/leave", lf); setLf({ ...lf, start_date: "", end_date: "", reason: "" }); }); }}>
            <Field id="lv-kind" label="Kind"><select id="lv-kind" value={lf.kind} onChange={(e) => setLf({ ...lf, kind: e.target.value })}>{["annual", "sick", "unpaid", "other"].map((k) => <option key={k}>{k}</option>)}</select></Field>
            <div className="grid2">
              <Field id="lv-from" label="From"><input id="lv-from" type="date" required value={lf.start_date} onChange={(e) => setLf({ ...lf, start_date: e.target.value })} /></Field>
              <Field id="lv-to" label="To"><input id="lv-to" type="date" required value={lf.end_date} onChange={(e) => setLf({ ...lf, end_date: e.target.value })} /></Field>
            </div>
            <Field id="lv-reason" label="Reason"><input id="lv-reason" value={lf.reason} onChange={(e) => setLf({ ...lf, reason: e.target.value })} /></Field>
            <button className="btn primary" disabled={act.busy}>Request</button>
          </form>
        </>
      )}
      {tab === "My expenses" && (
        <>
          <Table head={["Date", "Category", "Description", "Amount", "Status"]} empty={claims.length ? undefined : "No expense claims."}>
            {claims.map((c) => (<tr key={c.id}><td>{day(c.expense_date)}</td><td>{c.category}</td><td className="wrap">{c.description}</td><td className="num">{xaf(c.amount)}</td><td><Pill text={c.status} tone={c.status === "paid" ? "ok" : c.status === "rejected" ? "bad" : undefined} /></td></tr>))}
          </Table>
          {claims.filter((c) => c.actor === me.sub && c.status === "submitted").map((c) => (
            <div key={c.id} className="card"><strong>Receipt for: {c.description}</strong><Attachments entityType="expense" entityId={c.id} /></div>
          ))}
          <h2>New expense claim</h2>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post("/v1/expenses", { ...cf, amount: Math.round(Number(cf.amount)) }); setCf({ ...cf, category: "", description: "", amount: "" }); }); }}>
            <div className="grid2">
              <Field id="ex-date" label="Date"><input id="ex-date" type="date" required value={cf.expense_date} onChange={(e) => setCf({ ...cf, expense_date: e.target.value })} /></Field>
              <Field id="ex-cat" label="Category"><input id="ex-cat" required value={cf.category} onChange={(e) => setCf({ ...cf, category: e.target.value })} /></Field>
            </div>
            <Field id="ex-desc" label="Description"><input id="ex-desc" required value={cf.description} onChange={(e) => setCf({ ...cf, description: e.target.value })} /></Field>
            <Field id="ex-amt" label="Amount (XAF)"><input id="ex-amt" type="number" min="1" step="1" required value={cf.amount} onChange={(e) => setCf({ ...cf, amount: e.target.value })} /></Field>
            <p className="muted small">After submitting, attach a photo of the receipt to the claim below.</p>
            <button className="btn primary" disabled={act.busy}>Submit</button>
          </form>
        </>
      )}
      {tab === "Approvals" && (
        <>
          <h2>Leave</h2>
          <Table head={["Who", "From", "To", "Days", "Kind", "Status", ""]} empty={leave.length ? undefined : "Nothing waiting."}>
            {leave.map((l) => (
              <tr key={l.id}><td>{l.actor}</td><td>{day(l.start_date)}</td><td>{day(l.end_date)}</td><td className="num">{l.days}</td><td>{l.kind}</td><td><Pill text={l.status} /></td>
                <td>{reviewer && l.status === "pending" && l.actor !== me.sub && (<><button className="btn primary" onClick={() => act.run(() => raw.post(`/v1/leave/${l.id}/approve`))}>Approve</button> <button className="btn" onClick={() => act.run(() => raw.post(`/v1/leave/${l.id}/reject`))}>Reject</button></>)}</td></tr>
            ))}
          </Table>
          <h2>Expense claims</h2>
          {claims.filter((c) => c.status === "submitted" && c.actor !== me.sub).map((c) => (
            <details key={c.id} className="card"><summary>Receipts for {c.actor}: {c.description}</summary><Attachments entityType="expense" entityId={c.id} canUpload={false} /></details>
          ))}
          <Table head={["Who", "Date", "Category", "Amount", "Status", ""]} empty={claims.length ? undefined : "Nothing waiting."}>
            {claims.map((c) => (
              <tr key={c.id}><td>{c.actor}</td><td>{day(c.expense_date)}</td><td>{c.category}</td><td className="num">{xaf(c.amount)}</td><td><Pill text={c.status} /></td>
                <td>
                  {reviewer && c.status === "submitted" && c.actor !== me.sub && (<><button className="btn primary" onClick={() => act.run(() => raw.post(`/v1/expenses/${c.id}/approve`))}>Approve</button> <button className="btn" onClick={() => act.run(() => raw.post(`/v1/expenses/${c.id}/reject`))}>Reject</button></>)}
                  {payer && c.status === "approved" && c.actor !== me.sub && c.decided_by !== me.sub && <button className="btn primary" onClick={() => act.run(() => raw.post(`/v1/expenses/${c.id}/pay`))}>Mark paid</button>}
                </td></tr>
            ))}
          </Table>
        </>
      )}
    </>
  );
}
