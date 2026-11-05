"use client";
import { downloadFile } from "@/lib/api";
import { Icon } from "./Icon";
import { Err, useAction } from "./ui";

/** Downloads a CSV (opens in Excel) from an export endpoint. */
export function ExportButton({ path, filename, label = "Export CSV" }: { path: string; filename: string; label?: string }) {
  const a = useAction();
  return (
    <>
      <button className="btn" disabled={a.busy} onClick={() => a.run(() => downloadFile(path, `${filename}-${new Date().toISOString().slice(0, 10)}.csv`))}>
        <Icon name="download" size={16} /> {a.busy ? "Preparing…" : label}
      </button>
      <Err msg={a.err} />
    </>
  );
}
