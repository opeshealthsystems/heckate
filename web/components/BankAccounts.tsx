"use client";
import { useEffect, useState } from "react";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { day, xaf } from "@/lib/format";
import { ExportButton } from "./ExportButton";
import { Err, Field, Pill, Table, useAction } from "./ui";

interface Acct {
  id: string; ledger_account: string; bank_name: string; account_name: string; account_number: string; iban: string; swift: string; branch: string; currency: string; signatories: string;
  status: string; opened_on: string | null; ledger_balance: number; last_statement_to: string | null; last_statement_closing: number | null; unmatched_lines: number;
}

/** The company's bank accounts, each tied to its ledger account so the books and the bank can be read side by side. */
export function BankAccounts() {
  const me = useMe();
  const writer = me.roles.some((r) => ["ceo", "manager", "finance"].includes(r));
  const [rows, setRows] = useState<Acct[]>([]);
  const [accounts, setAccounts] = useState<{ account: string; name: string }[]>([]);
  const [err, setErr] = useState("");
  const blank = { ledger_account: "", bank_name: "", account_name: "HEKATE SYSTEMS SARL", account_number: "", iban: "", swift: "", branch: "", signatories: "", opened_on: "" };
  const [f, setF] = useState(blank);
  const load = () => raw.get<{ data: Acct[] }>("/v1/bank/accounts").then((r) => setRows(r.data)).catch((e) => setErr(e.message));
  useEffect(() => {
    void load();
    raw.get<{ data: { code: string; name: string; class: number }[] }>("/v1/accounting/accounts").then((r) => setAccounts(r.data.filter((a) => a.class === 5).map((a) => ({ account: a.code, name: a.name })))).catch(() => {});
  }, []);
  const act = useAction(async () => { setF(blank); await load(); });
  return (
    <>
      <Err msg={err} />
      <div className="actions"><ExportButton path="/v1/exports/bank-accounts" filename="bank-accounts" label="Export bank accounts CSV" /></div>
      {rows.map((a) => (
        <div className="card" key={a.id}>
          <p><strong>{a.bank_name}</strong> · {a.account_name} <Pill text={a.status} /></p>
          <p className="small">Account <code>{a.account_number}</code>{a.iban && <> · IBAN <code>{a.iban}</code></>}{a.swift && <> · SWIFT <code>{a.swift}</code></>}{a.branch && ` · ${a.branch} branch`} · {a.currency}{a.opened_on && ` · opened ${day(a.opened_on)}`}</p>
          <p className="small">Ledger account {a.ledger_account}: <strong>{xaf(a.ledger_balance)}</strong>
            {a.last_statement_to ? <> · bank statement to {day(a.last_statement_to)} closed at <strong>{xaf(a.last_statement_closing)}</strong></> : " · no statement imported yet"}
            {a.unmatched_lines > 0 && <> · <span className="tl bad">{a.unmatched_lines} bank line(s) not matched</span></>}</p>
          {a.signatories && <p className="small muted">Signatories: {a.signatories}</p>}
          {writer && a.status === "active" && <button className="btn ghost" disabled={act.busy} onClick={() => { if (confirm("Mark this account as closed?")) void act.run(() => raw.patch(`/v1/bank/accounts/${a.id}`, { close: true })); }}>Mark closed</button>}
        </div>
      ))}
      {rows.length === 0 && !err && <p className="muted">No bank account registered yet.</p>}
      {writer && (
        <>
          <h2>Register a bank account</h2>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(() => raw.post("/v1/bank/accounts", { ...f, opened_on: f.opened_on || undefined })); }}>
            <Field id="ba-ledger" label="Ledger account">
              <select id="ba-ledger" required value={f.ledger_account} onChange={(e) => setF({ ...f, ledger_account: e.target.value })}>
                <option value="">Select…</option>
                {accounts.map((a) => <option key={a.account} value={a.account}>{a.account} · {a.name}</option>)}
              </select>
            </Field>
            <div className="grid2">
              <Field id="ba-bank" label="Bank"><input id="ba-bank" required value={f.bank_name} onChange={(e) => setF({ ...f, bank_name: e.target.value })} /></Field>
              <Field id="ba-name" label="Account name"><input id="ba-name" required value={f.account_name} onChange={(e) => setF({ ...f, account_name: e.target.value })} /></Field>
            </div>
            <div className="grid2">
              <Field id="ba-num" label="Account number"><input id="ba-num" required value={f.account_number} onChange={(e) => setF({ ...f, account_number: e.target.value })} /></Field>
              <Field id="ba-iban" label="IBAN"><input id="ba-iban" value={f.iban} onChange={(e) => setF({ ...f, iban: e.target.value })} /></Field>
            </div>
            <div className="grid2">
              <Field id="ba-swift" label="SWIFT / BIC"><input id="ba-swift" value={f.swift} onChange={(e) => setF({ ...f, swift: e.target.value })} /></Field>
              <Field id="ba-branch" label="Branch"><input id="ba-branch" value={f.branch} onChange={(e) => setF({ ...f, branch: e.target.value })} /></Field>
            </div>
            <Field id="ba-sig" label="Signatories"><input id="ba-sig" value={f.signatories} onChange={(e) => setF({ ...f, signatories: e.target.value })} /></Field>
            <Field id="ba-open" label="Opened on"><input id="ba-open" type="date" value={f.opened_on} onChange={(e) => setF({ ...f, opened_on: e.target.value })} /></Field>
            <p className="muted small">Enter only what the bank has confirmed in writing. Nothing is filled in for you.</p>
            <Err msg={act.err} />
            <button className="btn primary" disabled={act.busy}>Register</button>
          </form>
        </>
      )}
    </>
  );
}
