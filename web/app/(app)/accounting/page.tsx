"use client";
import { useCallback, useEffect, useState } from "react";
import { raw } from "@/lib/api";
import { day, xaf } from "@/lib/format";
import { Err, Pill, Table, Tabs, useAction } from "@/components/ui";
import { Bank } from "@/components/Bank";

interface TB { rows: { account: string; name: string; debit: number; credit: number; balance: number }[]; total_debit: number; total_credit: number; balanced: boolean }
interface Entry { seq: number; journal: string; entry_date: string; ref: string; narration: string; hash: string; lines: { account: string; debit: number; credit: number }[] }
interface Verify { entries: number; intact: boolean; first_broken_seq?: number }

export default function Accounting() {
  const [tab, setTab] = useState("Trial balance");
  const [tb, setTb] = useState<TB | null>(null);
  const [entries, setEntries] = useState<Entry[]>([]);
  const [chain, setChain] = useState<Verify | null>(null);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [err, setErr] = useState("");
  const load = useCallback(() => {
    const q = new URLSearchParams();
    if (from) q.set("from", from);
    if (to) q.set("to", to);
    raw.get<TB>(`/v1/accounting/trial-balance?${q}`).then(setTb).catch((e) => setErr(e.message));
    raw.get<{ data: Entry[] }>("/v1/accounting/entries").then((r) => setEntries(r.data)).catch((e) => setErr(e.message));
  }, [from, to]);
  useEffect(() => { load(); }, [load]);
  const verify = useAction();
  return (
    <>
      <h1>Accounting</h1>
      <Err msg={err} />
      <Tabs tabs={["Trial balance", "Journal", "Bank", "Integrity"]} value={tab} onChange={setTab} />
      {tab === "Trial balance" && tb && (
        <>
          <div className="filters">
            <label>From<input type="date" value={from} onChange={(e) => setFrom(e.target.value)} /></label>
            <label>To<input type="date" value={to} onChange={(e) => setTo(e.target.value)} /></label>
          </div>
          <Table head={["Account", "Name", "Debit", "Credit", "Balance"]}>
            {tb.rows.map((r) => (
              <tr key={r.account}><td>{r.account}</td><td>{r.name}</td><td className="num">{xaf(r.debit)}</td><td className="num">{xaf(r.credit)}</td><td className="num">{xaf(r.balance)}</td></tr>
            ))}
            <tr><th colSpan={2}>Total</th><th className="num">{xaf(tb.total_debit)}</th><th className="num">{xaf(tb.total_credit)}</th><th><Pill text={tb.balanced ? "balanced" : "NOT BALANCED"} tone={tb.balanced ? "ok" : "bad"} /></th></tr>
          </Table>
          <p className="muted small">Starter chart of accounts: the accountant must validate it against SYSCOHADA before use.</p>
        </>
      )}
      {tab === "Journal" && (
        <Table head={["#", "Date", "Journal", "Reference", "Narration", "Lines"]} empty={entries.length ? undefined : "No entries yet."}>
          {entries.map((e) => (
            <tr key={e.seq}>
              <td>{e.seq}</td><td>{day(e.entry_date)}</td><td>{e.journal}</td><td>{e.ref || "–"}</td><td className="wrap">{e.narration}</td>
              <td className="wrap">{e.lines.map((l, i) => <div key={i}>{l.account} {l.debit ? `Dr ${xaf(l.debit)}` : `Cr ${xaf(l.credit)}`}</div>)}</td>
            </tr>
          ))}
        </Table>
      )}
      {tab === "Bank" && <Bank />}
      {tab === "Integrity" && (
        <>
          <p>Every ledger entry is chained to the one before it by a hash. Verifying recomputes the whole chain and reports the first entry that does not match.</p>
          <button className="btn primary" disabled={verify.busy} onClick={() => verify.run(async () => setChain(await raw.get<Verify>("/v1/accounting/verify")))}>Verify the ledger</button>
          <Err msg={verify.err} />
          {chain && (
            <p className={`result ${chain.intact ? "ok" : "bad"}`} role="status">
              {chain.intact ? `Intact: ${chain.entries} entries verified.` : `Broken at entry #${chain.first_broken_seq}. Investigate immediately.`}
            </p>
          )}
        </>
      )}
    </>
  );
}
