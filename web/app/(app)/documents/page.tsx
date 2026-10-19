"use client";
import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { api, fetchFile, raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { FORMS } from "@/lib/forms-library";
import { FILLABLE, FOLDER_ORDER, formPreview, type Narrative } from "@/lib/doc-library";
import { TERMS } from "@/lib/glossary";
import { day } from "@/lib/format";
import type { DocumentEntry } from "@/lib/types";
import { DocumentPreview, type PreviewSource } from "@/components/DocumentPreview";
import { VersionHistory } from "@/components/VersionHistory";
import { Err, Field, Pill, Tabs, Table, useAction } from "@/components/ui";
import { Icon } from "@/components/Icon";

const WRITERS = ["ceo", "manager", "sales", "hr", "support", "engineer", "procurement"];
const ISSUERS = ["ceo", "manager", "sales", "finance", "support", "engineer", "hr"];
const PLATFORM = { renders: "Filled and printed by the platform", records: "Recorded in its own module", template: "Approved template" } as const;
const tone = (c: string) => (c === "Restricted" ? "bad" : c === "Public" ? "ok" : c === "Confidential" ? "warn" : undefined);

export default function Documents() {
  const me = useMe();
  const writer = me.roles.some((r) => WRITERS.includes(r));
  const issuer = me.roles.some((r) => ISSUERS.includes(r));
  const [tab, setTab] = useState("Library");
  const [q, setQ] = useState("");
  const [folder, setFolder] = useState("");
  const [cls, setCls] = useState("");
  const [preview, setPreview] = useState<PreviewSource | null>(null);
  const [mine, setMine] = useState<Narrative[]>([]);
  const [f, setF] = useState({ code: "", title: "" });
  const [issued, setIssued] = useState<DocumentEntry[]>([]);
  const [history, setHistory] = useState<string | null>(null);
  const load = useCallback(() => raw.get<{ data: Narrative[] }>("/v1/narratives").then((r) => setMine(r.data)).catch(() => {}), []);
  useEffect(() => { void load(); }, [load]);
  const act = useAction(load);
  const folders = useMemo(() => [...new Set(FORMS.map((x) => x.folder))].sort((a, b) => FOLDER_ORDER.indexOf(a) - FOLDER_ORDER.indexOf(b)), []);
  const shown = [...FORMS].sort((a, b) => FOLDER_ORDER.indexOf(a.folder) - FOLDER_ORDER.indexOf(b.folder) || a.code.localeCompare(b.code, undefined, { numeric: true })).filter((x) =>
    (!folder || x.folder === folder) && (!cls || x.cls === cls) &&
    (!q || `${x.code} ${x.title} ${x.aliases.join(" ")} ${x.module}`.toLowerCase().includes(q.toLowerCase())));
  const tabs = ["Library", "My documents", "Glossary", ...(issuer ? ["Issue a reference"] : [])];

  const open = (n: Narrative) => setPreview({
    title: `${n.ref ?? "Draft"} · ${n.title}`,
    filename: `${n.ref ?? "draft-preview"}.pdf`,
    note: n.ref ? "Issued document" : "Draft preview: not issued and not valid",
    load: () => fetchFile(`/v1/narratives/${n.id}/${n.ref ? "pdf" : "preview"}`),
  });

  return (
    <>
      <h1>Documents</h1>
      <p className="muted">Preview any form before you fill it in, and any document before you print it.</p>
      <Err msg={act.err} />
      <Tabs tabs={tabs} value={tab} onChange={setTab} />

      {tab === "Library" && (
        <>
          <div className="filters">
            <label className="sr" htmlFor="doc-q">Search forms and documents</label>
            <input id="doc-q" type="search" placeholder="Search by code, name or other name" value={q} onChange={(e) => setQ(e.target.value)} />
            <label className="sr" htmlFor="doc-folder">Folder</label>
            <select id="doc-folder" value={folder} onChange={(e) => setFolder(e.target.value)}><option value="">All folders</option>{folders.map((x) => <option key={x}>{x}</option>)}</select>
            <label className="sr" htmlFor="doc-cls">Classification</label>
            <select id="doc-cls" value={cls} onChange={(e) => setCls(e.target.value)}><option value="">All classifications</option>{["Public", "Internal", "Confidential", "Restricted"].map((x) => <option key={x}>{x}</option>)}</select>
          </div>
          <Table head={["Code", "Name", "Classification", "Folder", "In the platform", ""]} empty={shown.length ? undefined : "Nothing matches."}>
            {shown.map((x) => (
              <tr key={x.code}>
                <td><code>{x.code}</code></td>
                <td className="wrap"><strong>{x.title}</strong>{x.aliases.length > 0 && <><br /><span className="muted small">Also: {x.aliases.join(", ")}</span></>}</td>
                <td><Pill text={x.cls} tone={tone(x.cls)} /></td>
                <td>{x.folder.replace(/^[A-Z]_/, "").replace(/_/g, " ")}</td>
                <td className="wrap small">{PLATFORM[x.platform]}</td>
                <td className="nowrap">
                  <button className="btn" onClick={() => setPreview(formPreview(x))}><Icon name="eye" size={16} /> Preview</button>{" "}
                  {writer && FILLABLE[x.code] && <Link className="btn primary" href={`/documents/new?code=${x.code}`}>Fill in</Link>}
                </td>
              </tr>
            ))}
          </Table>
        </>
      )}

      {tab === "My documents" && (
        <>
          <Table head={["Reference", "Title", "Kind", "Version", "Created", ""]} empty={mine.length ? undefined : "No documents yet. Fill one in from the Library."}>
            {mine.map((n) => (
              <tr key={n.id}>
                <td>{n.ref ?? <Pill text="draft" tone="warn" />}</td>
                <td className="wrap">{n.title}{n.superseded_by && <><br /><span className="muted small">Replaced by {n.superseded_by}</span></>}</td>
                <td>{n.form_code ? `HS-${n.form_code}` : n.kind.replace(/_/g, " ")}</td>
                <td>{n.revision}{n.superseded_by && <> <Pill text="replaced" /></>}</td>
                <td>{day(n.created_at)}</td>
                <td className="nowrap">
                  <button className="btn" onClick={() => open(n)}><Icon name="eye" size={16} /> Preview</button>{" "}
                  <button className="btn" aria-expanded={history === n.id} onClick={() => setHistory(history === n.id ? null : n.id)}><Icon name="time" size={16} /> History</button>{" "}
                  {!n.ref && writer && <Link className="btn" href={`/documents/new?id=${n.id}`}>Edit</Link>}{" "}
                  {!n.ref && writer && <button className="btn primary" disabled={act.busy} onClick={() => act.run(() => raw.post(`/v1/narratives/${n.id}/issue`))}>Issue</button>}
                  {n.ref && writer && !n.superseded_by && <button className="btn" disabled={act.busy} onClick={() => act.run(() => raw.post(`/v1/narratives/${n.id}/revise`, {}))}>Revise</button>}
                </td>
              </tr>
            ))}
          </Table>
          {history && (<section className="panel"><div className="panel-head"><h2>History</h2><button className="btn" onClick={() => setHistory(null)}>Close</button></div><div className="panel-body"><VersionHistory id={history} /></div></section>)}
        </>
      )}

      {tab === "Glossary" && (
        <>
          <h2>Forms and their names</h2>
          <p className="muted">Every code with its official name and the other names it goes by on file names and in conversation.</p>
          <Table head={["Code", "Official name", "Also known as", "Module"]}>
            {FORMS.map((x) => (<tr key={x.code}><td><code>{x.code}</code></td><td className="wrap">{x.title}</td><td className="wrap">{x.aliases.join(", ") || "–"}</td><td>{x.module}</td></tr>))}
          </Table>
          <h2>Terms</h2>
          <dl className="terms">{TERMS.map((t) => (<div key={t.term}><dt>{t.term}</dt><dd>{t.meaning}</dd></div>))}</dl>
        </>
      )}

      {tab === "Issue a reference" && (
        <>
          <p className="muted">Give a document you produced elsewhere a reference. Anyone can verify it at /verify.</p>
          <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { const d = await api.issueDocument(f); setIssued((p) => [d, ...p]); }); }}>
            <Field id="c" label="Code (for example INV, QT, FRM-23, CRT-A)"><input id="c" required value={f.code} onChange={(e) => setF({ ...f, code: e.target.value })} /></Field>
            <Field id="t" label="Title"><input id="t" required value={f.title} onChange={(e) => setF({ ...f, title: e.target.value })} /></Field>
            <button className="btn primary" disabled={act.busy}>Issue reference</button>
          </form>
          <ul className="thread">{issued.map((d) => <li key={d.ref}><strong>{d.ref}</strong> · {d.title}</li>)}</ul>
        </>
      )}

      {preview && <DocumentPreview source={preview} onClose={() => setPreview(null)} />}
    </>
  );
}
