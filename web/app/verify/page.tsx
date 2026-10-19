"use client";
import { useState } from "react";
import { api } from "@/lib/api";
import type { Verification } from "@/lib/types";

export default function Verify() {
  const [ref, setRef] = useState("");
  const [res, setRes] = useState<Verification | null>(null);
  const [err, setErr] = useState("");
  async function check(e: React.FormEvent) {
    e.preventDefault();
    setErr("");
    setRes(null);
    try {
      setRes(await api.verify(ref.trim()));
    } catch (x) {
      setErr((x as Error).message);
    }
  }
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
