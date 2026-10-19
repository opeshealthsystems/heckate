"use client";
import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { downloadFile, raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { xaf, day } from "@/lib/format";
import type { Invoice } from "@/lib/sales-types";
import { Err, Field, Pill, Table, useAction } from "@/components/ui";
import { Totals } from "@/components/Totals";
import { PayOnline } from "@/components/PayOnline";

export default function InvoicePage() {
  const { id } = useParams<{ id: string }>();
  const me = useMe();
  const [v, setV] = useState<Invoice | null>(null);
  const [loadErr, setLoadErr] = useState("");
  const [pay, setPay] = useState({ amount: "", method: "bank", reference: "" });
  const [credit, setCredit] = useState({ net: "", reason: "" });
  const load = useCallback(() => raw.get<Invoice>(`/v1/invoices/${id}`).then(setV).catch((e) => setLoadErr(e.message)), [id]);
  useEffect(() => { void load(); }, [load]);
  const act = useAction(load);
  if (loadErr && !v) return <p className="error">{loadErr}</p>;
  if (!v) return <p className="muted">Loading…</p>;
  const finance = me.roles.some((r) => ["finance", "manager", "ceo"].includes(r));
  const approver = me.roles.some((r) => ["manager", "ceo"].includes(r));
  const mustWaitForSecond = v.status === "pending_approval" && (v.requested_by === me.sub || v.created_by === me.sub);
  return (
    <>
      <h1>{v.title}</h1>
      <p className="meta">{v.ref ?? "Draft"} · <Pill text={v.status} tone={v.status === "paid" ? "ok" : undefined} /> · issued {day(v.issue_date)} · due {day(v.due_date)}</p>
      <Table head={["Description", "Qty", "Unit price", "Amount"]}>
        {v.lines?.map((l, i) => (
          <tr key={i}><td>{l.description}</td><td className="num">{l.quantity}</td><td className="num">{xaf(l.unit_price)}</td><td className="num">{xaf(Math.round(l.quantity * l.unit_price))}</td></tr>
        ))}
      </Table>
      <Totals net={v.net} tva={v.tva} cac={v.cac} total={v.total} />
      {(v.status === "issued" || v.status === "paid") && (
        <p className="meta">Paid {xaf(v.paid)} · credited {xaf(v.credited)} · <strong>balance {xaf(v.balance)}</strong></p>
      )}
      <Err msg={act.err} />
      {v.status === "pending_approval" && <p className="notice">This invoice is above the approval threshold and needs a second approver.</p>}
      <div className="actions">
        {(v.status === "issued" || v.status === "paid") && v.ref && <button className="btn" disabled={act.busy} onClick={() => act.run(() => downloadFile(`/v1/invoices/${id}/pdf`, `${v.ref}.pdf`))}>Download PDF</button>}
        {finance && v.status === "draft" && <button className="btn primary" disabled={act.busy} onClick={() => act.run(() => raw.post(`/v1/invoices/${id}/issue`))}>Issue</button>}
        {approver && v.status === "pending_approval" && !mustWaitForSecond && <button className="btn primary" disabled={act.busy} onClick={() => act.run(() => raw.post(`/v1/invoices/${id}/approve`))}>Approve and issue</button>}
      </div>

      {v.status === "issued" && v.balance > 0 && me.roles.some((r) => ["client_admin", "client_user"].includes(r)) && <PayOnline invoiceId={id} balance={v.balance} onPaid={() => void load()} />}

      {finance && v.status === "issued" && (
        <>
          <h2>Record a payment</h2>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post(`/v1/invoices/${id}/payments`, { amount: Math.round(Number(pay.amount)), method: pay.method, reference: pay.reference }); setPay({ amount: "", method: "bank", reference: "" }); }); }}>
            <Field id="p-amt" label="Amount (XAF)"><input id="p-amt" type="number" min="1" step="1" required value={pay.amount} onChange={(e) => setPay({ ...pay, amount: e.target.value })} /></Field>
            <Field id="p-method" label="Method">
              <select id="p-method" value={pay.method} onChange={(e) => setPay({ ...pay, method: e.target.value })}>
                {["bank", "momo", "orange_money", "card", "cash"].map((m) => <option key={m} value={m}>{m.replace("_", " ")}</option>)}
              </select>
            </Field>
            <Field id="p-ref" label="Reference"><input id="p-ref" value={pay.reference} onChange={(e) => setPay({ ...pay, reference: e.target.value })} /></Field>
            <button className="btn primary" disabled={act.busy}>Record payment</button>
          </form>
        </>
      )}
      {finance && (v.status === "issued" || v.status === "paid") && (
        <>
          <h2>Credit note</h2>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post(`/v1/invoices/${id}/credit-notes`, { net: Math.round(Number(credit.net)), reason: credit.reason }); setCredit({ net: "", reason: "" }); }); }}>
            <Field id="c-net" label="Net amount to credit (XAF)"><input id="c-net" type="number" min="1" step="1" required value={credit.net} onChange={(e) => setCredit({ ...credit, net: e.target.value })} /></Field>
            <Field id="c-reason" label="Reason"><input id="c-reason" required value={credit.reason} onChange={(e) => setCredit({ ...credit, reason: e.target.value })} /></Field>
            <button className="btn" disabled={act.busy}>Issue credit note</button>
          </form>
        </>
      )}
    </>
  );
}
