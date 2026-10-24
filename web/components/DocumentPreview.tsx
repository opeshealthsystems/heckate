"use client";
import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { saveBlob } from "@/lib/api";
import { Icon } from "./Icon";

export interface PreviewSource {
  title: string;
  filename: string;
  /** Fetches the PDF; authenticated endpoints and static files both work. */
  load: () => Promise<Blob>;
  /** Shown under the title, for example the version and the check value of an approved template. */
  note?: string;
}

/**
 * Shows a PDF page by page before it is printed or filled in. Pages are drawn in the page itself (pdf.js), so it
 * works on phones and under our Content-Security-Policy, and Print prints exactly what is shown.
 */
export function DocumentPreview({ source, onClose }: { source: PreviewSource; onClose: () => void }) {
  const box = useRef<HTMLDivElement>(null);
  const [blob, setBlob] = useState<Blob | null>(null);
  const [pages, setPages] = useState(0);
  const [err, setErr] = useState("");

  useEffect(() => {
    let dead = false;
    source.load().then((b) => !dead && setBlob(b)).catch((e) => !dead && setErr((e as Error).message || "Could not load the document"));
    return () => { dead = true; };
  }, [source]);

  useEffect(() => {
    if (!blob || !box.current) return;
    let dead = false;
    const host = box.current;
    (async () => {
      try {
        const pdfjs = await import("pdfjs-dist/build/pdf.min.mjs");
        pdfjs.GlobalWorkerOptions.workerSrc = "/pdf.worker.min.mjs";
        const doc = await pdfjs.getDocument({ data: new Uint8Array(await blob.arrayBuffer()) }).promise;
        if (dead) return;
        setPages(doc.numPages);
        host.replaceChildren();
        for (let n = 1; n <= doc.numPages; n++) {
          const page = await doc.getPage(n);
          if (dead) return;
          const base = page.getViewport({ scale: 1 });
          const scale = Math.max(1.5, (Math.min(host.clientWidth, 900) / base.width) * (window.devicePixelRatio || 1));
          const view = page.getViewport({ scale });
          const canvas = document.createElement("canvas");
          canvas.width = view.width;
          canvas.height = view.height;
          canvas.className = "pv-page";
          canvas.setAttribute("role", "img");
          canvas.setAttribute("aria-label", `Page ${n} of ${doc.numPages}`);
          host.appendChild(canvas);
          await page.render({ canvasContext: canvas.getContext("2d")!, viewport: view }).promise;
          canvas.dataset.ready = "1";
        }
      } catch (e) {
        if (!dead) setErr((e as Error).message || "This file could not be displayed");
      }
    })();
    return () => { dead = true; };
  }, [blob]);

  useEffect(() => {
    const key = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    const after = () => document.body.classList.remove("printing-preview");
    window.addEventListener("keydown", key);
    window.addEventListener("afterprint", after);
    document.body.classList.add("preview-open");
    return () => { window.removeEventListener("keydown", key); window.removeEventListener("afterprint", after); document.body.classList.remove("preview-open", "printing-preview"); };
  }, [onClose]);

  const print = () => { document.body.classList.add("printing-preview"); window.print(); };
  // In a portal so it sits above the sidebar whatever stacking context the page that opened it is in.
  return createPortal(
    <div className="pv-overlay" role="dialog" aria-modal="true" aria-label={`Preview: ${source.title}`}>
      <div className="pv-bar">
        <div className="pv-title"><strong>{source.title}</strong>{source.note && <span>{source.note}</span>}</div>
        <div className="pv-actions">
          <button className="btn" disabled={!pages} onClick={print}><Icon name="print" size={16} /> Print</button>
          <button className="btn" disabled={!blob} onClick={() => blob && saveBlob(blob, source.filename)}><Icon name="download" size={16} /> Download</button>
          <button className="btn" autoFocus onClick={onClose} aria-label="Close preview"><Icon name="close" size={16} /> Close</button>
        </div>
      </div>
      <div className="pv-body">
        {err ? <p className="error" role="alert">{err}</p> : !pages && <p className="muted" role="status">Loading preview…</p>}
        <div ref={box} className="pv-pages" />
      </div>
    </div>,
    document.body,
  );
}
