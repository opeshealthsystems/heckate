"use client";
import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import type { Verification } from "@/lib/types";
import { PublicShell, VerificationResult } from "@/components/Verification";

function Verify() {
  const [ref, setRef] = useState("");
  const [res, setRes] = useState<Verification | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [match, setMatch] = useState<"none" | "same" | "different" | "unregistered">("none");

  /** Hashes a file you hold, in your browser (it is never uploaded), and compares it with the register. */
  async function checkFile(f: File) {
    if (!res?.valid && !res?.sha256) return;
    const digest = await crypto.subtle.digest("SHA-256", await f.arrayBuffer());
    const hex = [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
    setMatch(!res.sha256 ? "unregistered" : hex === res.sha256 ? "same" : "different");
  }
  // Accepts a reference or a whole scanned QR address (…/v/<reference>.<version>.<date>.<class>.<signature>).
  async function run(input: string) {
    const text = input.trim();
    setErr(""); setRes(null); setMatch("none"); setBusy(true);
    try {
      const i = text.indexOf("/v/");
      setRes(i >= 0 ? await api.verifyQr(text.slice(i + 3)) : await api.verify(text.toUpperCase()));
    } catch (x) {
      setErr((x as Error).message);
    }
    setBusy(false);
  }
  // A link can carry the reference: /verify?ref=HS-INV-2026-000001. Following one from a result ("verify the newer
  // version") changes the address without reloading, so this runs whenever it changes.
  const q = useSearchParams().get("ref");
  useEffect(() => {
    if (q) { setRef(q); void run(q); }
  }, [q]); // eslint-disable-line react-hooks/exhaustive-deps

  const fileCheck = (
    <div className="vr-file">
      <label htmlFor="file">Check a PDF you were given <span className="muted small">(it stays on your device)</span></label>
      <input id="file" type="file" accept="application/pdf" onChange={(e) => { const f = e.target.files?.[0]; if (f) void checkFile(f); }} />
      {match === "same" && <p className="result ok" role="status">This file is identical to the original issued by HEKATE.</p>}
      {match === "different" && <p className="result bad" role="status">This file does NOT match the original. It may have been altered.</p>}
      {match === "unregistered" && <p className="muted" role="status">No original file has been recorded for this reference yet, so a file cannot be compared.</p>}
    </div>
  );
  return (
    <PublicShell>
      <section className="vr-form card">
        <h1>Verify a HEKATE document</h1>
        <p className="muted">Type the reference printed on the document, for example <code>HS-INV-2026-000123</code>, or paste the address from its QR code.</p>
        <form onSubmit={(e) => { e.preventDefault(); void run(ref); }}>
          <label htmlFor="ref">Reference or QR address</label>
          <div className="vr-input">
            <input id="ref" value={ref} onChange={(e) => setRef(e.target.value)} autoComplete="off" spellCheck={false} />
            <button className="btn primary" disabled={!ref.trim() || busy}>{busy ? "Checking…" : "Verify"}</button>
          </div>
        </form>
        {err && <p className="error" role="alert">{err}</p>}
      </section>
      {res && <VerificationResult v={res} fileCheck={fileCheck} />}
    </PublicShell>
  );
}

export default function VerifyPage() {
  return <Suspense fallback={null}><Verify /></Suspense>;
}
