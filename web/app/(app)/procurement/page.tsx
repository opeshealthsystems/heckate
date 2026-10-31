"use client";
import { useCallback, useEffect, useState } from "react";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { xaf } from "@/lib/format";
import { Err, Field, LinesEditor, Pill, Table, Tabs, emptyLine, toLines, useAction, type LineInput } from "@/components/ui";

interface Supplier { tenant_id: string; legal_name: string; bank_status: string; bank_account_last4: string | null; avg_score: number | null }
interface RFQ { id: string; ref: string | null; title: string; status: string; lines?: { position: number; description: string; quantity: number }[]; responses?: { supplier: string; supplier_name: string; total: number }[] }
interface PO { id: string; ref: string | null; supplier: string; title: string; status: string; net: number; created_by: string; requested_by: string | null; lines?: { position: number; description: string; quantity: number; unit_price: number; item_id: string | null; received_qty: number }[] }
interface Inv { id: string; po_id: string; supplier: string; number: string; net: number; vat: number; total: number; status: string; approved_by: string | null }
interface WH { id: string; name: string }

export default function Procurement() {
  const me = useMe();
  const supplier = me.roles.includes("supplier");
  const buyer = me.roles.some((r) => ["procurement", "manager", "ceo"].includes(r));
  const payer = me.roles.some((r) => ["finance", "manager", "ceo"].includes(r));
  const approver = me.roles.some((r) => ["manager", "ceo"].includes(r));
  const tabs = supplier ? ["RFQs", "Purchase orders", "Invoices", "Bank details"] : ["Suppliers", "RFQs", "Purchase orders", "Invoices"];
  const [tab, setTab] = useState(tabs[0]);
  const [sups, setSups] = useState<Supplier[]>([]);
  const [rfqs, setRfqs] = useState<RFQ[]>([]);
  const [pos, setPos] = useState<PO[]>([]);
  const [invs, setInvs] = useState<Inv[]>([]);
  const [whs, setWhs] = useState<WH[]>([]);
  const [open, setOpen] = useState<RFQ | null>(null);
  const [err, setErr] = useState("");
  const load = useCallback(async () => {
    const get = <T,>(p: string, set: (v: T) => void) => raw.get<{ data: T }>(p).then((r) => set(r.data)).catch((e) => { if (e.status !== 403) setErr(e.message); });
    if (!supplier) void get<Supplier[]>("/v1/suppliers", setSups);
    if (supplier || buyer) void get<RFQ[]>("/v1/rfqs", setRfqs);
    void get<PO[]>("/v1/purchase-orders", setPos);
    void get<Inv[]>("/v1/supplier-invoices", setInvs);
    if (me.roles.some((r) => ["ceo", "manager"].includes(r))) void get<WH[]>("/v1/warehouses", setWhs);
  }, [supplier, buyer, me.roles]);
  useEffect(() => { void load(); }, [load]);
  const act = useAction(load);

  // forms
  const [sf, setSf] = useState({ legal_name: "", rccm: "", email: "" });
  const [rf, setRf] = useState({ title: "", lines: "", suppliers: [] as string[] });
  const [pf, setPf] = useState({ supplier_id: "", title: "" });
  const [pl, setPl] = useState<LineInput[]>([emptyLine()]);
  const [prices, setPrices] = useState<Record<number, string>>({});
  const [inv, setInv] = useState({ po_id: "", number: "", net: "", vat: "0" });
  const [bank, setBank] = useState("");
  const [note, setNote] = useState<Record<string, string>>({});
  const [wh, setWh] = useState("");
  const name = (id: string) => sups.find((s) => s.tenant_id === id)?.legal_name ?? id.slice(0, 8);

  return (
    <>
      <h1>{supplier ? "Supplier portal" : "Purchasing and suppliers"}</h1>
      <Err msg={err || act.err} />
      <Tabs tabs={tabs} value={tab} onChange={setTab} />

      {tab === "Suppliers" && (
        <>
          <Table head={["Supplier", "Bank details", "Score", ""]} empty={sups.length ? undefined : "No suppliers yet."}>
            {sups.map((s) => (
              <tr key={s.tenant_id}>
                <td>{s.legal_name}</td>
                <td><Pill text={s.bank_status === "none" ? "none" : `${s.bank_status} ····${s.bank_account_last4 ?? ""}`} tone={s.bank_status === "verified" ? "ok" : s.bank_status === "pending_callback" ? "bad" : undefined} /></td>
                <td className="num">{s.avg_score?.toFixed(1) ?? "–"}</td>
                <td>
                  {payer && s.bank_status === "pending_callback" && (
                    <form className="inline" onSubmit={(e) => { e.preventDefault(); void act.run(() => raw.post(`/v1/suppliers/${s.tenant_id}/bank/verify`, { note: note[s.tenant_id] ?? "" })); }}>
                      <input aria-label={`Call-back note for ${s.legal_name}`} placeholder="Who you called, on which number" required minLength={5} value={note[s.tenant_id] ?? ""} onChange={(e) => setNote({ ...note, [s.tenant_id]: e.target.value })} />
                      <button className="btn primary">Verified by call-back</button>
                    </form>
                  )}
                </td>
              </tr>
            ))}
          </Table>
          {buyer && (
            <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post("/v1/suppliers", sf); setSf({ legal_name: "", rccm: "", email: "" }); }); }}>
              <h2>Register a supplier</h2>
              <Field id="s-name" label="Legal name"><input id="s-name" required value={sf.legal_name} onChange={(e) => setSf({ ...sf, legal_name: e.target.value })} /></Field>
              <div className="grid2">
                <Field id="s-rccm" label="RCCM"><input id="s-rccm" value={sf.rccm} onChange={(e) => setSf({ ...sf, rccm: e.target.value })} /></Field>
                <Field id="s-email" label="Email"><input id="s-email" type="email" value={sf.email} onChange={(e) => setSf({ ...sf, email: e.target.value })} /></Field>
              </div>
              <button className="btn primary" disabled={act.busy}>Register</button>
            </form>
          )}
        </>
      )}

      {tab === "RFQs" && (
        <>
          <Table head={["Reference", "Title", "Status", ""]} empty={rfqs.length ? undefined : supplier ? "No open requests for you." : "No RFQs yet."}>
            {rfqs.map((r) => (
              <tr key={r.id}>
                <td>{r.ref ?? "draft"}</td><td>{r.title}</td><td><Pill text={r.status} /></td>
                <td>
                  <button className="btn" onClick={() => raw.get<RFQ>(`/v1/rfqs/${r.id}`).then((d) => { setOpen(d); setPrices({}); }).catch((e) => setErr(e.message))}>View</button>{" "}
                  {buyer && r.status === "draft" && <button className="btn primary" onClick={() => act.run(() => raw.post(`/v1/rfqs/${r.id}/open`))}>Open</button>}{" "}
                  {buyer && r.status === "open" && <button className="btn" onClick={() => act.run(() => raw.post(`/v1/rfqs/${r.id}/close`))}>Close</button>}
                </td>
              </tr>
            ))}
          </Table>
          {open && (
            <section className="card">
              <h2>{open.title}</h2>
              <Table head={["#", "Item", "Quantity", ...(supplier && open.status === "open" ? ["Your unit price (XAF)"] : [])]}>
                {open.lines?.map((l) => (
                  <tr key={l.position}><td>{l.position}</td><td>{l.description}</td><td className="num">{l.quantity}</td>
                    {supplier && open.status === "open" && <td><input aria-label={`Unit price for line ${l.position}`} type="number" min="0" step="1" value={prices[l.position] ?? ""} onChange={(e) => setPrices({ ...prices, [l.position]: e.target.value })} /></td>}</tr>
                ))}
              </Table>
              {supplier && open.status === "open" && (
                <button className="btn primary" disabled={act.busy} onClick={() => act.run(async () => { await raw.post(`/v1/rfqs/${open.id}/respond`, { unit_prices: (open.lines ?? []).map((l) => ({ position: l.position, unit_price: Math.round(Number(prices[l.position] ?? 0)) })) }); setOpen(null); })}>Submit prices</button>
              )}
              {buyer && (open.responses?.length ?? 0) > 0 && (
                <>
                  <h3>Responses</h3>
                  <Table head={["Supplier", "Total", ""]}>
                    {open.responses?.map((p) => (
                      <tr key={p.supplier}><td>{p.supplier_name}</td><td className="num">{xaf(p.total)}</td>
                        <td>{open.status === "closed" && <button className="btn primary" onClick={() => act.run(async () => { await raw.post(`/v1/rfqs/${open.id}/award`, { supplier: p.supplier }); setOpen(null); })}>Award</button>}</td></tr>
                    ))}
                  </Table>
                </>
              )}
            </section>
          )}
          {buyer && (
            <form className="card form" onSubmit={(e) => {
              e.preventDefault();
              const lines = rf.lines.split("\n").map((l) => l.trim()).filter(Boolean).map((l) => { const [d, q] = l.split("|"); return { description: d.trim(), quantity: Number((q ?? "1").trim()) }; });
              void act.run(async () => { await raw.post("/v1/rfqs", { title: rf.title, lines, suppliers: rf.suppliers }); setRf({ title: "", lines: "", suppliers: [] }); });
            }}>
              <h2>New RFQ</h2>
              <Field id="rf-title" label="Title"><input id="rf-title" required value={rf.title} onChange={(e) => setRf({ ...rf, title: e.target.value })} /></Field>
              <Field id="rf-lines" label="Lines, one per row as: description | quantity"><textarea id="rf-lines" rows={3} required value={rf.lines} onChange={(e) => setRf({ ...rf, lines: e.target.value })} /></Field>
              <Field id="rf-sup" label="Invite suppliers (hold Ctrl or Cmd to pick several)">
                <select id="rf-sup" multiple required value={rf.suppliers} onChange={(e) => setRf({ ...rf, suppliers: [...e.target.selectedOptions].map((o) => o.value) })}>
                  {sups.map((s) => <option key={s.tenant_id} value={s.tenant_id}>{s.legal_name}</option>)}
                </select>
              </Field>
              <button className="btn primary" disabled={act.busy}>Create draft</button>
            </form>
          )}
        </>
      )}

      {tab === "Purchase orders" && (
        <>
          {buyer && whs.length > 0 && (
            <label>Receive into warehouse
              <select value={wh} onChange={(e) => setWh(e.target.value)}><option value="">Select…</option>{whs.map((w) => <option key={w.id} value={w.id}>{w.name}</option>)}</select>
            </label>
          )}
          <Table head={["Reference", "Supplier", "Title", "Net", "Status", ""]} empty={pos.length ? undefined : "No purchase orders."}>
            {pos.map((p) => (
              <tr key={p.id}>
                <td>{p.ref ?? "draft"}</td><td>{supplier ? "You" : name(p.supplier)}</td><td>{p.title}</td><td className="num">{xaf(p.net)}</td><td><Pill text={p.status} /></td>
                <td>
                  {buyer && p.status === "draft" && <button className="btn primary" onClick={() => act.run(() => raw.post(`/v1/purchase-orders/${p.id}/issue`))}>Issue</button>}
                  {approver && p.status === "pending_approval" && p.created_by !== me.sub && p.requested_by !== me.sub && <button className="btn primary" onClick={() => act.run(() => raw.post(`/v1/purchase-orders/${p.id}/approve`))}>Approve</button>}
                  {buyer && p.status === "issued" && (
                    <button className="btn" onClick={() => act.run(async () => {
                      const d = await raw.get<PO>(`/v1/purchase-orders/${p.id}`);
                      const lines = (d.lines ?? []).filter((l) => l.quantity > l.received_qty).map((l) => ({ position: l.position, qty: l.quantity - l.received_qty, warehouse_id: l.item_id ? wh : undefined }));
                      await raw.post(`/v1/purchase-orders/${p.id}/receive`, { lines });
                    })}>Receive all outstanding</button>
                  )}
                </td>
              </tr>
            ))}
          </Table>
          {buyer && (
            <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post("/v1/purchase-orders", { ...pf, lines: toLines(pl) }); setPf({ supplier_id: "", title: "" }); setPl([emptyLine()]); }); }}>
              <h2>New purchase order</h2>
              <Field id="po-sup" label="Supplier"><select id="po-sup" required value={pf.supplier_id} onChange={(e) => setPf({ ...pf, supplier_id: e.target.value })}><option value="">Select…</option>{sups.map((s) => <option key={s.tenant_id} value={s.tenant_id}>{s.legal_name}</option>)}</select></Field>
              <Field id="po-title" label="Title"><input id="po-title" required value={pf.title} onChange={(e) => setPf({ ...pf, title: e.target.value })} /></Field>
              <LinesEditor lines={pl} onChange={setPl} />
              <button className="btn primary" disabled={act.busy}>Create draft</button>
            </form>
          )}
        </>
      )}

      {tab === "Invoices" && (
        <>
          <Table head={["Number", ...(supplier ? [] : ["Supplier"]), "Net", "VAT", "Total", "Status", ""]} empty={invs.length ? undefined : "No supplier invoices."}>
            {invs.map((v) => (
              <tr key={v.id}>
                <td>{v.number}</td>{!supplier && <td>{name(v.supplier)}</td>}<td className="num">{xaf(v.net)}</td><td className="num">{xaf(v.vat)}</td><td className="num">{xaf(v.total)}</td>
                <td><Pill text={v.status} tone={v.status === "paid" ? "ok" : undefined} /></td>
                <td>
                  {payer && v.status === "submitted" && (<><button className="btn primary" onClick={() => act.run(() => raw.post(`/v1/supplier-invoices/${v.id}/approve`))}>Approve (3-way match)</button> <button className="btn" onClick={() => act.run(() => raw.post(`/v1/supplier-invoices/${v.id}/reject`))}>Reject</button></>)}
                  {payer && v.status === "approved" && v.approved_by !== me.sub && <button className="btn primary" onClick={() => act.run(() => raw.post(`/v1/supplier-invoices/${v.id}/pay`, { method: "bank" }))}>Pay by bank</button>}
                </td>
              </tr>
            ))}
          </Table>
          {supplier && (
            <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post(`/v1/purchase-orders/${inv.po_id}/invoices`, { number: inv.number, net: Math.round(Number(inv.net)), vat: Math.round(Number(inv.vat)) }); setInv({ po_id: "", number: "", net: "", vat: "0" }); }); }}>
              <h2>Submit an invoice</h2>
              <Field id="in-po" label="Purchase order"><select id="in-po" required value={inv.po_id} onChange={(e) => setInv({ ...inv, po_id: e.target.value })}><option value="">Select…</option>{pos.map((p) => <option key={p.id} value={p.id}>{p.ref} · {p.title}</option>)}</select></Field>
              <Field id="in-num" label="Your invoice number"><input id="in-num" required value={inv.number} onChange={(e) => setInv({ ...inv, number: e.target.value })} /></Field>
              <div className="grid2">
                <Field id="in-net" label="Net (XAF)"><input id="in-net" type="number" min="1" step="1" required value={inv.net} onChange={(e) => setInv({ ...inv, net: e.target.value })} /></Field>
                <Field id="in-vat" label="VAT (XAF)"><input id="in-vat" type="number" min="0" step="1" required value={inv.vat} onChange={(e) => setInv({ ...inv, vat: e.target.value })} /></Field>
              </div>
              <button className="btn primary" disabled={act.busy}>Submit</button>
            </form>
          )}
        </>
      )}

      {tab === "Bank details" && (
        <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.put("/v1/suppliers/me/bank", { account: bank }); setBank(""); }); }}>
          <h2>Bank account</h2>
          <p>New or changed bank details are checked by a phone call-back from our finance team before we can pay into them.</p>
          <Field id="bk" label="Account number or IBAN"><input id="bk" required minLength={8} value={bank} onChange={(e) => setBank(e.target.value)} /></Field>
          <button className="btn primary" disabled={act.busy}>Submit for verification</button>
        </form>
      )}
    </>
  );
}
