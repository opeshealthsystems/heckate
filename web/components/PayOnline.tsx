"use client";
import { useCallback, useEffect, useState } from "react";
import { raw } from "@/lib/api";
import { xaf } from "@/lib/format";
import { Err, Field, Pill, useAction } from "@/components/ui";

interface Attempt { id: string; amount: number; provider: string; status: "pending" | "succeeded" | "failed" | "expired" | "review"; reason: string }

const LABEL: Record<string, string> = { momo: "MTN Mobile Money", orange_money: "Orange Money" };

/** Pay an issued invoice by mobile money. The invoice is only marked paid once the provider confirms. */
export function PayOnline({ invoiceId, balance, onPaid }: { invoiceId: string; balance: number; onPaid: () => void }) {
  const [providers, setProviders] = useState<string[]>([]);
  const [attempts, setAttempts] = useState<Attempt[]>([]);
  const [provider, setProvider] = useState("");
  const [msisdn, setMsisdn] = useState("");
  const load = useCallback(async () => {
    const [p, a] = await Promise.all([raw.get<{ data: string[] }>("/v1/payment-providers"), raw.get<{ data: Attempt[] }>(`/v1/invoices/${invoiceId}/online-payments`)]);
    setProviders(p.data);
    setProvider((cur) => cur || p.data[0] || "");
    setAttempts(a.data);
  }, [invoiceId]);
  useEffect(() => { void load().catch(() => {}); }, [load]);
  const act = useAction(load);
  const pending = attempts.find((a) => a.status === "pending");
  // While a request waits for approval on the phone, ask the provider every few seconds.
  useEffect(() => {
    if (!pending) return;
    const t = setInterval(() => {
      raw.post<Attempt>(`/v1/online-payments/${pending.id}/refresh`).then((r) => {
        if (r.status !== "pending") { void load(); if (r.status === "succeeded") onPaid(); }
      }).catch(() => {});
    }, 5000);
    return () => clearInterval(t);
  }, [pending, load, onPaid]);
  if (providers.length === 0 || balance <= 0) return null;
  const last = attempts[0];
  return (
    <section className="card" aria-label="Pay online">
      <h2>Pay now</h2>
      <Err msg={act.err} />
      {pending ? (
        <p className="notice" role="status">A request for {xaf(pending.amount)} was sent to your phone. Approve it there; this page updates by itself.</p>
      ) : (
        <form className="form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post(`/v1/invoices/${invoiceId}/online-payments`, { provider, msisdn }); }); }}>
          {last && last.status !== "succeeded" && <p className="muted">{last.status === "failed" ? `The last attempt did not go through${last.reason ? ` (${last.reason})` : ""}.` : last.status === "review" ? "The last payment needs a check by our finance team; we will contact you." : "The last request timed out."}</p>}
          <Field id="po-prov" label="Pay with">
            <select id="po-prov" value={provider} onChange={(e) => setProvider(e.target.value)}>{providers.map((p) => <option key={p} value={p}>{LABEL[p] ?? p}</option>)}</select>
          </Field>
          <Field id="po-num" label="Mobile money number (with country code)">
            <input id="po-num" inputMode="numeric" autoComplete="tel" required placeholder="2376XXXXXXXX" value={msisdn} onChange={(e) => setMsisdn(e.target.value)} />
          </Field>
          <button className="btn primary" disabled={act.busy}>Pay {xaf(balance)}</button>
        </form>
      )}
      {attempts.some((a) => a.status === "succeeded") && <p><Pill text="payment received" tone="ok" /></p>}
    </section>
  );
}
