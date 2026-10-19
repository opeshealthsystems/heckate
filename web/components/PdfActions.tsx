"use client";
import { useState } from "react";
import { downloadFile, fetchFile } from "@/lib/api";
import { DocumentPreview } from "./DocumentPreview";
import { Icon } from "./Icon";
import { Err, useAction } from "./ui";

/** Preview (then print) and Download for one PDF served by the API. */
export function PdfActions({ path, reference, title }: { path: string; reference: string; title: string }) {
  const [open, setOpen] = useState(false);
  const dl = useAction();
  return (
    <>
      <button className="btn" onClick={() => setOpen(true)}><Icon name="eye" size={16} /> Preview and print</button>
      <button className="btn" disabled={dl.busy} onClick={() => dl.run(() => downloadFile(path, `${reference}.pdf`))}><Icon name="download" size={16} /> Download PDF</button>
      <Err msg={dl.err} />
      {open && <DocumentPreview source={{ title: `${reference} · ${title}`, filename: `${reference}.pdf`, load: () => fetchFile(path) }} onClose={() => setOpen(false)} />}
    </>
  );
}
