"use client";
import { useState } from "react";
import { PdfActions } from "./PdfActions";
import { ExportButton } from "./ExportButton";

/** Printable statements and spreadsheet exports for finance and the accountant, for a chosen period. */
export function Reports({ accounts, canSell }: { accounts: { account: string; name: string }[]; canSell: boolean }) {
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [account, setAccount] = useState(accounts[0]?.account ?? "");
  const q = new URLSearchParams();
  if (from) q.set("from", from);
  if (to) q.set("to", to);
  const qs = q.toString() ? `?${q}` : "";
  const Row = ({ title, hint, children }: { title: string; hint: string; children: React.ReactNode }) => (
    <li><div><strong>{title}</strong><p className="muted small">{hint}</p></div><div className="actions">{children}</div></li>
  );
  return (
    <>
      <p className="muted">Printable reports use the HEKATE letterhead. Exports are CSV files that open in Excel; amounts are whole francs so they can be summed.</p>
      <div className="filters">
        <label>From<input type="date" value={from} onChange={(e) => setFrom(e.target.value)} /></label>
        <label>To<input type="date" value={to} onChange={(e) => setTo(e.target.value)} /></label>
      </div>
      <ul className="rows reports">
        <Row title="Financial statements" hint="Income statement for the period and balance sheet at its end."><PdfActions path={`/v1/accounting/statements/pdf${qs}`} reference="financial-statements" title="Financial statements" /></Row>
        <Row title="Trial balance" hint="Every account with its debits, credits and balance."><PdfActions path={`/v1/accounting/trial-balance/pdf${qs}`} reference="trial-balance" title="Trial balance" /><ExportButton path={`/v1/exports/trial-balance${qs}`} filename="trial-balance" /></Row>
        <Row title="General ledger of one account" hint="Every entry on the account with a running balance.">
          <label className="sr" htmlFor="rp-acct">Account</label>
          <select id="rp-acct" value={account} onChange={(e) => setAccount(e.target.value)}>{accounts.map((a) => <option key={a.account} value={a.account}>{a.account} {a.name}</option>)}</select>
          {account && <PdfActions path={`/v1/accounting/ledger/${account}/pdf`} reference={`ledger-${account}`} title={`General ledger ${account}`} />}
        </Row>
        <Row title="All ledger lines" hint="Every posted line with its entry, journal, source and who posted it."><ExportButton path={`/v1/exports/ledger${qs}`} filename="ledger" /></Row>
        {canSell && (
          <>
            <Row title="Receivables ageing" hint="What clients owe, by how far past due."><PdfActions path="/v1/reports/ageing/pdf" reference="receivables-ageing" title="Receivables ageing" /><ExportButton path="/v1/exports/ageing" filename="receivables-ageing" /></Row>
            <Row title="Invoices" hint="Every invoice with its amounts, paid, credited and balance."><ExportButton path="/v1/exports/invoices" filename="invoices" /></Row>
            <Row title="Payments received" hint="Every payment with its invoice, client and method."><ExportButton path={`/v1/exports/payments${qs}`} filename="payments" /></Row>
          </>
        )}
      </ul>
    </>
  );
}
