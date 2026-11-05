"use client";
import { useEffect, useState } from "react";
import { raw } from "@/lib/api";
import { diffLines, versionText } from "@/lib/diff";
import type { Version } from "@/lib/doc-library";
import { when } from "@/lib/format";
import { Pill } from "./ui";

/** Every saved version of a document: who, when, why, what changed since the one before, and its content fingerprint. */
interface Checks { total: number; last: string | null; recent: { at: string; channel: string; result: string }[] }

export function VersionHistory({ id, reference }: { id: string; reference?: string | null }) {
  const [rows, setRows] = useState<Version[] | null>(null);
  const [err, setErr] = useState("");
  const [checks, setChecks] = useState<Checks | null>(null);
  useEffect(() => { if (reference) raw.get<Checks>(`/v1/documents/${reference}/verifications`).then(setChecks).catch(() => {}); }, [reference]);
  useEffect(() => { raw.get<{ data: Version[] }>(`/v1/narratives/${id}/versions`).then((r) => setRows(r.data)).catch((e) => setErr(e.message)); }, [id]);
  if (err) return <p className="error" role="alert">{err}</p>;
  if (!rows) return <p className="muted">Loading history…</p>;
  return (
    <>
      {reference && checks && (
        <p className="notice" role="status">
          <strong>{checks.total === 0 ? "Not verified by anyone yet." : `Verified ${checks.total} time${checks.total === 1 ? "" : "s"} by the public`}</strong>
          {checks.last && <> · last on {when(checks.last)} ({checks.recent[0]?.channel === "qr" ? "QR code" : checks.recent[0]?.channel}, {checks.recent[0]?.result.replace(/_/g, " ")})</>}
        </p>
      )}
    <ol className="history" aria-label="Version history">
      {[...rows].reverse().map((v) => {
        const prev = rows.find((x) => x.version === v.version - 1);
        const lines = prev ? diffLines(versionText(prev), versionText(v)) : null;
        const changed = lines?.filter((l) => l.kind !== "same").length ?? 0;
        return (
          <li key={v.version}>
            <div className="row">
              <span><strong>Version {v.version}</strong> <Pill text={v.event} tone={v.event === "issued" ? "ok" : undefined} /></span>
              <span className="muted small">{when(v.changed_at)} · {v.changed_by}</span>
            </div>
            <p>{v.change_note || "–"}</p>
            <details>
              <summary>{lines ? (changed ? `${changed} line${changed === 1 ? "" : "s"} changed since version ${v.version - 1}` : "No change in content") : "Content of this version"}</summary>
              <pre className="diff" tabIndex={0}>
                {(lines ?? versionText(v).split("\n").map((text) => ({ kind: "same" as const, text }))).map((l, i) => (
                  <span key={i} className={`d-${l.kind}`}>{l.kind === "add" ? "+ " : l.kind === "del" ? "- " : "  "}{l.text}{"\n"}</span>
                ))}
              </pre>
              <p className="muted small">Fingerprint (SHA-256): <code>{v.content_sha256}</code></p>
            </details>
          </li>
        );
      })}
    </ol>
    </>
  );
}
