"use client";
import { useState } from "react";
import { useParams } from "next/navigation";
import { API_URL } from "@/lib/api";
import { saveBlob } from "@/lib/api";
import { PublicShell } from "@/components/Verification";

/** A link HEKATE granted: downloads the original PDF. It works a limited number of times and then expires. */
export default function Access() {
  const { token } = useParams<{ token: string }>();
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState(false);
  async function get() {
    setBusy(true); setMsg("");
    try {
      const res = await fetch(`${API_URL}/v1/public/access/${encodeURIComponent(token)}`);
      if (!res.ok) { const p = await res.json().catch(() => ({})); throw new Error(p.detail || "This link is not valid."); }
      const name = /filename="([^"]+)"/.exec(res.headers.get("content-disposition") ?? "")?.[1] ?? "document.pdf";
      saveBlob(await res.blob(), name);
      setMsg("Downloaded. The file’s fingerprint can be checked at hekatesystems.com/verify.");
    } catch (e) { setMsg((e as Error).message); }
    setBusy(false);
  }
  return (
    <PublicShell>
      <section className="card vr-form">
        <h1>Your document from HEKATE</h1>
        <p className="muted">HEKATE approved your request to see this document. The link works a few times and expires, so download it once and keep the file.</p>
        <button className="btn primary" disabled={busy} onClick={get}>{busy ? "Preparing…" : "Download the PDF"}</button>
        {msg && <p role="status" className="notice">{msg}</p>}
      </section>
    </PublicShell>
  );
}
