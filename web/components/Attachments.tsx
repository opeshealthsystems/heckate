"use client";
import { useCallback, useEffect, useState } from "react";
import { downloadAttachment, raw, uploadAttachment, type Attachment } from "@/lib/api";
import { Err, useAction } from "./ui";

const kb = (n: number) => (n < 1024 * 1024 ? `${Math.max(1, Math.round(n / 1024))} KB` : `${(n / 1024 / 1024).toFixed(1)} MB`);

/** Files attached to a ticket, site visit or expense claim, with an upload control when allowed. */
export function Attachments({ entityType, entityId, canUpload = true }: { entityType: "ticket" | "intervention" | "expense"; entityId: string; canUpload?: boolean }) {
  const [files, setFiles] = useState<Attachment[]>([]);
  const [loadErr, setLoadErr] = useState("");
  const load = useCallback(
    () => raw.get<{ data: Attachment[] }>(`/v1/attachments?entity_type=${entityType}&entity_id=${entityId}`).then((r) => setFiles(r.data)).catch((e) => setLoadErr(e.message)),
    [entityType, entityId],
  );
  useEffect(() => { void load(); }, [load]);
  const up = useAction(load);
  const dl = useAction();
  return (
    <section>
      <h2>Attachments</h2>
      <Err msg={loadErr || up.err || dl.err} />
      {files.length === 0 && !loadErr && <p className="muted">No attachments.</p>}
      <ul className="thread">
        {files.map((a) => (
          <li key={a.id}>
            <button className="btn ghost" onClick={() => dl.run(() => downloadAttachment(a))}>{a.filename}</button>
            <span className="muted small"> · {kb(a.size)} · {new Date(a.created_at).toLocaleString()}</span>
          </li>
        ))}
      </ul>
      {canUpload && (
        <>
          <label htmlFor={`att-${entityId}`}>Add a photo or PDF (max 10 MB)</label>
          <input
            id={`att-${entityId}`}
            type="file"
            accept="image/jpeg,image/png,image/webp,application/pdf"
            disabled={up.busy}
            onChange={(e) => {
              const f = e.target.files?.[0];
              e.target.value = "";
              if (f) void up.run(() => uploadAttachment(entityType, entityId, f));
            }}
          />
        </>
      )}
    </section>
  );
}
