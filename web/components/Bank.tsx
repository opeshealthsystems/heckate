"use client";
import { useCallback, useEffect, useState } from "react";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { day, xaf } from "@/lib/format";
import { Err, Field, Pill, Table, useAction } from "@/components/ui";

interface Statement { id: string; account: string; period_from: string; period_to: string; opening: number; closing: number; lines: number; matched: number }
interface Candidate { entry_line_id: number; seq: number; entry_date: string; ref: string; narration: string; amount: number }
interface Line { id: string; position: number; line_date: string; description: string; reference: string; amount: number; entry_line_id: number | null; suggestions: Candidate[] }
interface Detail {
  statement: Statement; lines: Line[]; unmatched_ledger: Candidate[]; auto_matched?: number;
  summary: { ledger_balance: number; statement_closing: number; difference: number; unmatched_bank_total: number; unmatched_ledger_total: number; reconciled: boolean };
}

/** Bank reconciliation: import a statement, match its lines to the ledger, see what is left. */
export function Bank() {
  const me = useMe();
  const canWrite = me.roles.some((r) => ["finance", "manager", "ceo"].includes(r));
  const [list, setList] = useState<Statement[]>([]);
  const [open, setOpen] = useState<Detail | null>(null);
  const [form, setForm] = useState({ account: "521", opening: "0", closing: "", csv: "" });
  const load = useCallback(async () => {
    setList((await raw.get<{ data: Statement[] }>("/v1/bank/statements")).data);
  }, []);
  useEffect(() => { void load().catch(() => {}); }, [load]);
  const act = useAction(load);
  const show = (id: string) => act.run(async () => setOpen(await raw.get<Detail>(`/v1/bank/statements/${id}`)));
  const refresh = async () => { if (open) setOpen(await raw.get<Detail>(`/v1/bank/statements/${open.statement.id}`)); };
  return (
    <>
      <Err msg={act.err} />
      <Table head={["Account", "From", "To", "Closing", "Matched", ""]} empty={list.length ? undefined : "No statements imported."}>
        {list.map((s) => (
          <tr key={s.id}>
            <td>{s.account}</td><td>{day(s.period_from)}</td><td>{day(s.period_to)}</td><td className="num">{xaf(s.closing)}</td>
            <td><Pill text={`${s.matched} of ${s.lines}`} tone={s.matched === s.lines ? "ok" : undefined} /></td>
            <td><button className="btn" onClick={() => void show(s.id)}>Open</button></td>
          </tr>
        ))}
      </Table>
      {open && (
        <section className="card" aria-label="Reconciliation">
          <h2>Account {open.statement.account}, {day(open.statement.period_from)} to {day(open.statement.period_to)}</h2>
          <p className="meta">
            Ledger {xaf(open.summary.ledger_balance)} · bank {xaf(open.summary.statement_closing)} · difference {xaf(open.summary.difference)}{" "}
            <Pill text={open.summary.reconciled ? "reconciled" : "open items"} tone={open.summary.reconciled ? "ok" : undefined} />
          </p>
          {open.auto_matched !== undefined && <p className="notice" role="status">{open.auto_matched} line(s) matched automatically.</p>}
          <Table head={["Date", "Description", "Amount", "Match", ""]}>
            {open.lines.map((l) => (
              <tr key={l.id}>
                <td>{day(l.line_date)}</td><td className="wrap">{l.description}{l.reference && ` (${l.reference})`}</td><td className="num">{xaf(l.amount)}</td>
                <td>
                  {l.entry_line_id !== null ? <Pill text="matched" tone="ok" /> : l.suggestions.length === 0 ? <span className="muted">no candidate</span> : (
                    canWrite ? l.suggestions.map((c) => (
                      <button key={c.entry_line_id} className="btn" disabled={act.busy} onClick={() => void act.run(async () => { await raw.post(`/v1/bank/lines/${l.id}/match`, { entry_line_id: c.entry_line_id }); await refresh(); })}>
                        Match {day(c.entry_date)} {c.ref || c.narration}
                      </button>
                    )) : <span className="muted">{l.suggestions.length} candidate(s)</span>
                  )}
                </td>
                <td>{canWrite && l.entry_line_id !== null && <button className="btn" disabled={act.busy} onClick={() => void act.run(async () => { await raw.del(`/v1/bank/lines/${l.id}/match`); await refresh(); })}>Undo</button>}</td>
              </tr>
            ))}
          </Table>
          {canWrite && <button className="btn primary" disabled={act.busy} onClick={() => void act.run(async () => setOpen(await raw.post<Detail>(`/v1/bank/statements/${open.statement.id}/auto-match`)))}>Match the unambiguous lines</button>}
          <h3>In the ledger but not on the statement</h3>
          <Table head={["Date", "Reference", "Narration", "Amount"]} empty={open.unmatched_ledger.length ? undefined : "Nothing outstanding."}>
            {open.unmatched_ledger.map((c) => (<tr key={c.entry_line_id}><td>{day(c.entry_date)}</td><td>{c.ref}</td><td className="wrap">{c.narration}</td><td className="num">{xaf(c.amount)}</td></tr>))}
          </Table>
        </section>
      )}
      {canWrite && (
        <>
          <h2>Import a statement</h2>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { setOpen(await raw.post<Detail>("/v1/bank/statements", { account: form.account, opening: Math.round(Number(form.opening)), closing: Math.round(Number(form.closing)), csv: form.csv })); setForm({ ...form, csv: "", closing: "" }); }); }}>
            <div className="grid2">
              <Field id="bk-acct" label="Ledger account"><input id="bk-acct" required pattern="5[0-9]*" value={form.account} onChange={(e) => setForm({ ...form, account: e.target.value })} /></Field>
              <Field id="bk-open" label="Opening balance (XAF)"><input id="bk-open" type="number" step="1" required value={form.opening} onChange={(e) => setForm({ ...form, opening: e.target.value })} /></Field>
            </div>
            <Field id="bk-close" label="Closing balance (XAF)"><input id="bk-close" type="number" step="1" required value={form.closing} onChange={(e) => setForm({ ...form, closing: e.target.value })} /></Field>
            <Field id="bk-csv" label="Statement lines: date, description, amount, reference (money in is positive)"><textarea id="bk-csv" rows={8} required spellCheck={false} placeholder={"2026-10-02,TRANSFER IN ACME,300000,TR1\n2026-10-09,BANK CHARGES,-40000,"} value={form.csv} onChange={(e) => setForm({ ...form, csv: e.target.value })} /></Field>
            <button className="btn primary" disabled={act.busy}>Import</button>
          </form>
        </>
      )}
    </>
  );
}
