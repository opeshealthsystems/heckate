"use client";
import { useState, type ReactNode } from "react";
import { Icon } from "./Icon";

/** Runs an async action, shows its error, and calls onDone after success. */
export function useAction(onDone?: () => void | Promise<void>) {
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setErr("");
    try {
      await fn();
      await onDone?.();
    } catch (e) {
      setErr((e as Error).message);
    }
    setBusy(false);
  };
  return { err, busy, run, clear: () => setErr("") };
}

export function Err({ msg }: { msg: string }) {
  return msg ? <p className="error" role="alert">{msg}</p> : null;
}

export function Field({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <>
      <label htmlFor={id}>{label}</label>
      {children}
    </>
  );
}

export function Table({ head, children, empty }: { head: string[]; children: ReactNode; empty?: string }) {
  return (
    <div className="scroll">
      <table>
        <thead><tr>{head.map((h, i) => <th key={`${i}-${h}`} scope="col">{h || <span className="sr">Actions</span>}</th>)}</tr></thead>
        <tbody>{children}</tbody>
      </table>
      {empty && <p className="muted pad">{empty}</p>}
    </div>
  );
}

const GOOD = new Set(["approved", "paid", "posted", "reconciled", "received", "awarded", "completed", "closed", "resolved", "active", "verified", "issued"]);
const BAD = new Set(["rejected", "cancelled", "overdue", "failed", "void", "voided", "declined", "breached", "disputed"]);
const WARN = new Set(["pending", "submitted", "draft", "open", "partial", "partially_paid", "awaiting_approval", "in_review"]);

/** Status chip. Tone follows the status word unless one is passed, so the same state has the same colour on every screen. */
export const Pill = ({ text, tone }: { text: string; tone?: "ok" | "bad" | "warn" }) => {
  const key = text.toLowerCase();
  const t = tone ?? (GOOD.has(key) ? "ok" : BAD.has(key) ? "bad" : WARN.has(key) ? "warn" : "");
  return <span className={`chip ${t}`}>{text.replace(/_/g, " ")}</span>;
};

export function Tabs({ tabs, value, onChange }: { tabs: string[]; value: string; onChange: (t: string) => void }) {
  return (
    <div className="tabs" role="tablist">
      {tabs.map((t) => (
        <button key={t} role="tab" aria-selected={t === value} className={t === value ? "tab on" : "tab"} onClick={() => onChange(t)}>{t}</button>
      ))}
    </div>
  );
}

export interface LineInput { description: string; quantity: string; unit_price: string }
export const emptyLine = (): LineInput => ({ description: "", quantity: "1", unit_price: "" });

export function toLines(ls: LineInput[]) {
  return ls.map((l) => ({ description: l.description, quantity: Number(l.quantity), unit_price: Math.round(Number(l.unit_price)) }));
}

export function LinesEditor({ lines, onChange }: { lines: LineInput[]; onChange: (l: LineInput[]) => void }) {
  const set = (i: number, k: keyof LineInput, v: string) => onChange(lines.map((l, j) => (j === i ? { ...l, [k]: v } : l)));
  return (
    <fieldset className="lines">
      <legend>Lines (amounts in XAF)</legend>
      {lines.map((l, i) => (
        <div className="line" key={i}>
          <input aria-label={`Description ${i + 1}`} placeholder="Description" required value={l.description} onChange={(e) => set(i, "description", e.target.value)} />
          <input aria-label={`Quantity ${i + 1}`} type="number" step="0.01" min="0.01" required value={l.quantity} onChange={(e) => set(i, "quantity", e.target.value)} />
          <input aria-label={`Unit price ${i + 1}`} type="number" min="0" step="1" placeholder="Unit price" required value={l.unit_price} onChange={(e) => set(i, "unit_price", e.target.value)} />
          {lines.length > 1 && <button type="button" className="btn ghost" aria-label={`Remove line ${i + 1}`} onClick={() => onChange(lines.filter((_, j) => j !== i))}><Icon name="close" size={16} /></button>}
        </div>
      ))}
      <button type="button" className="btn" onClick={() => onChange([...lines, emptyLine()])}><Icon name="plus" size={16} /> Add line</button>
    </fieldset>
  );
}
