"use client";
import { useState } from "react";
import { cleanAnswers, type Answers, type FormField } from "@/lib/form-templates";
import { Err, Field } from "./ui";

/** One input per question. Used to fill a form online and to key in a paper form. */
export function FormFiller({ fields, initial, busy, error, submitLabel, onSave, onSubmit }: {
  fields: FormField[]; initial: Answers; busy: boolean; error: string; submitLabel: string;
  onSave?: (a: Record<string, string | number | boolean>) => void; onSubmit: (a: Record<string, string | number | boolean>) => void;
}) {
  const [a, setA] = useState<Answers>(initial);
  const set = (k: string, v: string | boolean) => setA({ ...a, [k]: v });
  return (
    <form className="card form" onSubmit={(e) => { e.preventDefault(); onSubmit(cleanAnswers(fields, a)); }}>
      {fields.map((f) => {
        const id = `q-${f.key}`;
        const label = f.label + (f.required ? " *" : "");
        if (f.type === "checkbox") return <label key={f.key} className="check"><input type="checkbox" checked={a[f.key] === true} onChange={(e) => set(f.key, e.target.checked)} /> {label}</label>;
        return (
          <Field key={f.key} id={id} label={label}>
            {f.type === "textarea" ? <textarea id={id} rows={3} required={f.required} value={String(a[f.key] ?? "")} onChange={(e) => set(f.key, e.target.value)} />
              : f.type === "select" ? <select id={id} required={f.required} value={String(a[f.key] ?? "")} onChange={(e) => set(f.key, e.target.value)}><option value="">Select…</option>{(f.options ?? []).map((o) => <option key={o}>{o}</option>)}</select>
              : <input id={id} required={f.required} type={f.type === "number" ? "number" : f.type === "date" ? "date" : "text"} step={f.type === "number" ? "any" : undefined} value={String(a[f.key] ?? "")} onChange={(e) => set(f.key, e.target.value)} />}
          </Field>
        );
      })}
      <Err msg={error} />
      <div className="actions">
        {onSave && <button type="button" className="btn" disabled={busy} onClick={() => onSave(cleanAnswers(fields, a))}>Save and finish later</button>}
        <button className="btn primary" disabled={busy}>{submitLabel}</button>
      </div>
    </form>
  );
}
