"use client";
import { useCallback, useEffect, useState } from "react";
import { raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { ExportButton } from "@/components/ExportButton";
import { Err, Field, Pill, Table, Tabs, useAction } from "@/components/ui";

interface Item { id: string; sku: string; name: string; unit: string; serialised: boolean; min_stock: number; on_hand: number; low: boolean }
interface WH { id: string; name: string }
interface Level { item_id: string; sku: string; item: string; warehouse_id: string; warehouse: string; qty: number }
interface Asset { id: string; item_id: string; item: string; serial: string; status: string; holder: string | null; tenant_id: string | null }

export default function Inventory() {
  const me = useMe();
  const staff = me.scope === "staff" && me.roles.some((r) => ["ceo", "manager", "logistics", "engineer", "support", "sales", "finance", "hr", "employee"].includes(r));
  const keeper = me.roles.some((r) => ["logistics", "manager", "ceo"].includes(r));
  const [tab, setTab] = useState(staff ? "Items" : "Equipment");
  const [items, setItems] = useState<Item[]>([]);
  const [whs, setWhs] = useState<WH[]>([]);
  const [levels, setLevels] = useState<Level[]>([]);
  const [assets, setAssets] = useState<Asset[]>([]);
  const [err, setErr] = useState("");
  const [item, setItem] = useState({ sku: "", name: "", serialised: false, min_stock: "0" });
  const [wh, setWh] = useState("");
  const [rcv, setRcv] = useState({ item_id: "", warehouse_id: "", qty: "1", serials: "", ref: "" });
  const [assign, setAssign] = useState<Record<string, string>>({});
  const load = useCallback(async () => {
    try {
      if (staff) {
        setItems((await raw.get<{ data: Item[] }>("/v1/items")).data);
        setWhs((await raw.get<{ data: WH[] }>("/v1/warehouses")).data);
        setLevels((await raw.get<{ data: Level[] }>("/v1/stock")).data);
      }
      setAssets((await raw.get<{ data: Asset[] }>("/v1/assets")).data);
    } catch (e) {
      setErr((e as Error).message);
    }
  }, [staff]);
  useEffect(() => { void load(); }, [load]);
  const act = useAction(load);
  const low = items.filter((i) => i.low);
  return (
    <>
      <h1>{staff ? "Inventory and assets" : "Your equipment"}</h1>
      <Err msg={err} />
      {staff && <div className="actions"><ExportButton path="/v1/exports/stock" filename="stock" label="Export stock CSV" /><ExportButton path="/v1/exports/assets" filename="assets" label="Export assets CSV" /></div>}
      {staff && <Tabs tabs={["Items", "Stock", "Equipment"]} value={tab} onChange={setTab} />}
      {staff && tab === "Items" && (
        <>
          {low.length > 0 && <p className="notice">{low.length} item(s) below minimum stock: {low.map((i) => i.sku).join(", ")}</p>}
          <Table head={["SKU", "Name", "On hand", "Minimum", ""]} empty={items.length ? undefined : "No items yet."}>
            {items.map((i) => (<tr key={i.id}><td>{i.sku}</td><td>{i.name}{i.serialised && " (serialised)"}</td><td className="num">{i.on_hand}</td><td className="num">{i.min_stock}</td><td>{i.low && <Pill text="low" tone="bad" />}</td></tr>))}
          </Table>
          {keeper && (
            <>
              <h2>New item</h2>
              <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post("/v1/items", { ...item, min_stock: Number(item.min_stock) }); setItem({ sku: "", name: "", serialised: false, min_stock: "0" }); }); }}>
                <div className="grid2">
                  <Field id="it-sku" label="SKU"><input id="it-sku" required value={item.sku} onChange={(e) => setItem({ ...item, sku: e.target.value })} /></Field>
                  <Field id="it-name" label="Name"><input id="it-name" required value={item.name} onChange={(e) => setItem({ ...item, name: e.target.value })} /></Field>
                </div>
                <Field id="it-min" label="Minimum stock"><input id="it-min" type="number" min="0" value={item.min_stock} onChange={(e) => setItem({ ...item, min_stock: e.target.value })} /></Field>
                <label className="check"><input type="checkbox" checked={item.serialised} onChange={(e) => setItem({ ...item, serialised: e.target.checked })} /> Tracked by serial number</label>
                <Err msg={act.err} />
                <button className="btn primary" disabled={act.busy}>Create item</button>
              </form>
              <h2>New warehouse</h2>
              <form className="inline" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post("/v1/warehouses", { name: wh }); setWh(""); }); }}>
                <Field id="wh-name" label="Name"><input id="wh-name" required value={wh} onChange={(e) => setWh(e.target.value)} /></Field>
                <button className="btn">Add</button>
              </form>
            </>
          )}
        </>
      )}
      {staff && tab === "Stock" && (
        <>
          <Table head={["Item", "Warehouse", "Quantity"]} empty={levels.length ? undefined : "No stock recorded."}>
            {levels.map((l, i) => (<tr key={i}><td>{l.sku} · {l.item}</td><td>{l.warehouse}</td><td className="num">{l.qty}</td></tr>))}
          </Table>
          {keeper && (
            <>
              <h2>Receive stock</h2>
              <form className="card form" onSubmit={(e) => { e.preventDefault(); void act.run(async () => { await raw.post("/v1/stock/receipts", { item_id: rcv.item_id, warehouse_id: rcv.warehouse_id, qty: Number(rcv.qty), ref: rcv.ref, serials: rcv.serials.trim() ? rcv.serials.split(/[\s,]+/).filter(Boolean) : undefined }); setRcv({ ...rcv, qty: "1", serials: "", ref: "" }); }); }}>
                <Field id="rc-item" label="Item"><select id="rc-item" required value={rcv.item_id} onChange={(e) => setRcv({ ...rcv, item_id: e.target.value })}><option value="">Select…</option>{items.map((i) => <option key={i.id} value={i.id}>{i.sku} · {i.name}</option>)}</select></Field>
                <Field id="rc-wh" label="Warehouse"><select id="rc-wh" required value={rcv.warehouse_id} onChange={(e) => setRcv({ ...rcv, warehouse_id: e.target.value })}><option value="">Select…</option>{whs.map((w) => <option key={w.id} value={w.id}>{w.name}</option>)}</select></Field>
                <Field id="rc-qty" label="Quantity"><input id="rc-qty" type="number" min="1" step="1" required value={rcv.qty} onChange={(e) => setRcv({ ...rcv, qty: e.target.value })} /></Field>
                <Field id="rc-ser" label="Serial numbers (serialised items; separate with spaces or commas)"><textarea id="rc-ser" rows={2} value={rcv.serials} onChange={(e) => setRcv({ ...rcv, serials: e.target.value })} /></Field>
                <Field id="rc-ref" label="Reference"><input id="rc-ref" value={rcv.ref} onChange={(e) => setRcv({ ...rcv, ref: e.target.value })} /></Field>
                <Err msg={act.err} />
                <button className="btn primary" disabled={act.busy}>Receive</button>
              </form>
            </>
          )}
        </>
      )}
      {tab === "Equipment" && (
        <Table head={["Item", "Serial", "Status", ...(staff ? ["Holder / client", ""] : [])]} empty={assets.length ? undefined : "No equipment."}>
          {assets.map((a) => (
            <tr key={a.id}>
              <td>{a.item}</td><td>{a.serial}</td><td><Pill text={a.status} /></td>
              {staff && <td>{a.holder ?? a.tenant_id?.slice(0, 8) ?? "–"}</td>}
              {staff && (
                <td>
                  {keeper && a.status === "in_stock" && (
                    <form className="inline" onSubmit={(e) => { e.preventDefault(); void act.run(() => raw.post("/v1/assets/issue", { serial_id: a.id, assignee: assign[a.id] })); }}>
                      <input aria-label={`Assign ${a.serial} to`} placeholder="Assign to (account id)" required value={assign[a.id] ?? ""} onChange={(e) => setAssign({ ...assign, [a.id]: e.target.value })} />
                      <button className="btn">Issue</button>
                    </form>
                  )}
                  {keeper && a.status === "issued" && whs[0] && <button className="btn" onClick={() => act.run(() => raw.post("/v1/assets/return", { serial_id: a.id, warehouse_id: whs[0].id }))}>Return to {whs[0].name}</button>}
                </td>
              )}
            </tr>
          ))}
        </Table>
      )}
    </>
  );
}
