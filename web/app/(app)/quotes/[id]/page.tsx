"use client";
import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { xaf, day } from "@/lib/format";
import type { Quote } from "@/lib/sales-types";
import { Err, Pill, Table, useAction } from "@/components/ui";
import { Totals } from "@/components/Totals";

export default function QuotePage() {
  const { id } = useParams<{ id: string }>();
  const me = useMe();
  const router = useRouter();
  const [q, setQ] = useState<Quote | null>(null);
  const [loadErr, setLoadErr] = useState("");
  const load = useCallback(() => raw.get<Quote>(`/v1/quotes/${id}`).then(setQ).catch((e) => setLoadErr(e.message)), [id]);
  useEffect(() => { void load(); }, [load]);
  const act = useAction(load);
  if (loadErr && !q) return <p className="error">{loadErr}</p>;
  if (!q) return <p className="muted">Loading…</p>;
  const seller = me.roles.some((r) => ["sales", "manager", "ceo"].includes(r));
  const invoicer = me.roles.some((r) => ["sales", "finance", "manager", "ceo"].includes(r));
  const clientAdmin = me.roles.includes("client_admin");
  return (
    <>
      <h1>{q.title}</h1>
      <p className="meta">{q.ref ?? "Draft"} · <Pill text={q.status} tone={q.status === "approved" ? "ok" : q.status === "rejected" ? "bad" : undefined} /> · valid until {day(q.valid_until)}</p>
      <Table head={["Description", "Qty", "Unit price", "Amount"]}>
        {q.lines?.map((l, i) => (
          <tr key={i}><td>{l.description}</td><td className="num">{l.quantity}</td><td className="num">{xaf(l.unit_price)}</td><td className="num">{xaf(Math.round(l.quantity * l.unit_price))}</td></tr>
        ))}
      </Table>
      <Totals net={q.net} tva={q.tva} cac={q.cac} total={q.total} />
      <Err msg={act.err} />
      <div className="actions">
        {seller && q.status === "draft" && <button className="btn primary" disabled={act.busy} onClick={() => act.run(() => raw.post(`/v1/quotes/${id}/send`))}>Send to client</button>}
        {clientAdmin && q.status === "sent" && (
          <>
            <button className="btn primary" disabled={act.busy} onClick={() => act.run(() => raw.post(`/v1/quotes/${id}/approve`))}>Approve</button>
            <button className="btn" disabled={act.busy} onClick={() => act.run(() => raw.post(`/v1/quotes/${id}/reject`))}>Reject</button>
          </>
        )}
        {invoicer && q.status === "approved" && (
          <button className="btn primary" disabled={act.busy} onClick={() => act.run(async () => { const inv = await raw.post<{ id: string }>(`/v1/quotes/${id}/invoice`); router.push(`/invoices/${inv.id}`); })}>Create invoice</button>
        )}
      </div>
    </>
  );
}
