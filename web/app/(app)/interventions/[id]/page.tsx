"use client";
import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { raw } from "@/lib/api";
import { when } from "@/lib/format";
import { Pill } from "@/components/ui";

interface Detail {
  id: string; engineer: string; status: string; summary: string; scheduled_for: string | null;
  checkin_at: string | null; checkin_lat: number | null; checkin_lng: number | null;
  checkout_at: string | null; checkout_lat: number | null; checkout_lng: number | null;
  report: string | null; signed_by: string | null; signature?: string; document_ref: string | null;
  parts: { item_id: string; qty: number }[];
}

export default function InterventionPage() {
  const { id } = useParams<{ id: string }>();
  const [v, setV] = useState<Detail | null>(null);
  const [err, setErr] = useState("");
  useEffect(() => { raw.get<Detail>(`/v1/interventions/${id}`).then(setV).catch((e) => setErr(e.message)); }, [id]);
  if (err) return <p className="error">{err}</p>;
  if (!v) return <p className="muted">Loading…</p>;
  const pos = (lat: number | null, lng: number | null) => (lat == null || lng == null ? "–" : `${lat.toFixed(5)}, ${lng.toFixed(5)}`);
  return (
    <>
      <h1>{v.summary || "Intervention"}</h1>
      <p className="meta"><Pill text={v.status} tone={v.status === "completed" ? "ok" : undefined} /> · engineer {v.engineer} · scheduled {when(v.scheduled_for)}</p>
      <dl className="facts">
        <dt>Checked in</dt><dd>{when(v.checkin_at)} at {pos(v.checkin_lat, v.checkin_lng)}</dd>
        <dt>Completed</dt><dd>{when(v.checkout_at)} at {pos(v.checkout_lat, v.checkout_lng)}</dd>
        {v.document_ref && (<><dt>Document</dt><dd><a href={`/verify?ref=${encodeURIComponent(v.document_ref)}`}>{v.document_ref}</a></dd></>)}
        {v.parts?.length > 0 && (<><dt>Parts used</dt><dd>{v.parts.map((p) => `${p.qty} × ${p.item_id.slice(0, 8)}`).join(", ")}</dd></>)}
      </dl>
      {v.report && <><h2>Report</h2><p className="card pre">{v.report}</p></>}
      {v.signature && (
        <>
          <h2>Client acceptance</h2>
          <p>Signed by {v.signed_by}</p>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={v.signature} alt={`Signature of ${v.signed_by}`} className="signature" />
        </>
      )}
    </>
  );
}
