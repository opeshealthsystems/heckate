"use client";
import { useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import type { Verification } from "@/lib/types";

const cap = (s?: string) => (s ? s.charAt(0).toUpperCase() + s.slice(1) : "");
const date = (s?: string | null) => (s ? new Date(s).toLocaleDateString("en-GB", { day: "numeric", month: "long", year: "numeric" }) : "–");

/** The public shell: navy header with the seal, a quiet watermark, and the company's verification footer. */
export function PublicShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="pub">
      <header className="pub-head">
        <Link href="/" className="pub-brand" aria-label="HEKATE SYSTEMS">
          <img src="/brand/HEKATE_Seal_White.png" alt="" width={44} height={44} />
          <span><strong>HEKATE SYSTEMS</strong><small>Document verification</small></span>
        </Link>
      </header>
      <main className="pub-main">{children}</main>
      <footer className="pub-foot">HEKATE SYSTEMS SARL · Engineering Trust · Securing Tomorrow · hekatesystems.com/verify</footer>
    </div>
  );
}

const HEAD: Record<string, { tone: "ok" | "warn" | "bad"; title: string; line: (v: Verification) => string }> = {
  authentic: { tone: "ok", title: "Authentic HEKATE document", line: () => "This document was issued by HEKATE SYSTEMS SARL and is current." },
  replaced: { tone: "warn", title: "Genuine, but replaced", line: (v) => `This document was issued by HEKATE, but a newer version, ${v.superseded_by}, has replaced it.` },
  revoked: { tone: "bad", title: "This document has been withdrawn", line: () => "HEKATE issued it, then withdrew it. Do not rely on it." },
  not_found: { tone: "bad", title: "Not a valid HEKATE document", line: () => "No document with this reference exists in the HEKATE register." },
  malformed: { tone: "bad", title: "Not a valid reference", line: () => "That is not the shape of a HEKATE reference, for example HS-INV-2026-000123." },
  signature_invalid: { tone: "bad", title: "Not a valid HEKATE document", line: () => "The code does not carry HEKATE's signature, or does not match the document it names. Treat the document as not genuine." },
};

function AccessRequest({ v }: { v: Verification }) {
  const [open, setOpen] = useState(false);
  const [f, setF] = useState({ name: "", email: "", organisation: "", reason: "" });
  const [state, setState] = useState<"idle" | "busy" | "sent">("idle");
  const [err, setErr] = useState("");
  if (state === "sent") {
    return <div className="notice ok" role="status"><strong>Request received.</strong> HEKATE management will review it and reply to {f.email}. Nothing is shared until a manager approves.</div>;
  }
  return (
    <section className="vr-access" aria-labelledby="vr-access-h">
      <h3 id="vr-access-h">Need to see this document?</h3>
      <p className="muted small">Verification confirms a document is genuine without showing it. If you are entitled to a copy, tell us who you are and why; a HEKATE manager decides.</p>
      {!open ? (
        <button className="btn" onClick={() => setOpen(true)}>Request access</button>
      ) : (
        <form className="form" onSubmit={(e) => { e.preventDefault(); setErr(""); setState("busy"); api.requestAccess({ ref: v.ref!, ...f }).then(() => setState("sent")).catch((x) => { setErr((x as Error).message); setState("idle"); }); }}>
          <label htmlFor="ra-name">Your name</label>
          <input id="ra-name" required maxLength={120} autoComplete="name" value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} />
          <label htmlFor="ra-email">Email (HEKATE replies here)</label>
          <input id="ra-email" type="email" required maxLength={200} autoComplete="email" value={f.email} onChange={(e) => setF({ ...f, email: e.target.value })} />
          <label htmlFor="ra-org">Organisation</label>
          <input id="ra-org" maxLength={200} autoComplete="organization" value={f.organisation} onChange={(e) => setF({ ...f, organisation: e.target.value })} />
          <label htmlFor="ra-reason">Why do you need it? <span className="muted small">({f.reason.trim().length}/1000, at least 20)</span></label>
          <textarea id="ra-reason" required minLength={20} maxLength={1000} rows={4} value={f.reason} onChange={(e) => setF({ ...f, reason: e.target.value })} />
          {err && <p className="error" role="alert">{err}</p>}
          <div className="actions"><button className="btn primary" disabled={state === "busy"}>Send request</button> <button type="button" className="btn" onClick={() => setOpen(false)}>Cancel</button></div>
        </form>
      )}
    </section>
  );
}

/** What a verification says: the verdict, the document's particulars, how it was checked, and what to do next. */
export function VerificationResult({ v, fileCheck }: { v: Verification; fileCheck?: React.ReactNode }) {
  const r = v.result ?? (v.valid ? "authentic" : v.status === "revoked" ? "revoked" : "not_found");
  const head = HEAD[r] ?? HEAD.not_found;
  const found = !!v.ref && r !== "not_found" && r !== "malformed" && r !== "signature_invalid";
  const sig = v.signature;
  return (
    <article className={`vr ${head.tone}`} aria-live="polite">
      <div className="vr-verdict">
        <span className="vr-seal">
          <img src="/brand/HEKATE_Seal_Navy.png" alt="" width={96} height={96} />
          <span className={`vr-badge ${head.tone}`} aria-hidden="true">{head.tone === "ok" ? "✓" : head.tone === "warn" ? "!" : "✕"}</span>
        </span>
        <div>
          <h2>{head.title}</h2>
          <p>{head.line(v)}</p>
          {r === "replaced" && v.superseded_by && <p><Link className="btn" href={`/verify?ref=${v.superseded_by}`}>Verify the newer version</Link></p>}
        </div>
      </div>
      {found && (
        <>
          <dl className="vr-facts">
            <div><dt>Document</dt><dd>{v.type}</dd></div>
            <div><dt>Title</dt><dd>{v.title}</dd></div>
            <div><dt>Reference</dt><dd><code>{v.ref}</code></dd></div>
            <div><dt>Version</dt><dd>{v.version}</dd></div>
            <div><dt>Issued</dt><dd>{date(v.issued_at)}</dd></div>
            <div><dt>Classification</dt><dd><span className={`chip ${v.classification === "restricted" ? "bad" : v.classification === "public" ? "ok" : v.classification === "confidential" ? "warn" : ""}`}>{cap(v.classification)}</span></dd></div>
            <div><dt>Issued by</dt><dd>{v.issued_by}</dd></div>
            <div><dt>Checked</dt><dd>{v.verified_count === 1 ? "Verified once" : `Verified ${v.verified_count} times`}{v.last_verified && v.verified_count && v.verified_count > 1 ? ` · last on ${date(v.last_verified)}` : ""}</dd></div>
          </dl>
          <div className="vr-checks" role="list">
            <p role="listitem" className="ok">✓ Found in the HEKATE document register</p>
            {sig?.checked && sig.valid && <p role="listitem" className="ok">✓ QR signature verified (Ed25519, HEKATE’s published key) and matches the register</p>}
            {!sig?.checked && <p role="listitem" className="muted">○ Looked up by reference. A scanned QR code also proves the code itself is HEKATE’s.</p>}
            {v.sha256 && <p role="listitem" className="ok">✓ Original file fingerprint on record: <code className="hash">{v.sha256}</code></p>}
          </div>
          {fileCheck}
          {r !== "revoked" && <AccessRequest v={v} />}
          <p className="vr-print"><button className="btn" onClick={() => window.print()}>Print this verification</button></p>
        </>
      )}
      {sig?.checked && !sig.valid && <p className="muted small">Signature check: {sig.reason}</p>}
    </article>
  );
}
