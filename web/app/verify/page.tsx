"use client";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { Verification } from "@/lib/types";

export default function Verify() {
  const [ref, setRef] = useState("");
  const [res, setRes] = useState<Verification | null>(null);
  const [err, setErr] = useState("");
  const [match, setMatch] = useState<"none" | "same" | "different" | "unregistered">("none");
  /** Hashes a file you hold, in your browser (it is never uploaded), and compares it with the register. */
  async function checkFile(f: File) {
    if (!res?.valid && !res?.sha256) return;
    const digest = await crypto.subtle.digest("SHA-256", await f.arrayBuffer());
    const hex = [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
    setMatch(!res.sha256 ? "unregistered" : hex === res.sha256 ? "same" : "different");
  }
  async function run(r: string) {
    setErr("");
    setRes(null);
    setMatch("none");
    try {
      setRes(await api.verify(r.trim()));
    } catch (x) {
      setErr((x as Error).message);
    }
  }
  const check = (e: React.FormEvent) => {
    e.preventDefault();
    void run(ref);
  };
  // A link or QR code can carry the reference: /verify?ref=HS-INV-2026-000001
  useEffect(() => {
    const q = new URLSearchParams(location.search).get("ref");
    if (q) {
      setRef(q);
      void run(q);
    }
  }, []);
  return (
    <main className="center">
      <div className="card narrow">
        <h1>Verify a HEKATE document</h1>
        <p className="muted">Enter the reference printed on the document, e.g. HS-INV-2026-000123.</p>
        <form onSubmit={check}>
          <label htmlFor="ref">Reference</label>
          <input id="ref" value={ref} onChange={(e) => setRef(e.target.value)} autoComplete="off" />
          <button className="btn primary block" disabled={!ref.trim()}>Verify</button>
        </form>
        {err && <p className="error">{err}</p>}
        {res && (
          <div className={`result ${res.valid ? "ok" : "bad"}`} role="status">
            {res.valid ? (
              <>
                <strong>Valid document</strong>
                <p>{res.title}</p>
                <p className="muted small">
                  {res.ref} · issued {res.issued_at && new Date(res.issued_at).toLocaleDateString()}
                </p>
                <label htmlFor="file">Check a PDF you were given (it stays on your device)</label>
                <input id="file" type="file" accept="application/pdf" onChange={(e) => { const f = e.target.files?.[0]; if (f) void checkFile(f); }} />
                {match === "same" && <p className="result ok" role="status">This file is identical to the original issued by HEKATE.</p>}
                {match === "different" && <p className="result bad" role="status">This file does NOT match the original. It may have been altered.</p>}
                {match === "unregistered" && <p className="muted" role="status">No file has been recorded for this reference yet, so a file cannot be compared.</p>}
              </>
            ) : (
              <>
                <strong>{res.status === "revoked" ? "This document has been revoked" : "Not a valid HEKATE document"}</strong>
                <p className="muted small">{res.reason ?? "Contact HEKATE if you believe this is an error."}</p>
              </>
            )}
          </div>
        )}
      </div>
    </main>
  );
}
