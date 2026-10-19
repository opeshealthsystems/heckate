"use client";
import { useEffect, useState } from "react";
import { raw } from "@/lib/api";
import { diffLines, versionText } from "@/lib/diff";
import type { Version } from "@/lib/doc-library";
import { when } from "@/lib/format";
import { Pill } from "./ui";

/** Every saved version of a document: who, when, why, what changed since the one before, and its content fingerprint. */
export function VersionHistory({ id }: { id: string }) {
  const [rows, setRows] = useState<Version[] | null>(null);
  const [err, setErr] = useState("");
  useEffect(() => { raw.get<{ data: Version[] }>(`/v1/narratives/${id}/versions`).then((r) => setRows(r.data)).catch((e) => setErr(e.message)); }, [id]);
  if (err) return <p className="error" role="alert">{err}</p>;
  if (!rows) return <p className="muted">Loading history…</p>;
  return (
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
  );
}
