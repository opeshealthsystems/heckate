import { useCallback, useState } from "react";
import { useFocusEffect } from "expo-router";
import { Row, Section } from "./List";

/** Status words shown to the person who made the request. */
export const STATUS_WORD: Record<string, string> = {
  draft: "Draft · not sent", submitted: "Waiting for approval", pending: "Waiting for approval",
  approved: "Approved", rejected: "Rejected", cancelled: "Cancelled", paid: "Paid",
};

/** Loads a person's own past requests whenever the screen is shown. `reload` refreshes after a change. */
export function useHistory<T>(load: () => Promise<{ data: T[] }>) {
  const [rows, setRows] = useState<T[] | null>(null);
  const [err, setErr] = useState("");
  const reload = useCallback(() => load().then((r) => { setRows(r.data); setErr(""); }).catch((e) => setErr((e as Error).message)), [load]);
  useFocusEffect(useCallback(() => void reload(), [reload]));
  return { rows, err, reload };
}

/** The "Your requests" list under a self-service form: the latest ten, newest first. */
export function HistorySection<T>({ header, rows, err, empty, render }: { header: string; rows: T[] | null; err: string; empty: string; render: (r: T) => React.ReactNode }) {
  return (
    <Section header={header} footer={err ? `Could not load: ${err}` : undefined} footerTone="red">
      {rows === null && !err ? <Row title="Loading…" /> : rows && rows.length ? rows.slice(0, 10).map(render) : !err && <Row title={empty} />}
    </Section>
  );
}
