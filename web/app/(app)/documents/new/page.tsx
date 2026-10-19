"use client";
import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { api, fetchFile, raw } from "@/lib/api";
import { FORMS } from "@/lib/forms-library";
import { FILLABLE, formPreview, type Narrative } from "@/lib/doc-library";
import type { Client } from "@/lib/types";
import { DocumentPreview, type PreviewSource } from "@/components/DocumentPreview";
import { Err, Field, useAction } from "@/components/ui";
import { Icon } from "@/components/Icon";

interface Sec { heading: string; body: string }

function Fill() {
  const router = useRouter();
  const code = useSearchParams().get("code") ?? "";
  const entry = FORMS.find((x) => x.code === code);
  const how = FILLABLE[code];
  const [clients, setClients] = useState<Client[]>([]);
  const [tenant, setTenant] = useState("");
  const [title, setTitle] = useState(entry?.title ?? "");
  const [secs, setSecs] = useState<Sec[]>([{ heading: "", body: "" }]);
  const [signed, setSigned] = useState("");
  const [saved, setSaved] = useState<Narrative | null>(null);
  const [preview, setPreview] = useState<PreviewSource | null>(null);
  useEffect(() => { api.clients().then((r) => setClients(r.data)).catch(() => {}); }, []);
  const act = useAction();

  if (!entry || !how) {
    return (<><h1>Fill in a document</h1><p className="error">That form is not one the platform fills in. <Link href="/documents">Back to the library</Link></p></>);
  }
  const body = () => ({ ...(saved ? {} : { kind: how.kind, ...(how.form ? { form: how.form } : {}), tenant_id: tenant }), title, signed_by: signed, sections: secs.filter((s) => s.body.trim()) });
  const save = async (): Promise<Narrative> => {
    const n = saved ? await raw.put<Narrative>(`/v1/narratives/${saved.id}`, body()) : await raw.post<Narrative>("/v1/narratives", body());
    setSaved(n);
    return n;
  };
  const setSec = (i: number, k: keyof Sec, v: string) => setSecs(secs.map((s, j) => (j === i ? { ...s, [k]: v } : s)));
  return (
    <>
      <h1>Fill in: {entry.title}</h1>
      <p className="meta"><code>{entry.code}</code> · {entry.cls} · <button type="button" className="linklike" onClick={() => setPreview(formPreview(entry))}><Icon name="eye" size={14} /> View the blank form</button></p>
      <Err msg={act.err} />
      <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await save(); router.push("/documents"); }); }}>
        <Field id="d-client" label="Client">
          <select id="d-client" required value={tenant} onChange={(e) => setTenant(e.target.value)} disabled={!!saved}>
            <option value="">Select…</option>
            {clients.map((c) => <option key={c.tenant_id} value={c.tenant_id}>{c.legal_name}</option>)}
          </select>
        </Field>
        {!clients.length && <p className="notice">Your role cannot list clients. Ask sales or a manager to prepare documents that are addressed to a client.</p>}
        <Field id="d-title" label="Title"><input id="d-title" required maxLength={200} value={title} onChange={(e) => setTitle(e.target.value)} /></Field>
        <fieldset className="lines">
          <legend>Content</legend>
          {secs.map((s, i) => (
            <div key={i} className="sec">
              <input aria-label={`Heading ${i + 1}`} placeholder="Heading (optional)" value={s.heading} onChange={(e) => setSec(i, "heading", e.target.value)} />
              <textarea aria-label={`Text ${i + 1}`} rows={4} required={i === 0} placeholder="Text" value={s.body} onChange={(e) => setSec(i, "body", e.target.value)} />
              {secs.length > 1 && <button type="button" className="btn ghost" aria-label={`Remove section ${i + 1}`} onClick={() => setSecs(secs.filter((_, j) => j !== i))}><Icon name="close" size={16} /></button>}
            </div>
          ))}
          <button type="button" className="btn" onClick={() => setSecs([...secs, { heading: "", body: "" }])}><Icon name="plus" size={16} /> Add a section</button>
        </fieldset>
        <Field id="d-signed" label="Signed for HEKATE by (optional)"><input id="d-signed" value={signed} onChange={(e) => setSigned(e.target.value)} /></Field>
        <div className="actions">
          <button type="button" className="btn" disabled={act.busy || !tenant || !title || !secs[0].body.trim()} onClick={() => act.run(async () => {
            const n = await save();
            setPreview({ title: `Draft · ${n.title}`, filename: "draft-preview.pdf", note: "Draft preview: not issued and not valid", load: () => fetchFile(`/v1/narratives/${n.id}/preview`) });
          })}><Icon name="eye" size={16} /> Preview as it will print</button>
          <button className="btn primary" disabled={act.busy}>Save draft</button>
        </div>
        <p className="muted small">A draft can be changed. You issue it from <Link href="/documents">My documents</Link>; issuing gives it a reference and freezes it.</p>
      </form>
      {preview && <DocumentPreview source={preview} onClose={() => setPreview(null)} />}
    </>
  );
}

export default function FillPage() {
  return (<Suspense fallback={<p className="muted">Loading…</p>}><Fill /></Suspense>);
}
