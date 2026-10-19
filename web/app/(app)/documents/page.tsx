"use client";
import { useState } from "react";
import { api } from "@/lib/api";
import type { DocumentEntry } from "@/lib/types";

export default function Documents() {
  const [f, setF] = useState({ code: "", title: "" });
  const [issued, setIssued] = useState<DocumentEntry[]>([]);
  const [err, setErr] = useState("");
  return (
    <>
      <h1>Documents</h1>
      <p className="muted">Issue a reference for a document. Anyone can verify it at /verify.</p>
      <form className="card form" onSubmit={(e) => { e.preventDefault(); setErr(""); api.issueDocument(f).then((d) => setIssued((p) => [d, ...p])).catch((x) => setErr(x.message)); }}>
        <label htmlFor="c">Code (e.g. INV, QT, FRM-23, CRT-A)</label>
        <input id="c" required value={f.code} onChange={(e) => setF({ ...f, code: e.target.value })} />
        <label htmlFor="t">Title</label>
        <input id="t" required value={f.title} onChange={(e) => setF({ ...f, title: e.target.value })} />
        {err && <p className="error">{err}</p>}
        <button className="btn primary">Issue reference</button>
      </form>
      <ul className="thread">
        {issued.map((d) => <li key={d.ref}><strong>{d.ref}</strong> · {d.title}</li>)}
      </ul>
    </>
  );
}
