"use client";
import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { when } from "@/lib/format";
import { Err, Pill, Table } from "@/components/ui";

interface Asset { id: string; item: string; serial: string; status: string; holder: string | null; tenant_id: string | null }
interface Hist { assignee: string; issued_by: string; issued_at: string; returned_at: string | null; document_ref: string | null }
interface Item { id: string; sku: string; name: string; unit: string; serialised: boolean; min_stock: number; on_hand: number; low: boolean }

/** Where a scanned QR sticker lands. It needs a sign-in, so a photo of a sticker shows nothing to a stranger. */
function Scan() {
  const q = useSearchParams();
  const me = useMe();
  const id = q.get("id");
  const itemId = q.get("item");
  const [asset, setAsset] = useState<{ asset: Asset; history: Hist[] } | null>(null);
  const [item, setItem] = useState<Item | null>(null);
  const [err, setErr] = useState("");
  useEffect(() => {
    if (id) raw.get<{ asset: Asset; history: Hist[] }>(`/v1/assets/${encodeURIComponent(id)}`).then(setAsset).catch((e) => setErr(e.message));
    else if (itemId) raw.get<Item>(`/v1/items/${encodeURIComponent(itemId)}`).then(setItem).catch((e) => setErr(e.message));
    else setErr("This sticker has no record attached.");
  }, [id, itemId]);
  return (
    <>
      <h1>{asset ? asset.asset.item : item ? item.name : "Scanned sticker"}</h1>
      <Err msg={err} />
      {asset && (
        <>
          <p className="meta"><Pill text={asset.asset.status} /> · serial <code>{asset.asset.serial}</code>{asset.asset.holder && ` · held by ${asset.asset.holder}`}{asset.asset.tenant_id && me.scope === "staff" && " · installed at a client"}</p>
          {me.scope === "staff" && (
            <>
              <h2>Who has had it</h2>
              <Table head={["Issued", "To", "By", "Returned", "Form"]} empty={asset.history.length ? undefined : "Never issued."}>
                {asset.history.map((h, i) => <tr key={i}><td>{when(h.issued_at)}</td><td>{h.assignee}</td><td>{h.issued_by}</td><td>{h.returned_at ? when(h.returned_at) : "still out"}</td><td>{h.document_ref ?? "–"}</td></tr>)}
              </Table>
              <p><Link href="/inventory">Back to inventory</Link></p>
            </>
          )}
        </>
      )}
      {item && (
        <>
          <p className="meta">SKU <code>{item.sku}</code> · {item.on_hand} {item.unit} on hand (minimum {item.min_stock}){item.low && <> <Pill text="low" tone="bad" /></>}{item.serialised && " · tracked by serial number"}</p>
          <p><Link href="/inventory">Back to inventory</Link></p>
        </>
      )}
    </>
  );
}

export default function AssetPage() {
  return <Suspense fallback={<p className="muted">Loading…</p>}><Scan /></Suspense>;
}
