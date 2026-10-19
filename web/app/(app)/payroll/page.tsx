"use client";
import { useCallback, useEffect, useState } from "react";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { day, when, xaf } from "@/lib/format";
import { Err, Field, Pill, Table, Tabs, useAction } from "@/components/ui";

interface Line { code: string; label: string; base: number; rate_bp: number; amount: number }
interface Slip { id: string; actor: string; full_name: string; period: string; gross: number; deductions: Line[]; irpp: number; cac: number; net: number; employer_lines: Line[]; employer_cost: number }
interface Run { id: string; period: string; status: "draft" | "approved" | "paid"; prepared_by: string; approved_by: string | null; paid_by: string | null; total_gross: number; total_net: number; total_cost: number; payslips?: Slip[]; skipped?: string[] }
interface Rule { code: string; label: string; party: "employee" | "employer"; rate_bp: number; ceiling: number; tax_deductible: boolean }
interface Config { abatement_bp: number; personal_deduction: number; cac_bp: number; rules: Rule[]; brackets: { upto: number | null; rate_bp: number }[] }
interface CfgView { config: Config; edited_by: string; edited_at: string; validated_by: string | null; validated_at: string | null }
interface Comp { employee_id: string; full_name: string; effective_from: string; base_salary: number; allowances: number }
interface Emp { id: string; actor: string; full_name: string }

const pct = (bp: number) => `${(bp / 100).toFixed(2).replace(/\.?0+$/, "")}%`;

export default function Payroll() {
  const me = useMe();
  const has = (...r: string[]) => me.roles.some((x) => r.includes(x));
  const canDecide = has("finance", "manager", "ceo");
  const canEditRules = has("finance", "ceo");
  const canSetPay = has("hr", "ceo");
  const [tab, setTab] = useState("Runs");
  const [runs, setRuns] = useState<Run[]>([]);
  const [open, setOpen] = useState<Run | null>(null);
  const [cfg, setCfg] = useState<CfgView | null>(null);
  const [draft, setDraft] = useState("");
  const [comps, setComps] = useState<Comp[]>([]);
  const [emps, setEmps] = useState<Emp[]>([]);
  const [period, setPeriod] = useState(new Date().toISOString().slice(0, 7));
  const [cf, setCf] = useState({ employee_id: "", effective_from: new Date().toISOString().slice(0, 10), base_salary: "", allowances: "0" });
  const load = useCallback(async () => {
    const [r, c] = await Promise.all([raw.get<{ data: Run[] }>("/v1/payroll/runs"), raw.get<CfgView>("/v1/payroll/config")]);
    setRuns(r.data);
    setCfg(c);
    setDraft((d) => d || JSON.stringify(c.config, null, 2));
    if (has("hr", "finance", "ceo")) raw.get<{ data: Comp[] }>("/v1/payroll/compensation").then((x) => setComps(x.data)).catch(() => {});
    if (has("hr", "manager", "ceo")) raw.get<{ data: Emp[] }>("/v1/employees").then((x) => setEmps(x.data)).catch(() => {});
    setOpen((o) => (o ? runs.find((x) => x.id === o.id) ?? o : o));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  useEffect(() => { void load().catch(() => {}); }, [load]);
  const act = useAction(load);
  const show = (id: string) => act.run(async () => setOpen(await raw.get<Run>(`/v1/payroll/runs/${id}`)));
  const step = (id: string, what: "approve" | "pay") => act.run(async () => { setOpen(await raw.post<Run>(`/v1/payroll/runs/${id}/${what}`)); });
  const validated = !!cfg?.validated_by;
  const tabs = ["Runs", "Rules", ...(has("hr", "finance", "ceo") ? ["Salaries"] : [])];
  return (
    <>
      <h1>Payroll</h1>
      <Err msg={act.err} />
      {cfg && !validated && (
        <p className="notice" role="status">The payroll rules have not been validated. No run can be prepared until someone other than the editor checks them under Rules.</p>
      )}
      <Tabs tabs={tabs} value={tab} onChange={setTab} />
      {tab === "Runs" && (
        <>
          <Table head={["Month", "Status", "Gross", "Net", "Employer cost", "Prepared by", ""]} empty={runs.length ? undefined : "No payroll runs yet."}>
            {runs.map((r) => (
              <tr key={r.id}>
                <td>{r.period}</td><td><Pill text={r.status} tone={r.status === "paid" ? "ok" : undefined} /></td>
                <td className="num">{xaf(r.total_gross)}</td><td className="num">{xaf(r.total_net)}</td><td className="num">{xaf(r.total_cost)}</td>
                <td>{r.prepared_by}</td><td><button className="btn" onClick={() => void show(r.id)}>Open</button></td>
              </tr>
            ))}
          </Table>
          <h2>Prepare a month</h2>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { setOpen(await raw.post<Run>("/v1/payroll/runs", { period })); }); }}>
            <Field id="pr-period" label="Month"><input id="pr-period" type="month" required value={period} onChange={(e) => setPeriod(e.target.value)} /></Field>
            <button className="btn primary" disabled={act.busy || !validated}>Prepare draft</button>
          </form>
          {open && (
            <section className="card" aria-label={`Run ${open.period}`}>
              <h2>{open.period} <Pill text={open.status} tone={open.status === "paid" ? "ok" : undefined} /></h2>
              <p className="meta">Prepared by {open.prepared_by}{open.approved_by && ` · approved by ${open.approved_by}`}{open.paid_by && ` · paid by ${open.paid_by}`}</p>
              {open.skipped && open.skipped.length > 0 && <p className="notice">No salary on file, left out: {open.skipped.join(", ")}.</p>}
              <Table head={["Employee", "Gross", "Deductions", "Income tax", "Council tax", "Net", "Employer cost"]}>
                {(open.payslips ?? []).map((p) => (
                  <tr key={p.id}>
                    <td>{p.full_name}</td><td className="num">{xaf(p.gross)}</td>
                    <td className="num">{xaf(p.deductions.reduce((a, l) => a + l.amount, 0))}</td>
                    <td className="num">{xaf(p.irpp)}</td><td className="num">{xaf(p.cac)}</td>
                    <td className="num"><strong>{xaf(p.net)}</strong></td><td className="num">{xaf(p.employer_cost)}</td>
                  </tr>
                ))}
              </Table>
              <div className="row">
                {open.status === "draft" && canDecide && open.prepared_by !== me.sub && <button className="btn primary" disabled={act.busy} onClick={() => void step(open.id, "approve")}>Approve and post to ledger</button>}
                {open.status === "draft" && open.prepared_by === me.sub && <span className="muted">Someone else must approve this run.</span>}
                {open.status === "approved" && canDecide && ![open.prepared_by, open.approved_by].includes(me.sub) && <button className="btn primary" disabled={act.busy} onClick={() => void step(open.id, "pay")}>Record salaries as paid</button>}
                {open.status === "approved" && [open.prepared_by, open.approved_by].includes(me.sub) && <span className="muted">Payment must be recorded by someone who neither prepared nor approved it.</span>}
                {open.status === "draft" && <button className="btn" disabled={act.busy} onClick={() => { if (confirm("Discard this draft?")) void act.run(async () => { await raw.del(`/v1/payroll/runs/${open.id}`); setOpen(null); }); }}>Discard draft</button>}
              </div>
            </section>
          )}
        </>
      )}
      {tab === "Rules" && cfg && (
        <>
          <p className="meta">
            {validated ? <>Validated by {cfg.validated_by} on {when(cfg.validated_at)}.</> : <>Edited by {cfg.edited_by} on {when(cfg.edited_at)}; waiting for validation.</>}
          </p>
          <Table head={["Code", "Contribution", "Paid by", "Rate", "Monthly ceiling"]}>
            {cfg.config.rules.map((r) => (<tr key={r.code}><td>{r.code}</td><td>{r.label}</td><td>{r.party}</td><td className="num">{pct(r.rate_bp)}</td><td className="num">{r.ceiling ? xaf(r.ceiling) : "none"}</td></tr>))}
          </Table>
          <Table head={["Annual taxable pay up to", "Rate"]}>
            {cfg.config.brackets.map((b, i) => (<tr key={i}><td className="num">{b.upto === null ? "above" : xaf(b.upto)}</td><td className="num">{pct(b.rate_bp)}</td></tr>))}
          </Table>
          <p className="muted small">Professional-expense allowance {pct(cfg.config.abatement_bp)} · yearly personal deduction {xaf(cfg.config.personal_deduction)} · council surcharge {pct(cfg.config.cac_bp)} of income tax.</p>
          {canEditRules && (
            <>
              {!validated && cfg.edited_by !== me.sub && (
                <button className="btn primary" disabled={act.busy} onClick={() => void act.run(async () => { await raw.post("/v1/payroll/config/validate"); })}>I have checked these rules: validate</button>
              )}
              {!validated && cfg.edited_by === me.sub && <p className="muted">You edited these rules, so someone else must validate them.</p>}
              <h2>Edit rules</h2>
              <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { const c = JSON.parse(draft) as Config; const v = await raw.put<CfgView>("/v1/payroll/config", c); setCfg(v); setDraft(JSON.stringify(v.config, null, 2)); }); }}>
                <Field id="cfg-json" label="Rules (JSON). Saving clears the validation."><textarea id="cfg-json" rows={16} spellCheck={false} value={draft} onChange={(e) => setDraft(e.target.value)} /></Field>
                <button className="btn primary" disabled={act.busy}>Save rules</button>
              </form>
            </>
          )}
        </>
      )}
      {tab === "Salaries" && (
        <>
          <Table head={["Employee", "Since", "Base salary", "Allowances"]} empty={comps.length ? undefined : "No salaries recorded."}>
            {comps.map((c) => (<tr key={c.employee_id}><td>{c.full_name}</td><td>{day(c.effective_from)}</td><td className="num">{xaf(c.base_salary)}</td><td className="num">{xaf(c.allowances)}</td></tr>))}
          </Table>
          {canSetPay && (
            <>
              <h2>Set a salary</h2>
              <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.put(`/v1/payroll/compensation/${cf.employee_id}`, { effective_from: cf.effective_from, base_salary: Math.round(Number(cf.base_salary)), allowances: Math.round(Number(cf.allowances)) }); setCf({ ...cf, base_salary: "", allowances: "0" }); }); }}>
                <Field id="sal-emp" label="Employee"><select id="sal-emp" required value={cf.employee_id} onChange={(e) => setCf({ ...cf, employee_id: e.target.value })}><option value="">Choose…</option>{emps.map((x) => <option key={x.id} value={x.id}>{x.full_name}</option>)}</select></Field>
                <div className="grid2">
                  <Field id="sal-from" label="From"><input id="sal-from" type="date" required value={cf.effective_from} onChange={(e) => setCf({ ...cf, effective_from: e.target.value })} /></Field>
                  <Field id="sal-base" label="Monthly base (XAF)"><input id="sal-base" type="number" min="0" step="1" required value={cf.base_salary} onChange={(e) => setCf({ ...cf, base_salary: e.target.value })} /></Field>
                </div>
                <Field id="sal-all" label="Monthly allowances (XAF)"><input id="sal-all" type="number" min="0" step="1" value={cf.allowances} onChange={(e) => setCf({ ...cf, allowances: e.target.value })} /></Field>
                <button className="btn primary" disabled={act.busy}>Save salary</button>
              </form>
            </>
          )}
        </>
      )}
    </>
  );
}
